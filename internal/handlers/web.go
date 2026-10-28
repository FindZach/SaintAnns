package handlers

import (
	"net/http"
	"strconv"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Ensure imports stay referenced in case of future conditional use.
var _ = models.AssetImage

// assetVersion is passed to templates as "version" and appended to the CSS and
// JS URLs. It changes on every deploy, so browsers fetch fresh assets after a
// release instead of reusing stale cached copies.
var assetVersion = strconv.FormatInt(time.Now().Unix(), 10)

// PublicHandler serves the stateless-ish public pages (home, contact, admin
// login) that still need the DB handle so the home page can pull Mass times
// and Courier headlines live.
type PublicHandler struct {
	db *gorm.DB
}

func NewPublicHandler(db *gorm.DB) *PublicHandler {
	return &PublicHandler{db: db}
}

func HealthCheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// Home renders the parish home page. Mass times + Courier headlines come live
// from the DB; everything else is static copy.
func (h *PublicHandler) Home(c *gin.Context) {
	massGroups := groupMassByChurch(LoadMassSchedule(h.db))
	confessions := LoadConfessionTimes(h.db)
	headlines := LoadCourierHeadlines(h.db, 5)
	stAnn, stMary := LoadChurchPhotos(h.db)
	upcoming := LoadUpcomingEvents(h.db, 5)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":           "Our Lady of the Valley Parish",
		"page":            "home",
		"massGroups":      massGroups,
		"confessionTimes": confessions,
		"headlines":       headlines,
		"stAnnPhoto":      stAnn,
		"stMaryPhoto":     stMary,
		"upcomingEvents":  upcoming,
		"version":         assetVersion,
	})
}

// LoadUpcomingEvents returns upcoming parish events for the home page,
// preferring events marked Featured but falling back to the next few events
// chronologically if there aren't enough featured ones. Only events whose
// start time is still in the future are included.
func LoadUpcomingEvents(db *gorm.DB, limit int) []models.Event {
	if db == nil || limit <= 0 {
		return nil
	}
	var events []models.Event
	// Featured first, then by starts_at ascending — a featured event always
	// outranks a non-featured one even if sooner, so staff can pin a Mass or
	// retreat at the top of the home page.
	if err := db.Preload("FlyerAsset").
		Where("starts_at >= NOW()").
		Order("featured DESC, starts_at ASC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil
	}
	return events
}

// LoadChurchPhotos returns the exterior photo an admin has explicitly tagged
// for each church. We only match assets whose original_name or alt_text
// contains an explicit "exterior" marker so we don't accidentally pick a
// chapel-sign or signage photo that happened to be filed under a church name
// (the WordPress import left both churches with ambiguous filenames). Until
// an admin uploads a tagged photo, the template renders a stained-glass SVG
// placeholder instead.
func LoadChurchPhotos(db *gorm.DB) (stAnn, stMary *models.Asset) {
	if db == nil {
		return nil, nil
	}
	stAnn = loadTaggedExterior(db, []string{"%stann%exterior%", "%st-ann%exterior%", "%hornell%exterior%"})
	stMary = loadTaggedExterior(db, []string{"%stmary%exterior%", "%st-mary%exterior%", "%rexville%exterior%"})
	return stAnn, stMary
}

func loadTaggedExterior(db *gorm.DB, patterns []string) *models.Asset {
	for _, pat := range patterns {
		var a models.Asset
		q := db.Where("kind = ? AND (original_name ILIKE ? OR alt_text ILIKE ?)",
			models.AssetImage, pat, pat).
			Order("byte_size DESC").
			Limit(1)
		if err := q.First(&a).Error; err == nil {
			return &a
		}
	}
	return nil
}

func (h *PublicHandler) Contact(c *gin.Context) {
	c.HTML(http.StatusOK, "contact.html", gin.H{
		"title":    "Contact Us - Our Lady of the Valley Parish",
		"page":     "contact",
		"subjects": models.ContactSubjects,
		"version":  assetVersion,
	})
}

func (h *PublicHandler) AdminLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "admin-login.html", gin.H{
		"title":   "Sign In - OLV Parish Admin",
		"page":    "admin-login",
		"version": assetVersion,
	})
}
