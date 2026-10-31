package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminStructuredHandler bundles the admin UI and CRUD for the typed content
// tables: Mass times, confession times, homilies, bulletins, events. Each
// table is small and single-purpose, so they share one handler.
type AdminStructuredHandler struct {
	db *gorm.DB
}

func NewAdminStructuredHandler(db *gorm.DB) *AdminStructuredHandler {
	return &AdminStructuredHandler{db: db}
}

// adminPageData is the common bundle every admin template needs.
func (h *AdminStructuredHandler) pageData(c *gin.Context, page, title string) gin.H {
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

// ---------- Mass Times ----------

func (h *AdminStructuredHandler) MassTimesPage(c *gin.Context) {
	rows := LoadMassSchedule(h.db)
	data := h.pageData(c, "admin-mass-times", "Mass Times — Admin — OLV Parish")
	data["rows"] = rows
	data["churchOptions"] = churchOptions()
	c.HTML(http.StatusOK, "admin-mass-times.html", data)
}

type massTimeRequest struct {
	Church      string `json:"church" binding:"required"`
	DayOfWeek   int    `json:"day_of_week"`
	TimeOfDay   string `json:"time_of_day" binding:"required"`
	Label       string `json:"label"`
	Notes       string `json:"notes"`
	StreamedURL string `json:"streamed_url"`
	Sort        int    `json:"sort"`
}

func (h *AdminStructuredHandler) MassTimeAPICreate(c *gin.Context) {
	var req massTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := models.MassSchedule{
		Church: req.Church, DayOfWeek: req.DayOfWeek, TimeOfDay: req.TimeOfDay,
		Label: req.Label, Notes: req.Notes, StreamedURL: req.StreamedURL, Sort: req.Sort,
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"row": row})
}

func (h *AdminStructuredHandler) MassTimeAPIUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req massTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"church":       req.Church,
		"day_of_week":  req.DayOfWeek,
		"time_of_day":  req.TimeOfDay,
		"label":        req.Label,
		"notes":        req.Notes,
		"streamed_url": req.StreamedURL,
		"sort":         req.Sort,
	}
	if err := h.db.Model(&models.MassSchedule{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) MassTimeAPIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.MassSchedule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// MassTimesReorder accepts an ordered list of ids and rewrites each row's sort.
func (h *AdminStructuredHandler) MassTimesReorder(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for i, id := range req.IDs {
		h.db.Model(&models.MassSchedule{}).Where("id = ?", id).Update("sort", (i+1)*10)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Reordered"})
}

// ---------- Confession Times ----------

func (h *AdminStructuredHandler) ConfessionsPage(c *gin.Context) {
	rows := LoadConfessionTimes(h.db)
	data := h.pageData(c, "admin-confessions", "Confession Times — Admin — OLV Parish")
	data["rows"] = rows
	data["churchOptions"] = churchOptions()
	data["seasonOptions"] = seasonOptions()
	c.HTML(http.StatusOK, "admin-confessions.html", data)
}

type confessionRequest struct {
	Church     string `json:"church" binding:"required"`
	DayOfWeek  int    `json:"day_of_week"`
	Label      string `json:"label"`
	StartOfDay string `json:"start_of_day" binding:"required"`
	EndOfDay   string `json:"end_of_day" binding:"required"`
	Season     string `json:"season"`
	Notes      string `json:"notes"`
	Sort       int    `json:"sort"`
}

func (h *AdminStructuredHandler) ConfessionAPICreate(c *gin.Context) {
	var req confessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row := models.ConfessionTime{
		Church: req.Church, DayOfWeek: req.DayOfWeek, Label: req.Label,
		StartOfDay: req.StartOfDay, EndOfDay: req.EndOfDay,
		Season: req.Season, Notes: req.Notes, Sort: req.Sort,
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"row": row})
}

func (h *AdminStructuredHandler) ConfessionAPIUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req confessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"church":       req.Church,
		"day_of_week":  req.DayOfWeek,
		"label":        req.Label,
		"start_of_day": req.StartOfDay,
		"end_of_day":   req.EndOfDay,
		"season":       req.Season,
		"notes":        req.Notes,
		"sort":         req.Sort,
	}
	if err := h.db.Model(&models.ConfessionTime{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) ConfessionAPIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.ConfessionTime{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ---------- Homilies ----------

func (h *AdminStructuredHandler) HomiliesPage(c *gin.Context) {
	var homilies []models.Homily
	h.db.Order("preached_on DESC").Find(&homilies)
	data := h.pageData(c, "admin-homilies", "Homilies — Admin — OLV Parish")
	data["homilies"] = homilies
	c.HTML(http.StatusOK, "admin-homilies.html", data)
}

type homilyRequest struct {
	PreachedOn  string `json:"preached_on" binding:"required"` // YYYY-MM-DD
	Title       string `json:"title" binding:"required"`
	Author      string `json:"author"`
	Body        string `json:"body"`
	AudioURL    string `json:"audio_url"`
	VideoURL    string `json:"video_url"`
	DocumentURL string `json:"document_url"`
}

func (h *AdminStructuredHandler) HomilyAPICreate(c *gin.Context) {
	var req homilyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preached, err := time.Parse("2006-01-02", req.PreachedOn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "preached_on must be YYYY-MM-DD"})
		return
	}
	row := models.Homily{
		PreachedOn: preached, Title: req.Title, Author: req.Author,
		Body: req.Body, AudioURL: req.AudioURL, VideoURL: req.VideoURL,
		DocumentURL: req.DocumentURL,
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"row": row})
}

