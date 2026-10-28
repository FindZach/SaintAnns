package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// Seed inserts the baseline content a brand-new deployment starts with:
// Mass times, confession times, nav tree, and a few starter Pages. It is
// idempotent — rows that are already present (keyed by their natural key) are
// skipped, so running --seed on a populated database is harmless.
func Seed(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("seed: no database connection")
	}
	log.Println("Seeding baseline parish content...")

	if err := seedMassSchedule(db); err != nil {
		return err
	}
	if err := seedConfessionTimes(db); err != nil {
		return err
	}
	if err := seedPages(db); err != nil {
		return err
	}
	if err := seedNav(db); err != nil {
		return err
	}

	log.Println("Seed complete")
	return nil
}

func seedMassSchedule(db *gorm.DB) error {
	// Pulled from the current olv14843.org Mass times.
	rows := []models.MassSchedule{
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Saturday), TimeOfDay: "16:30", Label: "Vigil", Sort: 10},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Sunday), TimeOfDay: "06:45", Sort: 20},
		{Church: models.ChurchStMary, DayOfWeek: int(time.Sunday), TimeOfDay: "08:45", Sort: 30},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Sunday), TimeOfDay: "10:45", Label: "Children's Liturgy of the Word", Sort: 40},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Monday), TimeOfDay: "12:10", StreamedURL: "https://www.facebook.com/", Sort: 50},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Tuesday), TimeOfDay: "12:10", StreamedURL: "https://www.facebook.com/", Sort: 60},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Thursday), TimeOfDay: "12:10", StreamedURL: "https://www.facebook.com/", Sort: 70},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Friday), TimeOfDay: "12:10", StreamedURL: "https://www.facebook.com/", Sort: 80},
	}
	for _, row := range rows {
		var existing models.MassSchedule
		err := db.Where("church = ? AND day_of_week = ? AND time_of_day = ?",
			row.Church, row.DayOfWeek, row.TimeOfDay).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&row).Error; err != nil {
				return fmt.Errorf("seed mass schedule: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("seed mass schedule (lookup): %w", err)
		}
	}
	return nil
}

func seedConfessionTimes(db *gorm.DB) error {
	rows := []models.ConfessionTime{
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Tuesday), StartOfDay: "11:30", EndOfDay: "12:00", Sort: 10},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Thursday), StartOfDay: "11:30", EndOfDay: "12:00", Sort: 20},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Saturday), StartOfDay: "15:00", EndOfDay: "16:15", Season: models.SeasonAdvent, Label: "Advent", Sort: 30},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Saturday), StartOfDay: "15:00", EndOfDay: "16:15", Season: models.SeasonLent, Label: "Lent", Sort: 31},
		{Church: models.ChurchStAnn, DayOfWeek: int(time.Saturday), StartOfDay: "15:30", EndOfDay: "16:15", Season: models.SeasonOrdinary, Label: "Outside Advent & Lent", Sort: 40},
	}
	for _, row := range rows {
		var existing models.ConfessionTime
		err := db.Where("church = ? AND day_of_week = ? AND start_of_day = ? AND season = ?",
			row.Church, row.DayOfWeek, row.StartOfDay, row.Season).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&row).Error; err != nil {
				return fmt.Errorf("seed confession times: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("seed confession times (lookup): %w", err)
		}
	}
	return nil
}

// seedPages creates the starter Page rows for the top-level nav sections.
// Each is published and has one placeholder rich_text block the admin can
// replace once the editor ships in Phase 4.
func seedPages(db *gorm.DB) error {
	now := time.Now()
	type pageSeed struct {
		slug, parent, title, nav, placeholder string
	}
	seeds := []pageSeed{
		{"about", "", "About the Parish", "About",
			"Our Lady of the Valley Parish serves St. Ann in Hornell and St. Mary in Rexville. Replace this placeholder with the parish's own words."},
		{"liturgy-and-sacraments", "", "Liturgy and Sacraments", "Liturgy & Sacraments",
			"Through the sacraments we encounter Christ. Baptism, Reconciliation, Eucharist, Confirmation, Anointing, Matrimony and Holy Orders. Replace this placeholder with pastoral copy."},
		{"minstries", "", "Ministries", "Ministries",
			"The ministries of Our Lady of the Valley welcome every parishioner into service. (The slug keeps the current site's spelling so inbound links don't break.)"},
		{"faith-formation", "", "Faith Formation", "Faith Formation",
			"Faith formation for children, youth, adults and candidates for the Rite of Christian Initiation."},
		{"events-news", "", "Events & News", "Events & News",
			"Upcoming parish events and recent news. Edit this page in the admin to add your own welcome; the live event list below updates whenever an event is added under Admin → Events."},
	}

	placeholderBlocks := func(text string) models.Blocks {
		data, _ := json.Marshal(map[string]string{"html": "<p>" + text + "</p>"})
		return models.Blocks{{Type: models.BlockRichText, Data: data}}
	}
	// events-news is the one seed page that gets a dynamic block alongside its
	// placeholder, so the "live list" promise is actually true out of the box.
	eventsNewsBlocks := func(text string) models.Blocks {
		intro, _ := json.Marshal(map[string]string{"html": "<p>" + text + "</p>"})
		eventList, _ := json.Marshal(map[string]interface{}{"limit": 10})
		return models.Blocks{
			{Type: models.BlockRichText, Data: intro},
			{Type: models.BlockEventList, Data: eventList},
		}
	}

	for _, s := range seeds {
		blocks := placeholderBlocks(s.placeholder)
		if s.slug == "events-news" {
			blocks = eventsNewsBlocks(s.placeholder)
		}
		var existing models.Page
		err := db.Where("slug = ?", s.slug).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			page := models.Page{
				Slug:        s.slug,
				ParentSlug:  s.parent,
				Title:       s.title,
				NavLabel:    s.nav,
				Blocks:      blocks,
				PublishedAt: &now,
			}
			if err := db.Create(&page).Error; err != nil {
				return fmt.Errorf("seed page %q: %w", s.slug, err)
			}
		} else if err != nil {
			return fmt.Errorf("seed page %q (lookup): %w", s.slug, err)
		}
	}
	return nil
}

// seedNav installs the top-level nav items. Children (per-page sub-nav) come
// from the Pages themselves in Phase 3.
func seedNav(db *gorm.DB) error {
	items := []models.NavItem{
		{Sort: 10, Label: "Home", Href: "/", Enabled: true},
		{Sort: 20, Label: "About", Href: "/about/", Enabled: true},
		{Sort: 30, Label: "Liturgy & Sacraments", Href: "/liturgy-and-sacraments/", Enabled: true},
		{Sort: 40, Label: "Ministries", Href: "/minstries/", Enabled: true},
		{Sort: 50, Label: "Faith Formation", Href: "/faith-formation/", Enabled: true},
		{Sort: 60, Label: "Events & News", Href: "/events-news/", Enabled: true},
		{Sort: 70, Label: "Contact", Href: "/contact/", Enabled: true},
	}
	for _, item := range items {
		var existing models.NavItem
		err := db.Where("parent_id IS NULL AND href = ?", item.Href).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&item).Error; err != nil {
				return fmt.Errorf("seed nav %q: %w", item.Label, err)
			}
		} else if err != nil {
			return fmt.Errorf("seed nav %q (lookup): %w", item.Label, err)
		}
	}
	return nil
}
