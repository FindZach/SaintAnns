package database

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// MigrateHomiliesFromPage scans the imported /liturgy-and-sacraments/last-weekends-homily/
// Page row for pdf_link blocks and creates a Homily row for each one, so the
// content the WordPress import captured as PDFs is also editable in
// /admin/homilies going forward.
//
// Idempotent: a Homily with the same (title, document_url) is skipped if
// already present, so re-running the command won't duplicate.
func MigrateHomiliesFromPage(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate homilies: no database connection")
	}
	log.Println("Migrating imported homilies from Page blocks into the homilies table ...")

	var page models.Page
	if err := db.Where("slug = ?", "liturgy-and-sacraments/last-weekends-homily").First(&page).Error; err != nil {
		return fmt.Errorf("migrate homilies: lookup page: %w", err)
	}

	type pdfLinkPayload struct {
		AssetID uint   `json:"asset_id"`
		Label   string `json:"label"`
	}

	created := 0
	skipped := 0
	for _, block := range page.Blocks {
		if block.Type != models.BlockPDFLink {
			continue
		}
		var p pdfLinkPayload
		if err := json.Unmarshal(block.Data, &p); err != nil || p.AssetID == 0 {
			continue
		}

		var asset models.Asset
		if err := db.First(&asset, p.AssetID).Error; err != nil {
			log.Printf("  skip: asset %d not found", p.AssetID)
			continue
		}
		docURL := asset.URL()

		title, author := splitTitleAuthor(p.Label)
		preachedOn := guessPreachedOn(title)

		var existing models.Homily
		if err := db.Where("title = ? AND document_url = ?", title, docURL).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		row := models.Homily{
			Title:       title,
			Author:      author,
			DocumentURL: docURL,
			PreachedOn:  preachedOn,
		}
		if err := db.Create(&row).Error; err != nil {
			log.Printf("  create failed for %q: %v", title, err)
			continue
		}
		log.Printf("  + %s — %s (%s)", preachedOn.Format("Jan 2 2006"), title, author)
		created++
	}

	log.Printf("Migration done. Created %d, skipped %d (already present).", created, skipped)
	return nil
}

// splitTitleAuthor pulls " by Author Name" off the end of a label.
// e.g. "Third Sunday of Lent by Deacon Bob" → ("Third Sunday of Lent", "Deacon Bob").
func splitTitleAuthor(label string) (string, string) {
	label = strings.TrimSpace(label)
	// Case-insensitive " by " match that preserves the preceding title.
	lower := strings.ToLower(label)
	idx := strings.LastIndex(lower, " by ")
	if idx <= 0 {
		return label, ""
	}
	title := strings.TrimSpace(label[:idx])
	author := strings.TrimSpace(label[idx+4:])
	// Strip trailing punctuation / parens from author.
	author = strings.Trim(author, " .,:;()[]")
	return title, author
}

// guessPreachedOn tries to pick a sensible preached_on date from a homily
// title. For the WordPress import, titles name a liturgical Sunday ("Third
// Sunday of Lent"), which doesn't pin a specific year — so we fall back to
// the most recent Easter-season date we can reasonably infer. If no better
// guess is available, we use "today" so the row is at least orderable.
func guessPreachedOn(title string) time.Time {
	// Matches an explicit date like "March 24, 2024" if the import happens
	// to preserve one. Otherwise returns today.
	explicit := regexp.MustCompile(`(?i)(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},\s+\d{4}`)
	if m := explicit.FindString(title); m != "" {
		if t, err := time.Parse("January 2, 2006", m); err == nil {
			return t
		}
	}
	return time.Now()
}
