// Command import scrapes pages from the existing olv14843.org site, converts
// each one to a stack of CMS blocks, downloads referenced images and PDFs as
// Assets, and upserts them into our Postgres database.
//
// Idempotent: pages key on their slug, assets key on their source URL (recorded
// in Asset.OriginalName). Re-running the importer updates existing rows rather
// than duplicating.
//
// Usage:
//
//	go run ./cmd/import --dry-run        # scrape and log; no DB writes, no downloads
//	go run ./cmd/import --url /about/    # import a single URL
//	go run ./cmd/import --limit 5        # cap the number of pages
//	go run ./cmd/import                  # full import
//
// Required env vars match the server's: DB_HOST, DB_PORT (default 5432),
// DB_USER (default postgres), DB_PASSWORD, DB_NAME (default olv_parish),
// DB_SSLMODE (default disable).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"olv-parish/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// baseURL is the live site we scrape from.
const baseURL = "https://www.olv14843.org"

// pageSpec defines one page we want to import — its slug (DB key), the live
// URL to fetch, and the parent / nav label hints.
type pageSpec struct {
	Slug       string
	URL        string
	Title      string
	NavLabel   string
	ParentSlug string
	// Skip: this page uses structured content (Mass times, confessions) and
	// we don't want to overwrite that with scraped prose.
	Skip bool
}

