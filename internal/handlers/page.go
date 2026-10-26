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

// Render looks up the Page row that matches the request URL's slug and renders
// it. Missing pages return 404. Degraded mode (no DB) returns 404 everywhere
// NoRoute catches, which is the right answer — we don't want to serve stale
// cached content either.
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

	var page models.Page
	if err := h.db.Where("slug = ?", slug).First(&page).Error; err != nil {
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
