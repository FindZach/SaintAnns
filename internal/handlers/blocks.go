package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"regexp"
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
// link to several sub-pages. Each child renders as a card with a derived icon
// and a short description pulled from its own content.
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
	out.WriteString(`<div class="parish-block-child-cards">`)
	for _, c := range children {
		label := c.NavLabel
		if label == "" {
			label = c.Title
		}
		icon := iconForSlug(c.Slug)
		desc := childPageDescription(c)
		fmt.Fprintf(out,
			`<a class="parish-child-card" href="/%s/">`+
				`<span class="parish-child-card-icon" aria-hidden="true"><i class="fas %s"></i></span>`+
				`<span class="parish-child-card-body">`+
				`<h3 class="parish-child-card-title">%s</h3>`+
				`%s`+
				`<span class="parish-child-card-cta">Learn more <i class="fas fa-arrow-right" aria-hidden="true"></i></span>`+
				`</span>`+
				`</a>`,
			html.EscapeString(c.Slug),
			icon,
			html.EscapeString(label),
			descMarkup(desc),
		)
	}
	out.WriteString(`</div>`)
}

func descMarkup(desc string) string {
	if desc == "" {
		return ""
	}
	return `<p class="parish-child-card-desc">` + html.EscapeString(desc) + `</p>`
}

// childPageDescription returns a short teaser for a child page. Prefers
// MetaDescription when set; otherwise strips tags from the first rich_text
// block and truncates to ~140 chars at a word boundary.
func childPageDescription(p models.Page) string {
	if md := strings.TrimSpace(p.MetaDescription); md != "" {
		return truncateAtWord(md, 140)
	}
	for _, b := range p.Blocks {
		if b.Type != models.BlockRichText {
			continue
		}
		var rd struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(b.Data, &rd); err != nil {
			continue
		}
		text := collapseWS(stripHTMLTags(rd.HTML))
		if text != "" {
			return truncateAtWord(text, 140)
		}
	}
	return ""
}

var (
	tagStripRE = regexp.MustCompile(`<[^>]+>`)
	whitespcRE = regexp.MustCompile(`\s+`)
)

func stripHTMLTags(s string) string { return tagStripRE.ReplaceAllString(s, " ") }
func collapseWS(s string) string {
	return strings.TrimSpace(whitespcRE.ReplaceAllString(s, " "))
}

func truncateAtWord(s string, max int) string {
	if len(s) <= max {
		return s
	}
	trimmed := s[:max]
	if cut := strings.LastIndex(trimmed, " "); cut > max/2 {
		trimmed = trimmed[:cut]
	}
	return strings.TrimRight(trimmed, " .,;:—–-") + "…"
}

