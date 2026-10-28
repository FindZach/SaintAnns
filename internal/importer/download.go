package importer

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"olv-parish/internal/models"
)

const (
	assetUploadRoot = "web/static/uploads/legacy"
	assetPathPrefix = "uploads/legacy"
	assetTimeout    = 20 * time.Second
	maxAssetBytes   = 25 * 1024 * 1024
)

// downloadAsset fetches a remote URL and records it as an Asset. Returns the
// Asset.ID so blocks can reference it. Idempotent within one import run via
// the in-memory assetCache; across runs via OriginalName lookup.
func (imp *importer) downloadAsset(srcURL, kindHint string) (uint, error) {
	if id, ok := imp.assetCache[srcURL]; ok {
		return id, nil
	}

	// Reuse any existing Asset with the same source URL (recorded in
	// OriginalName) so re-imports don't duplicate.
	if !imp.dryRun && imp.db != nil {
		var existing models.Asset
		if err := imp.db.Where("original_name = ?", srcURL).First(&existing).Error; err == nil {
			imp.assetCache[srcURL] = existing.ID
			return existing.ID, nil
		}
	}

	if imp.dryRun {
		if imp.verbose {
			fmt.Printf("[dry-run] would download %s\n", srcURL)
		}
		imp.assetCache[srcURL] = 0
		imp.assetCount++
		return 0, nil
	}

	// Fetch.
	client := &http.Client{Timeout: assetTimeout}
	req, err := http.NewRequest(http.MethodGet, srcURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "OLV-Parish-Importer/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Choose local path.
	parsed, err := url.Parse(srcURL)
	if err != nil {
		return 0, err
	}
	base := safeBasename(path.Base(parsed.Path))
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		ext = extForMIME(resp.Header.Get("Content-Type"))
		base = base + ext
	}
	now := time.Now()
	dir := filepath.Join(assetUploadRoot, now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	// Prefix with timestamp so same-named files from different pages don't collide.
	filename := fmt.Sprintf("%d-%s", now.UnixNano(), base)
	dest := filepath.Join(dir, filename)
	out, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	size, err := io.Copy(out, io.LimitReader(resp.Body, maxAssetBytes))
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(dest)
		return 0, fmt.Errorf("write: %v", err)
	}

	rel := strings.ReplaceAll(
		filepath.Join(assetPathPrefix, now.Format("2006"), now.Format("01"), filename),
		"\\", "/",
	)

	kind := kindHint
	if kind == "" {
		kind = classifyKind(resp.Header.Get("Content-Type"), ext)
	}

	asset := models.Asset{
		Kind:         kind,
		Path:         rel,
		OriginalName: srcURL, // we use this to dedup across import runs
		MIMEType:     resp.Header.Get("Content-Type"),
		ByteSize:     size,
	}
	if kind == models.AssetImage {
		if f, err := os.Open(dest); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				asset.Width = cfg.Width
				asset.Height = cfg.Height
			}
			f.Close()
		}
	}
	if err := imp.db.Create(&asset).Error; err != nil {
		_ = os.Remove(dest)
		return 0, err
	}
	imp.assetCache[srcURL] = asset.ID
	imp.assetCount++
	if imp.verbose {
		fmt.Printf("[asset] %s → #%d (%s, %d bytes)\n", srcURL, asset.ID, kind, size)
	}
	return asset.ID, nil
}

func safeBasename(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "file"
	}
	// Drop query strings if any slipped through.
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[:i]
	}
	// Lowercase + strip anything but alnum, dot, dash, underscore.
	var out strings.Builder
	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('-')
		}
	}
	s := out.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "file"
	}
	if len(s) > 80 {
		// Keep the extension if any.
		ext := filepath.Ext(s)
		s = s[:80-len(ext)] + ext
	}
	return s
}

func extForMIME(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "audio/mpeg":
		return ".mp3"
	case "video/mp4":
		return ".mp4"
	}
	return ".bin"
}

func classifyKind(contentType, ext string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.HasPrefix(ct, "image/"):
		return models.AssetImage
	case ct == "application/pdf":
		return models.AssetPDF
	case strings.HasPrefix(ct, "audio/"):
		return models.AssetAudio
	case strings.HasPrefix(ct, "video/"):
		return models.AssetVideo
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return models.AssetImage
	case ".pdf":
		return models.AssetPDF
	case ".mp3", ".m4a":
		return models.AssetAudio
	case ".mp4":
		return models.AssetVideo
	}
	return models.AssetImage
}
