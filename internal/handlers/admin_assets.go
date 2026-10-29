package handlers

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	uploadsRoot    = "web/static/uploads"
	uploadsURLBase = "uploads"
	maxUploadBytes = 20 * 1024 * 1024 // 20 MB per file
)

var allowedMIME = map[string]string{
	"image/jpeg":      AssetImageExt(".jpg"),
	"image/png":       AssetImageExt(".png"),
	"image/webp":      AssetImageExt(".webp"),
	"image/gif":       AssetImageExt(".gif"),
	"application/pdf": "pdf",
	"audio/mpeg":      "mp3",
	"audio/mp4":       "m4a",
	"video/mp4":       "mp4",
}

// AssetImageExt is a tiny helper so the keys in the map above read with their
// type instead of raw strings.
func AssetImageExt(ext string) string {
	return strings.TrimPrefix(ext, ".")
}

// AdminAssetsHandler owns the upload / list / delete endpoints the editor uses
// when the user picks an image, PDF, etc.
type AdminAssetsHandler struct {
	db *gorm.DB
}

func NewAdminAssetsHandler(db *gorm.DB) *AdminAssetsHandler {
	return &AdminAssetsHandler{db: db}
}

// LibraryPage renders /admin/assets, the asset library UI. The grid itself
// is populated via APIList over fetch so staff can upload, filter, and
// delete without a page reload.
func (h *AdminAssetsHandler) LibraryPage(c *gin.Context) {
	userValue, _ := c.Get("user")
	user, _ := userValue.(models.User)

	data := gin.H{
		"title":          "Asset Library — Admin — OLV Parish",
		"page":           "admin-assets",
		"version":        assetVersion,
		"user":           user,
		"maxUploadBytes": maxUploadBytes,
		"maxUploadLabel": fmt.Sprintf("%d MB", maxUploadBytes/(1024*1024)),
	}
	c.HTML(http.StatusOK, "admin-assets.html", data)
}

// APIList returns the asset library (optionally filtered by kind=image|pdf).
func (h *AdminAssetsHandler) APIList(c *gin.Context) {
	q := h.db.Order("created_at DESC").Limit(500)
	if kind := c.Query("kind"); kind != "" {
		q = q.Where("kind = ?", kind)
	}
	var assets []models.Asset
	if err := q.Find(&assets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load assets"})
		return
	}
	out := make([]gin.H, 0, len(assets))
	for _, a := range assets {
		out = append(out, gin.H{
			"id":            a.ID,
			"kind":          a.Kind,
			"url":           a.URL(),
			"original_name": a.OriginalName,
			"alt_text":      a.AltText,
			"width":         a.Width,
			"height":        a.Height,
		})
	}
	c.JSON(http.StatusOK, gin.H{"assets": out})
}

// APIUpload accepts a multipart upload, writes it to disk, and records an
// Asset row.
func (h *AdminAssetsHandler) APIUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	if file.Size > maxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "File is too large (20 MB max)"})
		return
	}
	kind, ext, ok := classifyUpload(file.Filename, file.Header.Get("Content-Type"))
	if !ok {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "Only images, PDFs, mp3/mp4/m4a are allowed"})
		return
	}

	// Save under uploads/YYYY/MM/<timestamp>-<safe-name>.<ext>
	now := time.Now()
	dir := filepath.Join(uploadsRoot, now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}
	base := safeFilename(file.Filename)
	filename := fmt.Sprintf("%d-%s%s", now.UnixNano(), base, extForMIME(ext, file.Filename))
	dest := filepath.Join(dir, filename)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read upload"})
		return
	}
	defer src.Close()
	out, err := os.Create(dest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write upload"})
		return
	}
	size, err := io.Copy(out, src)
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(dest)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save upload"})
		return
	}

	// Build the URL-ish path (always forward slashes, relative to web/static).
	rel := strings.ReplaceAll(filepath.Join(uploadsURLBase, now.Format("2006"), now.Format("01"), filename), "\\", "/")

	asset := models.Asset{
		Kind:         kind,
		Path:         rel,
		OriginalName: file.Filename,
		MIMEType:     file.Header.Get("Content-Type"),
		ByteSize:     size,
	}
	if kind == models.AssetImage {
		if f, err := os.Open(dest); err == nil {
			if img, _, err := image.DecodeConfig(f); err == nil {
				asset.Width = img.Width
				asset.Height = img.Height
			}
			f.Close()
		}
	}
	if uploadedBy := c.GetUint("user_id"); uploadedBy != 0 {
		asset.UploadedBy = &uploadedBy
	}
	if err := h.db.Create(&asset).Error; err != nil {
		_ = os.Remove(dest)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record asset"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            asset.ID,
		"kind":          asset.Kind,
		"url":           asset.URL(),
		"original_name": asset.OriginalName,
		"width":         asset.Width,
		"height":        asset.Height,
	})
}

// APIDelete removes an asset row AND the on-disk file. Blocks that reference
// a now-missing asset simply render as empty — Phase 5 can add a usage check.
func (h *AdminAssetsHandler) APIDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset id"})
		return
	}
	var asset models.Asset
	if err := h.db.First(&asset, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}
	if err := h.db.Delete(&asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete asset"})
		return
	}
	// Delete the file best-effort.
	_ = os.Remove(filepath.Join("web/static", asset.Path))
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

// classifyUpload maps an uploaded file to its Asset.Kind.
func classifyUpload(filename, contentType string) (kind, ext string, ok bool) {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	// Trim any ;charset=... suffix.
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if v, found := allowedMIME[ct]; found {
		switch ct {
		case "application/pdf":
			return models.AssetPDF, v, true
		case "audio/mpeg", "audio/mp4":
			return models.AssetAudio, v, true
		case "video/mp4":
			return models.AssetVideo, v, true
		default:
			return models.AssetImage, v, true
		}
	}
	// Fallback: try extension sniffing for browsers that don't send MIME.
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return models.AssetImage, "jpg", true
	case ".png":
		return models.AssetImage, "png", true
	case ".webp":
		return models.AssetImage, "webp", true
	case ".gif":
		return models.AssetImage, "gif", true
	case ".pdf":
		return models.AssetPDF, "pdf", true
	case ".mp3":
		return models.AssetAudio, "mp3", true
	case ".m4a":
		return models.AssetAudio, "m4a", true
	case ".mp4":
		return models.AssetVideo, "mp4", true
	}
	return "", "", false
}

func extForMIME(detected, filename string) string {
	// Use the detected extension if it's one of ours, otherwise fall back to
	// whatever the filename already had.
	if detected != "" && !strings.ContainsAny(detected, "/\\.") {
		return "." + detected
	}
	if ext := filepath.Ext(filename); ext != "" {
		return ext
	}
	return ""
}

func safeFilename(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimSuffix(raw, filepath.Ext(raw))
	var out strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_':
			out.WriteRune(r)
		case r == ' ' || r == '.':
			out.WriteRune('-')
		}
	}
	s := out.String()
	if s == "" {
		s = "file"
	}
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}
