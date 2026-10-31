package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
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
	data["parentOptions"] = parentOptionsFrom(pages, "")
	c.HTML(http.StatusOK, "admin-pages.html", data)
}

type parentOption struct {
	Slug, Label string
}

// parentOptionsFrom lists pages that can hold children, excluding `self` and
// its descendants. Labels are indented by depth so the tree is readable.
func parentOptionsFrom(pages []models.Page, self string) []parentOption {
	sorted := append([]models.Page(nil), pages...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })
	out := make([]parentOption, 0, len(sorted))
	for _, p := range sorted {
		if self != "" && (p.Slug == self || strings.HasPrefix(p.Slug, self+"/")) {
			continue
		}
		depth := strings.Count(p.Slug, "/")
		if depth > 1 {
			continue // keep the menu to sections and their direct sub-pages
		}
		out = append(out, parentOption{Slug: p.Slug, Label: strings.Repeat("— ", depth) + p.Title})
	}
	return out
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

	var all []models.Page
	h.db.Order("slug ASC").Find(&all)

	data := h.pageData(c, "admin-page-edit", "Edit — "+page.Title+" — Admin")
	data["pageRow"] = page
	data["blocksJSON"] = string(blocksJSON)
	data["parentOptions"] = parentOptionsFrom(all, page.Slug)
	c.HTML(http.StatusOK, "admin-page-edit.html", data)
}

// APISave accepts the full new block list and page meta from the editor.
func (h *AdminPagesHandler) APISave(c *gin.Context) {
	slug := strings.Trim(c.Param("slug"), "/")
	var req struct {
		Title    string `json:"title"`
		NavLabel string `json:"nav_label"`
		// Pointers so a request that omits these leaves the stored values alone.
		ParentSlug      *string        `json:"parent_slug"`
		MetaDescription *string        `json:"meta_description"`
		HeroImageSlot   *string        `json:"hero_image_slot"`
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
	if req.ParentSlug != nil {
		parent := strings.Trim(*req.ParentSlug, "/")
		if parent == page.Slug || strings.HasPrefix(parent, page.Slug+"/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A page can't be placed under itself"})
			return
		}
		page.ParentSlug = parent
	}
	if req.MetaDescription != nil {
		page.MetaDescription = strings.TrimSpace(*req.MetaDescription)
	}
	if req.HeroImageSlot != nil {
		page.HeroImageSlot = *req.HeroImageSlot
	}
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
	parent := strings.Trim(req.ParentSlug, "/")
	slug := strings.Trim(req.Slug, "/")
	if slug == "" || !validSlugPath(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The address can only use lowercase letters, numbers and dashes"})
		return
	}
	if parent != "" && !strings.HasPrefix(slug, parent+"/") {
		slug = parent + "/" + slug
	}

	var existing models.Page
	if err := h.db.Unscoped().Where("slug = ?", slug).First(&existing).Error; err == nil {
		if existing.DeletedAt.Valid {
			// A previously deleted page held this address; clear it out so the
			// unique index doesn't block the new page.
			h.db.Unscoped().Delete(&existing)
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "A page already exists at /" + slug + "/"})
			return
		}
	}

	page := models.Page{
		Slug:       slug,
		ParentSlug: parent,
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
