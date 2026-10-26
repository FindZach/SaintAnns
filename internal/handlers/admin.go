package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"
	_ "time/tzdata" // Bundles zone data, so parishZone loads even where the OS has none

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// adminListLimit caps the admin lists, which show the newest rows first.
const adminListLimit = 500

// parishZone is the time zone to show times in across the admin pages.
var parishZone = loadParishZone()

func loadParishZone() *time.Location {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Local
	}
	return zone
}

// AdminHandler serves the admin dashboard. Only users with the admin role
// reach it (see middleware.AdminPageMiddleware and AdminMiddleware).
type AdminHandler struct {
	db *gorm.DB
}

// adminStats are the Overview page's figures.
type adminStats struct {
	MessagesLast30 int64
	UnreadMessages int64
	Staff          int64
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// Dashboard is the admin Overview page.
func (h *AdminHandler) Dashboard(c *gin.Context) {
	since := time.Now().AddDate(0, 0, -30)
	var stats adminStats
	logFailure("count recent messages", h.db.Model(&models.ContactMessage{}).Where("created_at >= ?", since).Count(&stats.MessagesLast30).Error)
	logFailure("count unread messages", h.db.Model(&models.ContactMessage{}).Where("read_at IS NULL").Count(&stats.UnreadMessages).Error)
	logFailure("count staff", h.db.Model(&models.User{}).Count(&stats.Staff).Error)

	var messages []models.ContactMessage
	logFailure("load recent messages", h.db.Order("created_at DESC").Limit(5).Find(&messages).Error)
	inParishZone(messages)

	data := h.pageData(c, "admin", "Admin Dashboard - OLV Parish")
	data["stats"] = stats
	data["messages"] = messages
	c.HTML(http.StatusOK, "admin.html", data)
}

// Messages lists Contact page messages, newest first.
func (h *AdminHandler) Messages(c *gin.Context) {
	unreadOnly := c.Query("show") == "unread"
	query := h.db.Order("created_at DESC").Limit(adminListLimit)
	if unreadOnly {
		query = query.Where("read_at IS NULL")
	}
	var messages []models.ContactMessage
	logFailure("load messages", query.Find(&messages).Error)
	inParishZone(messages)

	var total int64
	logFailure("count messages", h.db.Model(&models.ContactMessage{}).Count(&total).Error)

	data := h.pageData(c, "admin-messages", "Messages - Admin - OLV Parish")
	data["messages"] = messages
	data["unreadOnly"] = unreadOnly
	data["totalMessages"] = total
	c.HTML(http.StatusOK, "admin-messages.html", data)
}

// MarkMessageRead marks a message read or unread, and returns the new unread count.
func (h *AdminHandler) MarkMessageRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}
	var req struct {
		Read bool `json:"read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var readAt interface{}
	if req.Read {
		readAt = time.Now()
	}
	result := h.db.Model(&models.ContactMessage{}).Where("id = ?", id).Update("read_at", readAt)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update message"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread": h.unreadMessages()})
}

// DeleteMessage soft-deletes a message so it can still be recovered.
func (h *AdminHandler) DeleteMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}
	result := h.db.Delete(&models.ContactMessage{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread": h.unreadMessages()})
}

// pageData is what every admin page passes to its template.
func (h *AdminHandler) pageData(c *gin.Context, page, title string) gin.H {
	user, _ := c.Get("user")
	return gin.H{
		"title":          title,
		"page":           page,
		"user":           user,
		"unreadMessages": h.unreadMessages(),
		"version":        assetVersion,
	}
}

func (h *AdminHandler) unreadMessages() int64 {
	var unread int64
	logFailure("count unread messages", h.db.Model(&models.ContactMessage{}).Where("read_at IS NULL").Count(&unread).Error)
	return unread
}

// inParishZone shows messages' times in the parish's time zone.
func inParishZone(messages []models.ContactMessage) {
	for i := range messages {
		messages[i].CreatedAt = messages[i].CreatedAt.In(parishZone)
	}
}

// logFailure logs a failed admin query; the page still shows what did load.
func logFailure(what string, err error) {
	if err != nil {
		log.Printf("Admin: failed to %s: %v", what, err)
	}
}
