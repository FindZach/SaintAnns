package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// RenderBlocks walks a Page's or NewsPost's ordered block list and returns the
// composed HTML. Each block type is handled by a dedicated small function so a
// new block type is one case + one function to add.
//
// The DB handle is used by the dynamic blocks (mass_times_card,
// confession_times_card, event_list, bulletin_list, staff_grid). Static
// blocks (heading, rich_text, image, …) ignore it.
func RenderBlocks(blocks models.Blocks, db *gorm.DB) template.HTML {
	var out strings.Builder
	for _, block := range blocks {
		switch block.Type {
		case models.BlockHeading:
			renderHeading(&out, block.Data)
		case models.BlockRichText:
			renderRichText(&out, block.Data)
		case models.BlockImage:
			renderImage(&out, block.Data, db)
		case models.BlockImageRow:
			renderImageRow(&out, block.Data, db)
		case models.BlockPDFLink:
			renderPDFLink(&out, block.Data, db)
		case models.BlockVideoEmbed:
			renderVideoEmbed(&out, block.Data)
		case models.BlockMassTimesCard:
			renderMassTimesCard(&out, db)
		case models.BlockConfessions:
			renderConfessionsCard(&out, db)
		case models.BlockEventList:
			renderEventList(&out, block.Data, db)
		case models.BlockBulletinList:
			renderBulletinList(&out, block.Data, db)
		case models.BlockStaffGrid:
			renderStaffGrid(&out, block.Data, db)
		case models.BlockCTAButton:
			renderCTAButton(&out, block.Data)
		case models.BlockDivider:
			out.WriteString(`<hr class="parish-block-divider">`)
		case models.BlockHTMLRaw:
			renderHTMLRaw(&out, block.Data)
		case models.BlockChildPages:
			renderChildPages(&out, block.Data, db)
		default:
			// Unknown block types render as an admin-visible comment, not a 500.
			fmt.Fprintf(&out, "<!-- unknown block type: %s -->", html.EscapeString(block.Type))
		}
	}
	return template.HTML(out.String())
}

// ---------- Per-type renderers ----------

type headingData struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

func renderHeading(out *strings.Builder, raw json.RawMessage) {
	var d headingData
	if err := json.Unmarshal(raw, &d); err != nil || d.Text == "" {
		return
	}
	level := d.Level
	if level < 2 || level > 4 {
		level = 2
	}
	fmt.Fprintf(out, `<h%d class="parish-block-heading parish-block-heading-%d">%s</h%d>`,
		level, level, html.EscapeString(d.Text), level)
}

type richTextData struct {
	HTML string `json:"html"`
}

func renderRichText(out *strings.Builder, raw json.RawMessage) {
	var d richTextData
	if err := json.Unmarshal(raw, &d); err != nil || d.HTML == "" {
		return
	}
	out.WriteString(`<div class="parish-block-rich">`)
	out.WriteString(sanitizeHTML(d.HTML))
	out.WriteString(`</div>`)
}

type imageData struct {
	AssetID uint   `json:"asset_id"`
	Alt     string `json:"alt"`
	Caption string `json:"caption"`
	Size    string `json:"size"` // "small" | "medium" | "wide" | "" (default: medium)
}

func renderImage(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	var d imageData
	if err := json.Unmarshal(raw, &d); err != nil {
		return
	}
	asset := loadAsset(db, d.AssetID)
	if asset == nil {
		return
	}
	size := d.Size
	if size == "" {
		size = "medium"
	}
	alt := d.Alt
	if alt == "" {
		alt = asset.AltText
	}
	fmt.Fprintf(out, `<figure class="parish-block-image parish-block-image-%s">`, html.EscapeString(size))
	fmt.Fprintf(out, `<img src="%s" alt="%s" loading="lazy">`,
		html.EscapeString(asset.URL()), html.EscapeString(alt))
	if d.Caption != "" {
		fmt.Fprintf(out, `<figcaption>%s</figcaption>`, html.EscapeString(d.Caption))
	}
	out.WriteString(`</figure>`)
}

type imageRowData struct {
	AssetIDs []uint `json:"asset_ids"`
	Caption  string `json:"caption"`
}

