package handlers

import (
	"sort"
	"strings"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// NavEntry is one rendered nav link (template-friendly). Children, when set,
// render as a dropdown underneath the top-level link.
type NavEntry struct {
	Label    string
	Href     string
	External bool
	Children []NavEntry
}

// extraChildren lets us surface routes that aren't backed by a Page row.
// Keyed by the top-level nav slug (parent); each entry becomes a child link.
var extraChildren = map[string][]NavEntry{
	"liturgy-and-sacraments": {
		{Label: "Mass Times", Href: "/liturgy-and-sacraments/mass-times/"},
		{Label: "Confession Times", Href: "/liturgy-and-sacraments/confession-times/"},
	},
	"events-news": {
		{Label: "Calendar", Href: "/events-news/calendar/"},
	},
}

// LoadPublicNav returns the top-level enabled nav items in sort order with
// their child pages attached as a dropdown. Falls back to a static list if
// the DB is unavailable.
func LoadPublicNav(db *gorm.DB) []NavEntry {
	if db == nil {
		return fallbackNav()
	}
	var items []models.NavItem
	if err := db.Where("parent_id IS NULL AND enabled = ?", true).
		Order("sort ASC").Find(&items).Error; err != nil || len(items) == 0 {
		return fallbackNav()
	}

	out := make([]NavEntry, 0, len(items))
	for _, item := range items {
		entry := NavEntry{Label: item.Label, Href: item.Href, External: item.External}
		if !item.External {
			if parent := slugFromHref(item.Href); parent != "" {
				entry.Children = loadNavChildren(db, parent)
			}
		}
		out = append(out, entry)
	}
	return out
}

// loadNavChildren returns every published descendant page under parentSlug,
// plus any hand-added extraChildren, sorted by depth then nav_label so the
// most direct children appear first.
func loadNavChildren(db *gorm.DB, parentSlug string) []NavEntry {
	var pages []models.Page
	// Direct children (parent_slug = X) plus one more level (parent_slug like X/%)
	// gives us e.g. all 7 sacraments nested under /liturgy-and-sacraments/sacraments.
	if err := db.Where(
		"(parent_slug = ? OR parent_slug LIKE ?) AND published_at IS NOT NULL AND published_at <= NOW()",
		parentSlug, parentSlug+"/%",
	).Order("slug ASC").Find(&pages).Error; err != nil {
		pages = nil
	}

	children := make([]NavEntry, 0, len(pages)+2)
	seen := make(map[string]bool)
	for _, p := range pages {
		if seen[p.Slug] {
			continue
		}
		seen[p.Slug] = true
		label := p.NavLabel
		if label == "" {
			label = p.Title
		}
		children = append(children, NavEntry{
			Label: label,
			Href:  "/" + p.Slug + "/",
		})
	}
	// Append extra child routes (e.g. Calendar, Mass Times) that aren't Page rows.
	for _, extra := range extraChildren[parentSlug] {
		children = append(children, extra)
	}

	// Sort by depth (fewer slashes = closer to parent) then label so hub
	// pages like "Sacraments" come before the 7 individual sacraments.
	sort.SliceStable(children, func(i, j int) bool {
		di := strings.Count(children[i].Href, "/") - 2 // discount leading + trailing
		dj := strings.Count(children[j].Href, "/") - 2
		if di != dj {
			return di < dj
		}
		return children[i].Label < children[j].Label
	})

	// Cap to keep dropdowns scannable.
	const maxDropdown = 14
	if len(children) > maxDropdown {
		children = children[:maxDropdown]
	}
	return children
}

// slugFromHref turns "/about/" into "about", "/" into "". Used to find the
// parent_slug for the children query.
func slugFromHref(href string) string {
	return strings.Trim(href, "/")
}

func fallbackNav() []NavEntry {
	return []NavEntry{
		{Label: "Home", Href: "/"},
		{Label: "Mass Times", Href: "/liturgy-and-sacraments/mass-times/"},
		{Label: "Contact", Href: "/contact/"},
	}
}