// iconForSlug picks a Font Awesome icon that fits the ministry's theme.
// Falls back to a generic church icon so unknown slugs still render.
func iconForSlug(slug string) string {
	s := strings.ToLower(slug)
	switch {
	case strings.Contains(s, "music"):
		return "fa-music"
	case strings.Contains(s, "lector"):
		return "fa-book-open"
	case strings.Contains(s, "altar-server"):
		return "fa-cross"
	case strings.Contains(s, "funeral"):
		return "fa-pray"
	case strings.Contains(s, "usher"):
		return "fa-door-open"
	case strings.Contains(s, "grief"):
		return "fa-heart"
	case strings.Contains(s, "preparing-to-honor"):
		return "fa-dove"
	case strings.Contains(s, "bereavement"):
		return "fa-dove"
	case strings.Contains(s, "children"), strings.Contains(s, "childrens"):
		return "fa-child"
	case strings.Contains(s, "mom"), strings.Contains(s, "support-group"):
		return "fa-people-group"
	case strings.Contains(s, "pastoral-council"):
		return "fa-users-gear"
	case strings.Contains(s, "finance"):
		return "fa-coins"
	case strings.Contains(s, "liturgy-committee"):
		return "fa-church"
	case strings.Contains(s, "cemetery"):
		return "fa-monument"
	case strings.Contains(s, "holy-communion"), strings.Contains(s, "emhc"), strings.Contains(s, "extra-ordinary"):
		return "fa-wine-glass"
	case strings.Contains(s, "young-adult"), strings.Contains(s, "faith-on-tap"):
		return "fa-mug-hot"
	case strings.Contains(s, "rite-of-christian-initiation"), strings.Contains(s, "rcia"):
		return "fa-user-plus"
	case strings.Contains(s, "quest"), strings.Contains(s, "youth"):
		return "fa-users"
	case strings.Contains(s, "faith-formation"), strings.Contains(s, "faith-foramtion"):
		return "fa-bible"
	case strings.Contains(s, "sacraments"), strings.Contains(s, "baptism"), strings.Contains(s, "confirmation"), strings.Contains(s, "reconciliation"), strings.Contains(s, "matrimony"), strings.Contains(s, "holy-orders"), strings.Contains(s, "anointing"), strings.Contains(s, "adoration"):
		return "fa-cross"
	}
	return "fa-church"
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

// Compiled once at package load. These strip entire script/style/noscript
// elements including their contents (case-insensitive, dot matches newline);
// the previous "substring remove" approach left the JS body behind as text.
var (
	scriptBlockRE   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script\s*>`)
	styleBlockRE    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style\s*>`)
	noscriptBlockRE = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript\s*>`)
	// WP "Email Encoder Bundle" leaves this pair: <span id="eeb-N-M"></span>
	// followed by a script that URL-decodes the real address into the span.
	// Pull both out of the string and substitute a plain mailto link.
	wpEEBPairRE = regexp.MustCompile(`(?is)<span[^>]*id="eeb-[^"]*"[^>]*>\s*</span>\s*(<script[^>]*>[^<]*?decodeURIComponent\("([^"]+)"\)[^<]*</script>)?`)
	// Fallback for already-sanitized leftovers: the raw body of the script
	// after our old sanitizer removed just the <script> markers.
	wpEEBScriptBodyRE = regexp.MustCompile(`(?is)\s*type="text/javascript">[^<]*decodeURIComponent\("([^"]+)"\)[^<]*>`)
	// Another WP encoder variant: <a href=";" data-enc-email="rot13[at]rot13" ...>label</a>
	wpEncLinkRE = regexp.MustCompile(`(?is)<a[^>]*data-enc-email="([^"]+)"[^>]*>(.*?)</a>`)
	// On-attribute event handlers; strip after block removals so we don't try
	// to parse attributes we already deleted.
	onEventRE = regexp.MustCompile(`(?i)\s(on(?:click|load|error|focus|mouseover|submit|change|blur|keyup|keydown|input))\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
)

// sanitizeHTML removes elements that have no place in parish body copy and
// cleans up leftovers from the WordPress import. In order:
//  1. Delete entire <script>, <style>, <noscript> elements (contents included).
//  2. Decode "Email Encoder Bundle" obfuscations into plain mailto links so
//     an address like david.prete@dor.org is kept, not stripped.
//  3. Scrub on-event attributes and javascript: URLs.
func sanitizeHTML(in string) string {
	s := in

	// (2a) Encoder "pair" form: <span id="eeb-…"></span><script>decodeURIComponent("…")</script>
	s = wpEEBPairRE.ReplaceAllStringFunc(s, func(match string) string {
		m := wpEEBPairRE.FindStringSubmatch(match)
		if len(m) >= 3 && m[2] != "" {
			if addr, err := url.QueryUnescape(m[2]); err == nil && strings.Contains(addr, "@") {
				return mailtoLink(addr)
			}
		}
		// No decodable address — drop the span but keep nothing visible.
		return ""
	})

	// (2b) data-enc-email ROT13 form.
	s = wpEncLinkRE.ReplaceAllStringFunc(s, func(match string) string {
		m := wpEncLinkRE.FindStringSubmatch(match)
		if len(m) >= 3 {
			if addr := decodeROT13Email(m[1]); strings.Contains(addr, "@") {
				return mailtoLink(addr)
			}
		}
		return ""
	})

	// (1) Now that encoder matches used the script tags, remove any surviving
	// script/style/noscript blocks whole.
	s = scriptBlockRE.ReplaceAllString(s, "")
	s = styleBlockRE.ReplaceAllString(s, "")
	s = noscriptBlockRE.ReplaceAllString(s, "")

	// (2c) Encoder "body-only" form left by the previous sanitizer: `
	// type="text/javascript">...decodeURIComponent("…")...>` with no opening
	// <script. We already decoded in (2a) via the <span> that preceded it,
	// but when the <span> is missing we still want to decode this body.
	s = wpEEBScriptBodyRE.ReplaceAllStringFunc(s, func(match string) string {
		m := wpEEBScriptBodyRE.FindStringSubmatch(match)
		if len(m) >= 2 {
			if addr, err := url.QueryUnescape(m[1]); err == nil && strings.Contains(addr, "@") {
				return mailtoLink(addr)
			}
		}
		return ""
	})

	// (3) Scrub on-event handlers and javascript: urls.
	s = onEventRE.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "javascript:", "about:blank#")
	s = strings.ReplaceAll(s, "JAVASCRIPT:", "about:blank#")

	return s
}

func mailtoLink(addr string) string {
	addr = strings.TrimSpace(addr)
	return fmt.Sprintf(`<a href="mailto:%s">%s</a>`,
		html.EscapeString(addr), html.EscapeString(addr))
}

// decodeROT13Email turns the WP "data-enc-email" format
// (ROT13-encoded local + literal "[at]" + ROT13-encoded domain) back into
// local@domain. The "[at]" marker is a plain separator — it is not itself
// ROT13-encoded in the source — so we split on it first and decode each
// side independently.
func decodeROT13Email(s string) string {
	parts := strings.SplitN(s, "[at]", 2)
	if len(parts) != 2 {
		return rot13(s)
	}
	return rot13(parts[0]) + "@" + rot13(parts[1])
}

func rot13(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			r = 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			r = 'A' + (r-'A'+13)%26
		}
		b.WriteRune(r)
	}
	return b.String()
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
