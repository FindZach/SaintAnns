// Package importer scrapes pages from the existing olv14843.org site, converts
// each one to a stack of CMS blocks, downloads referenced images and PDFs as
// Assets, and upserts them into the Postgres database.
//
// Idempotent: pages key on their slug, assets key on their source URL (recorded
// in Asset.OriginalName). Re-running the importer updates existing rows rather
// than duplicating.
//
// Two entry points call Run:
//   - cmd/import — standalone binary for dev use.
//   - main.go's --import flag — in-container entry so prod can import without
//     needing Go installed. See HANDOFF.md.
package importer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// DefaultBaseURL is the live site we scrape from.
const DefaultBaseURL = "https://www.olv14843.org"

// Options are the knobs the caller passes to Run.
type Options struct {
	DryRun    bool   // scrape and log; no DB writes or downloads
	SingleURL string // import just this URL (e.g. /about/)
	Limit     int    // stop after N pages (0 = all)
	BaseURL   string // live site base URL; defaults to DefaultBaseURL when empty
	Verbose   bool
}

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

// Run walks the page list and imports each one. db must be a GORM handle to a
// Postgres instance; the caller is responsible for opening it.
func Run(db *gorm.DB, opts Options) error {
	if db == nil {
		return fmt.Errorf("importer: nil db")
	}
	base := opts.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	imp := newImporter(db, base, opts.DryRun, opts.Verbose)

	targets := pages
	if opts.SingleURL != "" {
		targets = nil
		for _, p := range pages {
			if p.URL == opts.SingleURL || p.Slug == strings.Trim(opts.SingleURL, "/") {
				targets = append(targets, p)
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("no entry in pages[] matches URL %q", opts.SingleURL)
		}
	}

	log.Printf("Importing %d pages (dry-run=%v)", len(targets), opts.DryRun)

	imported := 0
	for _, p := range targets {
		if p.Skip {
			if opts.Verbose {
				log.Printf("[skip] %s (structured content)", p.URL)
			}
			continue
		}
		if err := imp.importPage(p); err != nil {
			log.Printf("[fail] %s: %v", p.URL, err)
			continue
		}
		imported++
		if opts.Limit > 0 && imported >= opts.Limit {
			log.Printf("Reached --limit %d; stopping", opts.Limit)
			break
		}
	}

	log.Printf("Done. Imported %d / %d pages, downloaded %d assets",
		imported, len(targets), imp.assetCount)
	return nil
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
