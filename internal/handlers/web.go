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
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":           "Our Lady of the Valley Parish",
		"page":            "home",
		"massGroups":      massGroups,
		"confessionTimes": confessions,
		"headlines":       headlines,
		"stAnnPhoto":      stAnn,
		"stMaryPhoto":     stMary,
		"version":         assetVersion,
	})
}

// LoadChurchPhotos returns the single best exterior photo for each church,
// picking the largest-byte-size image whose stored path matches a known
// church-name pattern. Returns nil for one or both if nothing matches, in
// which case the template falls back to a placeholder — admins can then
// upload a photo via /admin/assets.
func LoadChurchPhotos(db *gorm.DB) (stAnn, stMary *models.Asset) {
	if db == nil {
		return nil, nil
	}
	stAnn = loadBestAssetMatching(db, []string{"%stannhornell%", "%st-ann%", "%st_ann%"})
	stMary = loadBestAssetMatching(db, []string{"%stmaryrexville%", "%st-mary%", "%st_mary%"})
	return stAnn, stMary
}

func loadBestAssetMatching(db *gorm.DB, patterns []string) *models.Asset {
	for _, pat := range patterns {
		var a models.Asset
		q := db.Where("kind = ? AND original_name ILIKE ?", models.AssetImage, pat).
			Or("kind = ? AND path ILIKE ?", models.AssetImage, pat).
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
