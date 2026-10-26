package handlers

import (
	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// NavEntry is one rendered nav link (template-friendly).
type NavEntry struct {
	Label    string
	Href     string
	External bool
}

// LoadPublicNav returns the top-level enabled nav items in sort order. Falls
// back to a static list if the DB is unavailable or empty.
func LoadPublicNav(db *gorm.DB) []NavEntry {
	if db != nil {
		var items []models.NavItem
		if err := db.Where("parent_id IS NULL AND enabled = ?", true).
			Order("sort ASC").Find(&items).Error; err == nil && len(items) > 0 {
			out := make([]NavEntry, 0, len(items))
			for _, item := range items {
				out = append(out, NavEntry{Label: item.Label, Href: item.Href, External: item.External})
			}
			return out
		}
	}
	// Fallback — keeps the shell navigable in degraded mode.
	return []NavEntry{
		{"Home", "/", false},
		{"Mass Times", "/liturgy-and-sacraments/mass-times/", false},
		{"Contact", "/contact/", false},
	}
}