func (h *AdminStructuredHandler) HomilyAPIUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req homilyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preached, err := time.Parse("2006-01-02", req.PreachedOn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "preached_on must be YYYY-MM-DD"})
		return
	}
	updates := map[string]interface{}{
		"preached_on":  preached,
		"title":        req.Title,
		"author":       req.Author,
		"body":         req.Body,
		"audio_url":    req.AudioURL,
		"video_url":    req.VideoURL,
		"document_url": req.DocumentURL,
	}
	if err := h.db.Model(&models.Homily{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) HomilyAPIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.Homily{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ---------- Bulletins ----------

func (h *AdminStructuredHandler) BulletinsPage(c *gin.Context) {
	var bulletins []models.Bulletin
	h.db.Preload("Asset").Order("published_on DESC").Find(&bulletins)
	data := h.pageData(c, "admin-bulletins", "Bulletins — Admin — OLV Parish")
	data["bulletins"] = bulletins
	c.HTML(http.StatusOK, "admin-bulletins.html", data)
}

type bulletinRequest struct {
	PublishedOn string `json:"published_on" binding:"required"`
	Label       string `json:"label" binding:"required"`
	AssetID     uint   `json:"asset_id" binding:"required"`
}

func (h *AdminStructuredHandler) BulletinAPICreate(c *gin.Context) {
	var req bulletinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	published, err := time.Parse("2006-01-02", req.PublishedOn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "published_on must be YYYY-MM-DD"})
		return
	}
	row := models.Bulletin{PublishedOn: published, Label: req.Label, AssetID: req.AssetID}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"row": row})
}

