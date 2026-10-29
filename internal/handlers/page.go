package handlers

import (
	"html/template"
	"net/http"
	"strings"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PageHandler serves CMS-managed pages. One handler covers every URL that
// isn't matched by a specific route (About, Ministries, Faith Formation,
// individual sacraments, etc.). It's wired as the gin NoRoute catch-all.
type PageHandler struct {
	db *gorm.DB
}

func NewPageHandler(db *gorm.DB) *PageHandler {
	return &PageHandler{db: db}
}

// shortAliases maps WordPress-style shortcut permalinks to the canonical
// nested slug for the same page. The live olv14843.org site exposes
// `/holy-communion`, `/first-penance`, etc. as pretty permalinks that resolve
// to the full `/liturgy-and-sacraments/sacraments/...` paths; the imported
// content kept those shortcuts, so we honor them here with a 301 redirect
// to the canonical URL. Entries are the shortcut (no leading slash) → canonical slug.
var shortAliases = map[string]string{
	"holy-communion":        "liturgy-and-sacraments/sacraments/holy-communion",
	"first-communion":       "liturgy-and-sacraments/sacraments/holy-communion",
	"first-penance":         "liturgy-and-sacraments/sacraments/reconciliation",
	"reconciliation":        "liturgy-and-sacraments/sacraments/reconciliation",
	"baptism":               "liturgy-and-sacraments/sacraments/baptism",
	"confirmation":          "liturgy-and-sacraments/sacraments/confirmation",
	"matrimony":             "liturgy-and-sacraments/sacraments/matrimony",
	"holy-orders":           "liturgy-and-sacraments/sacraments/holy-orders",
	"anointing-of-the-sick": "liturgy-and-sacraments/sacraments/anointing-of-the-sick",
	"sacraments":            "liturgy-and-sacraments/sacraments",
	"mass-times":            "liturgy-and-sacraments/mass-times",
	"confession-times":      "liturgy-and-sacraments/confession-times",
	"perpetual-adoration":   "liturgy-and-sacraments/perpetural-adoration",
	"perpetural-adoration":  "liturgy-and-sacraments/perpetural-adoration",
	"ministries":            "minstries",
	"parish-registration":   "about/parish-registration",
	"bulletins":             "about/bulletins",
	"donate":                "about/donate",
	"staff":                 "about/staff",
	"photo-albums":          "about/photo-albums",
}

// Render looks up the Page row that matches the request URL's slug and renders
// it. Missing pages try a basename / shortAliases redirect before 404ing, so
// WordPress-style short permalinks the import preserved still work.
func (h *PageHandler) Render(c *gin.Context) {
	slug := slugFromPath(c.Request.URL.Path)
	if slug == "" {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	if h.db == nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	// Dynamic overrides: these slugs have Page rows left from the WordPress
	// import, but we render them from live DB tables instead so staff edits
	// via /admin/homilies etc. actually show up.
	switch slug {
	case "liturgy-and-sacraments/last-weekends-homily":
		h.renderLatestHomily(c)
		return
	}

	var page models.Page
	err := h.db.Where("slug = ?", slug).First(&page).Error
	if err != nil {
		// Event detail: /events-news/<slug>/ looks up the Event row by its
		// trailing slug and renders the event-detail template. Must come
		// before shortAliases so an event doesn't get mis-redirected to a
		// page with the same final segment.
		if eventSlug := strings.TrimPrefix(slug, "events-news/"); eventSlug != slug && eventSlug != "" && !strings.Contains(eventSlug, "/") {
			if h.renderEventDetail(c, eventSlug) {
				return
			}
		}
		// Fallback: short permalink → canonical nested slug.
		if canonical := resolveShortAlias(h.db, slug); canonical != "" {
			c.Redirect(http.StatusMovedPermanently, "/"+canonical+"/")
			return
		}
		c.String(http.StatusNotFound, "Not found")
		return
	}
	if !page.IsPublished() {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	rendered := RenderBlocks(page.Blocks, h.db)

	data := gin.H{
		"title":        page.Title + " — Our Lady of the Valley Parish",
		"page":         "cms",
		"pageData":     page,
		"renderedBody": template.HTML(rendered),
		"version":      assetVersion,
	}
	if page.MetaDescription != "" {
		data["metaDescription"] = page.MetaDescription
	}

	c.HTML(http.StatusOK, "page.html", data)
}

// renderLatestHomily serves /liturgy-and-sacraments/last-weekends-homily/,
// pulling the most recent Homily by preached_on date. If staff have posted
// more than one, the next few show as a "Recent homilies" list below.
func (h *PageHandler) renderLatestHomily(c *gin.Context) {
	var latest models.Homily
	err := h.db.Order("preached_on DESC").Limit(1).First(&latest).Error
	hasHomily := err == nil

	var recent []models.Homily
	if hasHomily {
		_ = h.db.Where("id <> ?", latest.ID).
			Order("preached_on DESC").
			Limit(6).
			Find(&recent).Error
	}

	data := gin.H{
		"title":     "Last Weekend's Homily — Our Lady of the Valley Parish",
		"page":      "homily",
		"hasHomily": hasHomily,
		"homily":    latest,
		"recent":    recent,
		"version":   assetVersion,
	}
	c.HTML(http.StatusOK, "homily.html", data)
}

// renderEventDetail resolves slug to an Event row and renders the event
// template. Returns true when it served the response (either a 200 render
// or its own 404); false means the slug didn't match any event and the
// caller should keep trying other fallbacks. The flyer asset is preloaded
// so the template can render it inline.
func (h *PageHandler) renderEventDetail(c *gin.Context, slug string) bool {
	var ev models.Event
	err := h.db.Preload("FlyerAsset").Where("slug = ?", slug).First(&ev).Error
	if err != nil {
		return false
	}
	data := gin.H{
		"title":   ev.Title + " — Our Lady of the Valley Parish",
		"page":    "event-detail",
		"event":   ev,
		"version": assetVersion,
	}
	c.HTML(http.StatusOK, "event.html", data)
	return true
}

// resolveShortAlias returns the canonical slug for a shortcut URL, or empty.
// It consults the hand-maintained shortAliases map first, then falls back to
// a last-segment match: if the shortcut has no slashes and exactly one
// published page's slug ends with "/<shortcut>", that one wins.
func resolveShortAlias(db *gorm.DB, shortcut string) string {
	shortcut = strings.Trim(shortcut, "/")
	if shortcut == "" || strings.Contains(shortcut, "/") {
		return "" // only consider single-segment shortcuts
	}
	if canonical, ok := shortAliases[shortcut]; ok {
		return canonical
	}
	var matches []models.Page
	if err := db.Where("slug LIKE ? AND published_at IS NOT NULL AND published_at <= NOW()", "%/"+shortcut).
		Limit(2).Find(&matches).Error; err != nil {
		return ""
	}
	if len(matches) == 1 {
		return matches[0].Slug
	}
	return ""
}

// slugFromPath turns "/about/parish-registration/" into
// "about/parish-registration" so it matches Page.Slug. Trailing slashes are
// optional both in the stored slug and the URL.
func slugFromPath(path string) string {
	path = strings.Trim(path, "/")
	// Guard against admin / static leaking through — those routes are
	// registered explicitly and should never hit the catch-all, but be safe.
	switch {
	case path == "":
		return ""
	case strings.HasPrefix(path, "admin"),
		strings.HasPrefix(path, "api"),
		strings.HasPrefix(path, "static"),
		strings.HasPrefix(path, "health"):
		return ""
	}
	return path
}
