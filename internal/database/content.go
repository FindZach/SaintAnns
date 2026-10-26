package database

import (
	"fmt"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// EnsureContentSchema is the additive migration path for Phase 2's content
// model. It runs on every startup so a production deploy picks up new tables
// without a manual migration. AutoMigrate only adds — it never drops data.
func EnsureContentSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(contentModels()...); err != nil {
		return fmt.Errorf("failed to ensure content schema: %w", err)
	}
	return nil
}

// contentModels is the list of Phase 2 content tables. Keep it alongside the
// full Migrate() in database.go so both stay in sync.
func contentModels() []interface{} {
	return []interface{}{
		&models.Asset{},
		&models.Page{},
		&models.MassSchedule{},
		&models.ConfessionTime{},
		&models.Bulletin{},
		&models.Homily{},
		&models.Event{},
		&models.NewsPost{},
		&models.HeroSlide{},
		&models.Album{},
		&models.AlbumPhoto{},
		&models.Staff{},
		&models.Ministry{},
		&models.NavItem{},
		&models.CourierHeadline{},
		&models.ParishRegistration{},
	}
}