// BulletinAPIUpdate edits a bulletin's date and label, and optionally swaps
// its PDF (asset_id 0 keeps the current file).
func (h *AdminStructuredHandler) BulletinAPIUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req struct {
		PublishedOn string `json:"published_on" binding:"required"`
		Label       string `json:"label" binding:"required"`
		AssetID     uint   `json:"asset_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Date and label are required"})
		return
	}
	published, err := time.Parse("2006-01-02", req.PublishedOn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "published_on must be YYYY-MM-DD"})
		return
	}
	updates := map[string]interface{}{"published_on": published, "label": strings.TrimSpace(req.Label)}
	if req.AssetID != 0 {
		var asset models.Asset
		if err := h.db.First(&asset, req.AssetID).Error; err != nil || asset.Kind != models.AssetPDF {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The new file must be an uploaded PDF"})
			return
		}
		updates["asset_id"] = req.AssetID
	}
	result := h.db.Model(&models.Bulletin{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bulletin not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) BulletinAPIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.Bulletin{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ---------- Events ----------

func (h *AdminStructuredHandler) EventsPage(c *gin.Context) {
	var events []models.Event
	h.db.Preload("FlyerAsset").Order("starts_at DESC").Find(&events)
	data := h.pageData(c, "admin-events", "Events — Admin — OLV Parish")
	data["events"] = events
	c.HTML(http.StatusOK, "admin-events.html", data)
}

type eventRequest struct {
	Slug     string `json:"slug"`
	Title    string `json:"title" binding:"required"`
	Location string `json:"location"`
	StartsAt string `json:"starts_at" binding:"required"` // RFC3339 or YYYY-MM-DDTHH:MM
	EndsAt   string `json:"ends_at"`
	Body     string `json:"body"`
	FlyerID  uint   `json:"flyer_asset_id"`
	Featured bool   `json:"featured"`
	Tag      string `json:"tag"`
}

func (h *AdminStructuredHandler) EventAPICreate(c *gin.Context) {
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	starts, err := parseFlexibleTime(req.StartsAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid starts_at: " + err.Error()})
		return
	}
	ends, _ := parseFlexibleTime(req.EndsAt)
	row := models.Event{
		Slug: h.eventSlug(req.Slug, req.Title, starts, 0), Title: req.Title, Location: req.Location,
		StartsAt: starts, Body: req.Body, Featured: req.Featured, Tag: req.Tag,
	}
	if !ends.IsZero() {
		row.EndsAt = &ends
	}
	if req.FlyerID != 0 {
		row.FlyerAssetID = &req.FlyerID
	}
	if err := h.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"row": row})
}

func (h *AdminStructuredHandler) EventAPIUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	starts, err := parseFlexibleTime(req.StartsAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid starts_at: " + err.Error()})
		return
	}
	ends, _ := parseFlexibleTime(req.EndsAt)
	updates := map[string]interface{}{
		"slug":           h.eventSlug(req.Slug, req.Title, starts, uint(id)),
		"title":          req.Title,
		"location":       req.Location,
		"starts_at":      starts,
		"body":           req.Body,
		"featured":       req.Featured,
		"tag":            req.Tag,
		"flyer_asset_id": nil,
		"ends_at":        nil,
	}
	if !ends.IsZero() {
		updates["ends_at"] = ends
	}
	if req.FlyerID != 0 {
		updates["flyer_asset_id"] = req.FlyerID
	}
	if err := h.db.Model(&models.Event{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) EventAPIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.Event{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// ---------- Helpers ----------

type kv struct {
	Value, Label string
}

func churchOptions() []kv {
	return []kv{
		{models.ChurchStAnn, "St. Ann's, Hornell"},
		{models.ChurchStMary, "St. Mary's, Rexville"},
		{models.ChurchEither, "Either church"},
		{models.ChurchParish, "Parish-wide"},
	}
}

func seasonOptions() []kv {
	return []kv{
		{models.SeasonAll, "Year-round"},
		{models.SeasonAdvent, "Advent only"},
		{models.SeasonLent, "Lent only"},
		{models.SeasonOrdinary, "Outside Advent & Lent"},
	}
}

// parseFlexibleTime reads an event time. Times without an explicit offset
// (what <input type="datetime-local"> sends) are parish wall-clock time, so
// they're interpreted in America/New_York — otherwise a 7 pm event would be
// stored as 7 pm UTC and drop off "upcoming" lists hours early.
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, parishZone); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse time %q (expected YYYY-MM-DDTHH:MM)", s)
}

// eventSlug cleans a staff-entered address, or builds one from the title and
// start date ("fall-festival-2026-10-12") when it's blank, then makes it
// unique against other events (including deleted ones, which still hold
// their address in the unique index).
func (h *AdminStructuredHandler) eventSlug(raw, title string, starts time.Time, excludeID uint) string {
	base := slugify(raw)
	if base == "" {
		base = slugify(title)
		if !starts.IsZero() {
			base = strings.Trim(base+"-"+starts.In(parishZone).Format("2006-01-02"), "-")
		}
	}
	if base == "" {
		base = "event"
	}
	slug := base
	for i := 2; i < 100; i++ {
		var n int64
		h.db.Unscoped().Model(&models.Event{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&n)
		if n == 0 {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return slug
}