func renderImageRow(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	var d imageRowData
	if err := json.Unmarshal(raw, &d); err != nil || len(d.AssetIDs) == 0 {
		return
	}
	out.WriteString(`<div class="parish-block-image-row">`)
	for _, id := range d.AssetIDs {
		asset := loadAsset(db, id)
		if asset == nil {
			continue
		}
		fmt.Fprintf(out, `<img src="%s" alt="%s" loading="lazy">`,
			html.EscapeString(asset.URL()), html.EscapeString(asset.AltText))
	}
	if d.Caption != "" {
		fmt.Fprintf(out, `<p class="parish-block-image-row-caption">%s</p>`,
			html.EscapeString(d.Caption))
	}
	out.WriteString(`</div>`)
}

type pdfLinkData struct {
	AssetID uint   `json:"asset_id"`
	Label   string `json:"label"`
}

func renderPDFLink(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	var d pdfLinkData
	if err := json.Unmarshal(raw, &d); err != nil {
		return
	}
	asset := loadAsset(db, d.AssetID)
	if asset == nil {
		return
	}
	label := d.Label
	if label == "" {
		label = asset.OriginalName
	}
	fmt.Fprintf(out, `<p class="parish-block-pdf"><a href="%s" target="_blank" rel="noopener"><i class="fas fa-file-pdf"></i> %s</a></p>`,
		html.EscapeString(asset.URL()), html.EscapeString(label))
}

type videoEmbedData struct {
	Provider string `json:"provider"` // "youtube" | "facebook" | "vimeo" | ""
	URL      string `json:"url"`
}

func renderVideoEmbed(out *strings.Builder, raw json.RawMessage) {
	var d videoEmbedData
	if err := json.Unmarshal(raw, &d); err != nil || d.URL == "" {
		return
	}
	src := videoEmbedURL(d.Provider, d.URL)
	if src == "" {
		// Fall back to a link so the content isn't lost.
		fmt.Fprintf(out, `<p class="parish-block-video-link"><a href="%s" target="_blank" rel="noopener">%s</a></p>`,
			html.EscapeString(d.URL), html.EscapeString(d.URL))
		return
	}
	fmt.Fprintf(out, `<div class="parish-block-video"><iframe src="%s" title="Embedded video" allow="autoplay; encrypted-media; picture-in-picture" allowfullscreen loading="lazy"></iframe></div>`,
		html.EscapeString(src))
}

type ctaButtonData struct {
	Href  string `json:"href"`
	Label string `json:"label"`
	Style string `json:"style"` // "primary" | "ghost"
}

func renderCTAButton(out *strings.Builder, raw json.RawMessage) {
	var d ctaButtonData
	if err := json.Unmarshal(raw, &d); err != nil || d.Href == "" || d.Label == "" {
		return
	}
	class := "parish-btn parish-btn-primary"
	if d.Style == "ghost" {
		class = "parish-btn parish-btn-ghost"
	}
	external := strings.HasPrefix(d.Href, "http") || strings.HasPrefix(d.Href, "https")
	rel := ""
	target := ""
	if external {
		target = ` target="_blank"`
		rel = ` rel="noopener"`
	}
	fmt.Fprintf(out, `<p class="parish-block-cta"><a href="%s"%s%s class="%s">%s</a></p>`,
		html.EscapeString(d.Href), target, rel, class, html.EscapeString(d.Label))
}

type htmlRawData struct {
	HTML string `json:"html"`
}

// renderHTMLRaw emits user-supplied HTML verbatim. The block is admin-only in
// the editor; it still passes through sanitizeHTML to strip scripts.
func renderHTMLRaw(out *strings.Builder, raw json.RawMessage) {
	var d htmlRawData
	if err := json.Unmarshal(raw, &d); err != nil || d.HTML == "" {
		return
	}
	out.WriteString(sanitizeHTML(d.HTML))
}

// ---------- Dynamic blocks (DB-driven) ----------

func renderMassTimesCard(out *strings.Builder, db *gorm.DB) {
	if db == nil {
		return
	}
	rows := LoadMassSchedule(db)
	if len(rows) == 0 {
		return
	}
	out.WriteString(`<div class="parish-block-mass-card parish-card">`)
	out.WriteString(`<h3 class="parish-card-title">Mass Times</h3>`)
	out.WriteString(`<ul class="parish-mass-list">`)
	for _, r := range rows {
		fmt.Fprintf(out, `<li><span>%s %s</span><strong>%s</strong></li>`,
			html.EscapeString(ChurchLabel(r.Church)),
			html.EscapeString(weekdayLabel(r.DayOfWeek)),
			html.EscapeString(FormatTimeOfDay(r.TimeOfDay)))
	}
	out.WriteString(`</ul>`)
	out.WriteString(`</div>`)
}