// pages is the full URL map we try to import. It mirrors what we saw on the
// live nav and keeps the current site's URL slugs verbatim (typos and all).
var pages = []pageSpec{
	{Slug: "about", URL: "/about/", Title: "About the Parish", NavLabel: "About"},
	{Slug: "about/parish-registration", URL: "/about/parish-registration/", Title: "Parish Registration", NavLabel: "Parish Registration", ParentSlug: "about"},
	{Slug: "about/staff", URL: "/about/staff/", Title: "Staff", NavLabel: "Staff", ParentSlug: "about"},
	{Slug: "about/photo-albums", URL: "/about/photo-albums/", Title: "Photo Albums", NavLabel: "Photo Albums", ParentSlug: "about"},
	{Slug: "about/bulletins", URL: "/about/bulletins/", Title: "Bulletins", NavLabel: "Bulletins", ParentSlug: "about"},
	{Slug: "about/donate", URL: "/about/donate/", Title: "Donate", NavLabel: "Donate", ParentSlug: "about"},
	{Slug: "about/eucharistic-revival", URL: "/about/eucharistic-revival/", Title: "Eucharistic Revival", NavLabel: "Eucharistic Revival", ParentSlug: "about"},

	{Slug: "liturgy-and-sacraments", URL: "/liturgy-and-sacraments/", Title: "Liturgy and Sacraments", NavLabel: "Liturgy & Sacraments"},
	// Mass times + confession times are structured content — don't overwrite.
	{Slug: "liturgy-and-sacraments/mass-times", URL: "/liturgy-and-sacraments/mass-times/", Skip: true},
	{Slug: "liturgy-and-sacraments/confession-times", URL: "/liturgy-and-sacraments/confession-times/", Skip: true},
	{Slug: "liturgy-and-sacraments/sacraments", URL: "/liturgy-and-sacraments/sacraments/", Title: "Sacraments", NavLabel: "Sacraments", ParentSlug: "liturgy-and-sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/baptism", URL: "/liturgy-and-sacraments/sacraments/baptism/", Title: "Baptism", NavLabel: "Baptism", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/reconciliation", URL: "/liturgy-and-sacraments/sacraments/reconciliation/", Title: "Reconciliation", NavLabel: "Reconciliation", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/holy-communion", URL: "/liturgy-and-sacraments/sacraments/holy-communion/", Title: "Holy Communion", NavLabel: "Holy Communion", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/confirmation", URL: "/liturgy-and-sacraments/sacraments/confirmation/", Title: "Confirmation", NavLabel: "Confirmation", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/anointing-of-the-sick", URL: "/liturgy-and-sacraments/sacraments/anointing-of-the-sick/", Title: "Anointing of the Sick", NavLabel: "Anointing of the Sick", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/matrimony", URL: "/liturgy-and-sacraments/sacraments/matrimony/", Title: "Matrimony", NavLabel: "Matrimony", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/sacraments/holy-orders", URL: "/liturgy-and-sacraments/sacraments/holy-orders/", Title: "Holy Orders", NavLabel: "Holy Orders", ParentSlug: "liturgy-and-sacraments/sacraments"},
	{Slug: "liturgy-and-sacraments/perpetural-adoration", URL: "/liturgy-and-sacraments/perpetural-adoration/", Title: "Perpetual Adoration", NavLabel: "Perpetual Adoration", ParentSlug: "liturgy-and-sacraments"},
	{Slug: "liturgy-and-sacraments/last-weekends-homily", URL: "/liturgy-and-sacraments/last-weekends-homily/", Title: "Last Weekend's Homily", NavLabel: "Last Weekend's Homily", ParentSlug: "liturgy-and-sacraments"},

	{Slug: "minstries", URL: "/minstries/", Title: "Ministries", NavLabel: "Ministries"},
	{Slug: "minstries/parish-pastoral-council", URL: "/minstries/parish-pastoral-council/", Title: "Parish Pastoral Council", NavLabel: "Parish Pastoral Council", ParentSlug: "minstries"},
	{Slug: "minstries/finance-committee", URL: "/minstries/finance-committee/", Title: "Finance Committee", NavLabel: "Finance Committee", ParentSlug: "minstries"},
	{Slug: "minstries/liturgy-committee", URL: "/minstries/liturgy-committee/", Title: "Liturgy Committee", NavLabel: "Liturgy Committee", ParentSlug: "minstries"},
	{Slug: "minstries/cemetery-committee", URL: "/minstries/cemetery-committee/", Title: "Cemetery Committee", NavLabel: "Cemetery Committee", ParentSlug: "minstries"},
	{Slug: "minstries/usher-ministry", URL: "/minstries/usher-ministry/", Title: "Usher Ministry", NavLabel: "Usher Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/altar-server-ministry", URL: "/minstries/altar-server-ministry/", Title: "Altar Server Ministry", NavLabel: "Altar Server Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/funeral-server-ministry", URL: "/minstries/funeral-server-ministry/", Title: "Funeral Server Ministry", NavLabel: "Funeral Server Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/music-ministry", URL: "/minstries/music-ministry/", Title: "Music Ministry", NavLabel: "Music Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/lector-ministry", URL: "/minstries/lector-ministry/", Title: "Lector Ministry", NavLabel: "Lector Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/extra-ordinary-minister-of-holy-communion-ministry", URL: "/minstries/extra-ordinary-minister-of-holy-communion-ministry/", Title: "Extraordinary Minister of Holy Communion", NavLabel: "EMHC Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/bereavement-ministry", URL: "/minstries/bereavement-ministry/", Title: "Bereavement Ministry", NavLabel: "Bereavement Ministry", ParentSlug: "minstries"},
	{Slug: "minstries/bereavement-ministry/grief-support-group", URL: "/minstries/bereavement-ministry/grief-support-group/", Title: "Grief Support Group", NavLabel: "Grief Support Group", ParentSlug: "minstries/bereavement-ministry"},
	{Slug: "minstries/bereavement-ministry/preparing-to-honor-your-loved-one", URL: "/minstries/bereavement-ministry/preparing-to-honor-your-loved-one/", Title: "Preparing to Honor Your Loved One", NavLabel: "Preparing to Honor Your Loved One", ParentSlug: "minstries/bereavement-ministry"},
	{Slug: "minstries/childrens-liturgy-of-the-word", URL: "/minstries/childrens-liturgy-of-the-word/", Title: "Children's Liturgy of the Word", NavLabel: "Children's Liturgy of the Word", ParentSlug: "minstries"},
	{Slug: "minstries/moms-support-group", URL: "/minstries/moms-support-group/", Title: "Mom's Support Group", NavLabel: "Mom's Support Group", ParentSlug: "minstries"},

	{Slug: "faith-formation", URL: "/faith-formation/", Title: "Faith Formation", NavLabel: "Faith Formation"},
	{Slug: "faith-formation/young-adult-faith-on-tap", URL: "/faith-formation/young-adult-faith-on-tap/", Title: "Young Adult: Faith on Tap", NavLabel: "Young Adult - Faith on Tap", ParentSlug: "faith-formation"},
	{Slug: "faith-formation/faith-foramtion-for-adults", URL: "/faith-formation/faith-foramtion-for-adults/", Title: "Faith Formation for Adults", NavLabel: "Faith Formation for Adults", ParentSlug: "faith-formation"},
	{Slug: "faith-formation/childrens-faith-formation", URL: "/faith-formation/childrens-faith-formation/", Title: "Children's Faith Formation", NavLabel: "Children's Faith Formation", ParentSlug: "faith-formation"},
	{Slug: "faith-formation/rite-of-christian-initiation-for-adults", URL: "/faith-formation/rite-of-christian-initiation-for-adults/", Title: "Rite of Christian Initiation for Adults", NavLabel: "RCIA", ParentSlug: "faith-formation"},
	{Slug: "faith-formation/quest-youth-group", URL: "/faith-formation/quest-youth-group/", Title: "Quest Youth Group", NavLabel: "Quest Youth Group", ParentSlug: "faith-formation"},

	{Slug: "events-news", URL: "/events-news/", Title: "Events & News", NavLabel: "Events & News"},
	{Slug: "events-news/bishop-bonnicis-installation-mass", URL: "/events-news/bishop-bonnicis-installation-mass/", Title: "Bishop Bonnici's Installation Mass", NavLabel: "Parish News", ParentSlug: "events-news"},
}

