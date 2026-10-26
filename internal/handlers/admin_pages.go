package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminPagesHandler owns the admin UI for CMS pages (list, edit, create,
// delete, save blocks) and the JSON API behind them.
type AdminPagesHandler struct {
	db *gorm.DB
}

func NewAdminPagesHandler(db *gorm.DB) *AdminPagesHandler {
	return &AdminPagesHandler{db: db}
}

// adminPageRow is one entry in the admin pages list.
type adminPageRow struct {
	models.Page
	BlockCount int
	Depth      int
}

// List renders /admin/pages — the pages index.
func (h *AdminPagesHandler) List(c *gin.Context) {
	var pages []models.Page
	if err := h.db.Order("parent_slug ASC, slug ASC").Find(&pages).Error; err != nil {
		c.String(http.StatusInternalServerError, "Failed to load pages")
		return
	}
	rows := make([]adminPageRow, 0, len(pages))
	for _, p := range pages {
		rows = append(rows, adminPageRow{
			Page:       p,
			BlockCount: len(p.Blocks),
			Depth:      strings.Count(p.Slug, "/"),
		})
	}

	data := h.pageData(c, "admin-pages", "Pages — Admin — OLV Parish")
	data["pages"] = rows
	c.HTML(http.StatusOK, "admin-pages.html", data)
}

// Edit renders /admin/pages/:slug — the block editor.
func (h *AdminPagesHandler) Edit(c *gin.Context) {
	slug := strings.Trim(c.Param("slug"), "/")
	var page models.Page
	err := h.db.Where("slug = ?", slug).First(&page).Error
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/pages")
		return
	}

	// Pass blocks as JSON. We put it in a data-* attribute on the editor
	// element rather than inside a <script> tag because html/template's
	// JS-context escaping chokes on raw JSON there; attribute escaping is
	// well-behaved and the client reads it with dataset.
	blocksJSON, _ := json.Marshal(page.Blocks)
	if len(blocksJSON) == 0 {
		blocksJSON = []byte("[]")
	}

	data := h.pageData(c, "admin-page-edit", "Edit — "+page.Title+" — Admin")
	data["pageRow"] = page
	data["blocksJSON"] = string(blocksJSON)
	c.HTML(http.StatusOK, "admin-page-edit.html", data)
}

// APISave accepts the full new block list and page meta from the editor.
func (h *AdminPagesHandler) APISave(c *gin.Context) {
	slug := strings.Trim(c.Param("slug"), "/")
	var req struct {
		Title           string         `json:"title"`
		NavLabel        string         `json:"nav_label"`
		MetaDescription string         `json:"meta_description"`
		HeroImageSlot   string         `json:"hero_image_slot"`
		Blocks          []models.Block `json:"blocks"`
		Publish         bool           `json:"publish"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var page models.Page
	if err := h.db.Where("slug = ?", slug).First(&page).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}

	page.Title = strings.TrimSpace(req.Title)
	if page.Title == "" {
		page.Title = page.Slug
	}
	page.NavLabel = req.NavLabel
	page.MetaDescription = req.MetaDescription
	page.HeroImageSlot = req.HeroImageSlot
	page.Blocks = models.Blocks(req.Blocks)
	if req.Publish && page.PublishedAt == nil {
		now := time.Now()
		page.PublishedAt = &now
	}
	if updatedBy := c.GetUint("user_id"); updatedBy != 0 {
		page.UpdatedByID = &updatedBy
	}

	if err := h.db.Save(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save page"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Saved", "page": page})
}

// APICreate makes a new blank Page.
func (h *AdminPagesHandler) APICreate(c *gin.Context) {
	var req struct {
		Slug       string `json:"slug" binding:"required"`
		ParentSlug string `json:"parent_slug"`
		Title      string `json:"title" binding:"required"`
		NavLabel   string `json:"nav_label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slug := strings.Trim(req.Slug, "/")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Slug is required"})
		return
	}

	var existing models.Page
	if err := h.db.Where("slug = ?", slug).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A page with that slug already exists"})
		return
	}

	page := models.Page{
		Slug:       slug,
		ParentSlug: req.ParentSlug,
		Title:      req.Title,
		NavLabel:   req.NavLabel,
		Blocks:     models.Blocks{},
	}
	if err := h.db.Create(&page).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create page"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Created", "page": page})
}

// APIDelete soft-deletes a page.
func (h *AdminPagesHandler) APIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page id"})
		return
	}
	result := h.db.Delete(&models.Page{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete page"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// APIPublish toggles a page's publication state.
func (h *AdminPagesHandler) APIPublish(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page id"})
		return
	}
	var req struct {
		Publish bool `json:"publish"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	update := map[string]interface{}{"published_at": nil}
	if req.Publish {
		update["published_at"] = time.Now()
	}
	if err := h.db.Model(&models.Page{}).Where("id = ?", id).Updates(update).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update page"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

// pageData is the admin-pages equivalent of AdminHandler.pageData — it bundles
// the common fields every admin template expects.
func (h *AdminPagesHandler) pageData(c *gin.Context, page, title string) gin.H {
	user, _ := c.Get("user")
	var unread int64
	h.db.Model(&models.ContactMessage{}).Where("read_at IS NULL").Count(&unread)
	return gin.H{
		"title":          title,
		"page":           page,
		"user":           user,
		"unreadMessages": unread,
		"version":        assetVersion,
	}
}