func renderConfessionsCard(out *strings.Builder, db *gorm.DB) {
	if db == nil {
		return
	}
	rows := LoadConfessionTimes(db)
	if len(rows) == 0 {
		return
	}
	out.WriteString(`<div class="parish-block-mass-card parish-card parish-card-accent">`)
	out.WriteString(`<h3 class="parish-card-title">Reconciliation</h3>`)
	out.WriteString(`<ul class="parish-mass-list">`)
	for _, r := range rows {
		label := weekdayLabel(r.DayOfWeek)
		if r.Label != "" {
			label += " · " + r.Label
		}
		fmt.Fprintf(out, `<li><span>%s</span><strong>%s – %s</strong></li>`,
			html.EscapeString(label),
			html.EscapeString(FormatTimeOfDay(r.StartOfDay)),
			html.EscapeString(FormatTimeOfDay(r.EndOfDay)))
	}
	out.WriteString(`</ul>`)
	out.WriteString(`</div>`)
}

type eventListData struct {
	Tag   string `json:"tag"`
	Limit int    `json:"limit"`
}

func renderEventList(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	if db == nil {
		return
	}
	var d eventListData
	_ = json.Unmarshal(raw, &d)
	limit := d.Limit
	if limit <= 0 || limit > 50 {
		limit = 6
	}
	var events []models.Event
	q := db.Where("starts_at >= NOW()").Order("starts_at ASC").Limit(limit)
	if d.Tag != "" {
		q = q.Where("tag = ?", d.Tag)
	}
	if err := q.Find(&events).Error; err != nil || len(events) == 0 {
		return
	}
	out.WriteString(`<ul class="parish-block-event-list">`)
	for _, e := range events {
		fmt.Fprintf(out, `<li><a href="/events-news/%s"><time>%s</time><strong>%s</strong></a></li>`,
			html.EscapeString(e.Slug),
			html.EscapeString(e.StartsAt.Format("Mon Jan 2 · 3:04 PM")),
			html.EscapeString(e.Title))
	}
	out.WriteString(`</ul>`)
}

type bulletinListData struct {
	Limit int `json:"limit"`
}

func renderBulletinList(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	if db == nil {
		return
	}
	var d bulletinListData
	_ = json.Unmarshal(raw, &d)
	limit := d.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var bulletins []models.Bulletin
	if err := db.Preload("Asset").Order("published_on DESC").Limit(limit).Find(&bulletins).Error; err != nil || len(bulletins) == 0 {
		return
	}
	out.WriteString(`<ul class="parish-block-bulletin-list">`)
	for _, b := range bulletins {
		href := ""
		if b.Asset != nil {
			href = b.Asset.URL()
		}
		fmt.Fprintf(out, `<li><a href="%s" target="_blank" rel="noopener"><i class="fas fa-file-pdf"></i> <time>%s</time> &middot; %s</a></li>`,
			html.EscapeString(href),
			html.EscapeString(b.PublishedOn.Format("January 2, 2006")),
			html.EscapeString(b.Label))
	}
	out.WriteString(`</ul>`)
}

type childPagesData struct {
	ParentSlug string `json:"parent_slug"`
	Intro      string `json:"intro"`
}

// renderChildPages lists every published Page whose ParentSlug matches the
// block's parent_slug. Used by the importer for hub pages (/minstries/,
// /faith-formation/, etc.) that have no body of their own on the live site but
// link to several sub-pages.
func renderChildPages(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	if db == nil {
		return
	}
	var d childPagesData
	_ = json.Unmarshal(raw, &d)
	if d.ParentSlug == "" {
		return
	}
	var children []models.Page
	if err := db.Where("parent_slug = ? AND published_at IS NOT NULL AND published_at <= NOW()", d.ParentSlug).
		Order("title ASC").Find(&children).Error; err != nil || len(children) == 0 {
		return
	}
	if d.Intro != "" {
		fmt.Fprintf(out, `<p class="parish-block-child-pages-intro">%s</p>`, html.EscapeString(d.Intro))
	}
	out.WriteString(`<ul class="parish-block-child-pages">`)
	for _, c := range children {
		label := c.NavLabel
		if label == "" {
			label = c.Title
		}
		fmt.Fprintf(out, `<li><a href="/%s/">%s</a></li>`,
			html.EscapeString(c.Slug), html.EscapeString(label))
	}
	out.WriteString(`</ul>`)
}

