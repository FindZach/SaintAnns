package database

import (
	"fmt"
	"log"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// GroupPagePhotos turns runs of three or more stacked pictures on every page
// into photo-gallery blocks (see models.GroupPhotoRuns). The import left
// album pages as long columns of medium-sized images; a gallery fills the
// width of any screen and opens pictures full-size.
//
// Idempotent: a page with no such run is left untouched.
func GroupPagePhotos(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("group photos: no database connection")
	}
	log.Println("Grouping stacked pictures into photo galleries ...")
	var pages []models.Page
	if err := db.Find(&pages).Error; err != nil {
		return fmt.Errorf("group photos: list pages: %w", err)
	}
	updated := 0
	for i := range pages {
		grouped, changed := models.GroupPhotoRuns(pages[i].Blocks, 3)
		if !changed {
			continue
		}
		before := len(pages[i].Blocks)
		if err := db.Model(&pages[i]).Update("blocks", grouped).Error; err != nil {
			log.Printf("  %s: save failed: %v", pages[i].Slug, err)
			continue
		}
		log.Printf("  %s: %d blocks -> %d", pages[i].Slug, before, len(grouped))
		updated++
	}
	log.Printf("Done. %d page(s) updated.", updated)
	return nil
}
