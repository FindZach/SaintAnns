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

// loadNavChildren returns the direct children of parentSlug (not grandchildren),
// plus any hand-added extraChildren, sorted alphabetically. If more items exist
// than will fit comfortably in a dropdown, the first N show + a "See all →"
// overflow link to the hub page where every item is listed as a card.
//
// We deliberately don't walk grandchildren here (so Sacraments doesn't list
// all 7 individual sacraments in the nav — the hub page's child_pages block
// does that with icons and descriptions). Keeps the dropdowns scannable.
func loadNavChildren(db *gorm.DB, parentSlug string) []NavEntry {
	var pages []models.Page
	if err := db.Where(
		"parent_slug = ? AND published_at IS NOT NULL AND published_at <= NOW()",
		parentSlug,
	).Order("COALESCE(NULLIF(nav_label, ''), title) ASC").Find(&pages).Error; err != nil {
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
		if !seen[strings.Trim(extra.Href, "/")] {
			children = append(children, extra)
		}
	}

	// Final alphabetical sort so extraChildren interleave with Page rows
	// instead of always sitting at the bottom.
	sort.SliceStable(children, func(i, j int) bool {
		return strings.ToLower(children[i].Label) < strings.ToLower(children[j].Label)
	})

	// Cap at 7 for scannability. When truncated, surface a "See all →" link
	// to the hub page so nothing is hidden — just moved one click deeper.
	const maxDropdown = 7
	if len(children) > maxDropdown {
		seeAll := NavEntry{
			Label: "See all →",
			Href:  "/" + parentSlug + "/",
		}
		children = children[:maxDropdown]
		children = append(children, seeAll)
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