type staffGridData struct {
	MinistrySlug string `json:"ministry_slug"`
}

func renderStaffGrid(out *strings.Builder, raw json.RawMessage, db *gorm.DB) {
	if db == nil {
		return
	}
	var d staffGridData
	_ = json.Unmarshal(raw, &d)
	// MinistrySlug is reserved for a future filter when Staff has a ministry
	// association; today it loads every staff member.
	_ = d
	var staff []models.Staff
	if err := db.Preload("PhotoAsset").Order("sort ASC, name ASC").Find(&staff).Error; err != nil || len(staff) == 0 {
		return
	}
	out.WriteString(`<div class="parish-block-staff-grid">`)
	for _, s := range staff {
		out.WriteString(`<article class="parish-staff-card">`)
		if s.PhotoAsset != nil {
			fmt.Fprintf(out, `<img src="%s" alt="%s" class="parish-staff-photo" loading="lazy">`,
				html.EscapeString(s.PhotoAsset.URL()), html.EscapeString(s.Name))
		}
		fmt.Fprintf(out, `<h3>%s</h3>`, html.EscapeString(s.Name))
		fmt.Fprintf(out, `<p class="parish-staff-role">%s</p>`, html.EscapeString(s.Role))
		if s.Email != "" {
			fmt.Fprintf(out, `<p><a href="mailto:%s">%s</a></p>`,
				html.EscapeString(s.Email), html.EscapeString(s.Email))
		}
		out.WriteString(`</article>`)
	}
	out.WriteString(`</div>`)
}

// ---------- Helpers ----------

func loadAsset(db *gorm.DB, id uint) *models.Asset {
	if db == nil || id == 0 {
		return nil
	}
	var asset models.Asset
	if err := db.First(&asset, id).Error; err != nil {
		return nil
	}
	return &asset
}

// sanitizeHTML strips script / style / event-handler attributes. The parish
// doesn't need rich HTML — the editor's rich_text block will produce a limited
// set of tags — but just in case something slips through, we clean up here.
func sanitizeHTML(in string) string {
	// Minimal passthrough strip. For a real-world launch, swap in bluemonday.
	s := in
	for _, bad := range []string{"<script", "</script", "<style", "</style",
		"javascript:", "onerror=", "onclick=", "onload=", "onfocus="} {
		s = strings.ReplaceAll(s, bad, "")
		s = strings.ReplaceAll(s, strings.ToUpper(bad), "")
	}
	return s
}

func videoEmbedURL(provider, raw string) string {
	switch strings.ToLower(provider) {
	case "youtube", "yt":
		// Accept either a full URL or just an id.
		if id := extractYouTubeID(raw); id != "" {
			return "https://www.youtube.com/embed/" + id
		}
	case "vimeo":
		if id := extractVimeoID(raw); id != "" {
			return "https://player.vimeo.com/video/" + id
		}
	case "facebook", "fb":
		// Facebook embed takes the video URL as a parameter.
		return "https://www.facebook.com/plugins/video.php?href=" + urlEncode(raw)
	}
	// Unknown provider: refuse to embed, caller falls back to a link.
	return ""
}

func extractYouTubeID(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, prefix := range []string{"https://youtu.be/", "https://www.youtube.com/watch?v=", "https://www.youtube.com/embed/"} {
		if strings.HasPrefix(raw, prefix) {
			rest := strings.TrimPrefix(raw, prefix)
			if i := strings.IndexAny(rest, "?&"); i >= 0 {
				rest = rest[:i]
			}
			return rest
		}
	}
	// Treat anything short and ID-shaped as a raw ID.
	if len(raw) <= 15 && !strings.ContainsAny(raw, "/?&= ") {
		return raw
	}
	return ""
}

func extractVimeoID(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "https://vimeo.com/")
	raw = strings.TrimPrefix(raw, "https://player.vimeo.com/video/")
	if i := strings.IndexAny(raw, "?&"); i >= 0 {
		raw = raw[:i]
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return raw
	}
	return ""
}

func urlEncode(s string) string {
	// Minimal escaping — &, ?, = are the ones that matter for a plugin.
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, "?", "%3F")
	s = strings.ReplaceAll(s, "=", "%3D")
	return s
}