func main() {
	var (
		dryRun     = flag.Bool("dry-run", false, "scrape and log; no DB writes or downloads")
		singleURL  = flag.String("url", "", "import just this URL (e.g. /about/)")
		limit      = flag.Int("limit", 0, "stop after N pages (0 = all)")
		baseFlag   = flag.String("base", baseURL, "live site base URL")
		verbose    = flag.Bool("verbose", false, "chattier logs")
	)
	flag.Parse()

	db, err := openDB()
	if err != nil {
		log.Fatalf("DB: %v", err)
	}

	imp := newImporter(db, *baseFlag, *dryRun, *verbose)

	targets := pages
	if *singleURL != "" {
		targets = nil
		for _, p := range pages {
			if p.URL == *singleURL || p.Slug == strings.Trim(*singleURL, "/") {
				targets = append(targets, p)
				break
			}
		}
		if len(targets) == 0 {
			log.Fatalf("no entry in pages[] matches URL %q", *singleURL)
		}
	}

	log.Printf("Importing %d pages (dry-run=%v)", len(targets), *dryRun)

	imported := 0
	for _, p := range targets {
		if p.Skip {
			if *verbose {
				log.Printf("[skip] %s (structured content)", p.URL)
			}
			continue
		}
		if err := imp.importPage(p); err != nil {
			log.Printf("[fail] %s: %v", p.URL, err)
			continue
		}
		imported++
		if *limit > 0 && imported >= *limit {
			log.Printf("Reached --limit %d; stopping", *limit)
			break
		}
	}

	log.Printf("Done. Imported %d / %d pages, downloaded %d assets",
		imported, len(targets), imp.assetCount)
}

// --------- Importer orchestration ---------

type importer struct {
	db         *gorm.DB
	base       string
	baseURL    *url.URL
	dryRun     bool
	verbose    bool
	assetCache map[string]uint // source URL → Asset.ID
	assetCount int
}

func newImporter(db *gorm.DB, base string, dryRun, verbose bool) *importer {
	u, _ := url.Parse(base)
	return &importer{
		db:         db,
		base:       strings.TrimRight(base, "/"),
		baseURL:    u,
		dryRun:     dryRun,
		verbose:    verbose,
		assetCache: map[string]uint{},
	}
}

func (imp *importer) importPage(p pageSpec) error {
	full := imp.base + p.URL
	log.Printf("[fetch] %s", full)
	doc, err := fetchHTML(full)
	if err != nil {
		return err
	}

	// Build blocks.
	ex := newExtractor(imp, p)
	blocks, err := ex.extract(doc)
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		// Fall back to a placeholder so the Page still exists.
		raw, _ := json.Marshal(map[string]string{"html": "<p>Content from the live site could not be extracted. Edit this page in the admin to add content.</p>"})
		blocks = models.Blocks{{Type: models.BlockRichText, Data: raw}}
	}

	log.Printf("[page] %s → %d blocks", p.Slug, len(blocks))

	if imp.dryRun {
		if imp.verbose {
			out, _ := json.MarshalIndent(blocks, "", "  ")
			log.Printf("[dry-run] would upsert page %s with blocks:\n%s", p.Slug, string(out))
		}
		return nil
	}

	now := time.Now()
	var page models.Page
	err = imp.db.Where("slug = ?", p.Slug).First(&page).Error
	if err == gorm.ErrRecordNotFound {
		page = models.Page{
			Slug:        p.Slug,
			ParentSlug:  p.ParentSlug,
			Title:       p.Title,
			NavLabel:    p.NavLabel,
			Blocks:      blocks,
			PublishedAt: &now,
		}
		return imp.db.Create(&page).Error
	}
	if err != nil {
		return err
	}
	// Update existing. Preserve nav_label / title if the DB has non-empty
	// values and the import spec is empty.
	page.ParentSlug = p.ParentSlug
	if p.Title != "" {
		page.Title = p.Title
	}
	if p.NavLabel != "" {
		page.NavLabel = p.NavLabel
	}
	page.Blocks = blocks
	if page.PublishedAt == nil {
		page.PublishedAt = &now
	}
	return imp.db.Save(&page).Error
}

// --------- DB connection ---------

func openDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", ""),
		envOr("DB_NAME", "olv_parish"),
		envOr("DB_SSLMODE", "disable"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}
	// AutoMigrate the content tables we touch so a fresh DB can be imported into.
	if err := db.AutoMigrate(&models.Page{}, &models.Asset{}); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}
	return db, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
