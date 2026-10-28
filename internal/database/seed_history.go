package database

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// SeedHistoryPage upserts the /about/our-history/ parish-history page. Idempotent:
// re-running overwrites the Blocks with the latest canonical body. Visual
// anchors are embedded SVG + styled milestone tiles so the page is visually
// rich without depending on uploaded photos (which the parish can add later
// via /admin/pages/about/our-history/).
func SeedHistoryPage(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("seed history: no database connection")
	}
	log.Println("Seeding parish history page at /about/our-history/ ...")

	blocks := historyBlocks()

	now := time.Now()
	const slug = "about/our-history"
	var existing models.Page
	err := db.Where("slug = ?", slug).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		page := models.Page{
			Slug:            slug,
			ParentSlug:      "about",
			Title:           "Our History",
			NavLabel:        "Our History",
			MetaDescription: "The 175-year story of St. Ann's Church in Hornell — from a priest's visit to Thomas Doorley's home in 1843 to the 2026 Jubilee of Our Lady of the Valley Parish.",
			Blocks:          blocks,
			PublishedAt:     &now,
		}
		if err := db.Create(&page).Error; err != nil {
			return fmt.Errorf("seed history: create: %w", err)
		}
		log.Printf("Created /about/our-history/ (id=%d, %d blocks)", page.ID, len(blocks))
		return nil
	}
	if err != nil {
		return fmt.Errorf("seed history: lookup: %w", err)
	}
	existing.Blocks = blocks
	if existing.Title == "" {
		existing.Title = "Our History"
	}
	if existing.NavLabel == "" {
		existing.NavLabel = "Our History"
	}
	if existing.MetaDescription == "" {
		existing.MetaDescription = "The 175-year story of St. Ann's Church in Hornell — from a priest's visit to Thomas Doorley's home in 1843 to the 2026 Jubilee of Our Lady of the Valley Parish."
	}
	if existing.PublishedAt == nil {
		existing.PublishedAt = &now
	}
	if err := db.Save(&existing).Error; err != nil {
		return fmt.Errorf("seed history: save: %w", err)
	}
	log.Printf("Updated /about/our-history/ (id=%d, %d blocks)", existing.ID, len(blocks))
	return nil
}

// historyBlocks is the canonical ordered block list for the history page.
// Visual elements (hero banner, milestone tiles) are embedded as rich_text
// blocks containing inline SVG and self-contained styles — the page needs no
// uploaded photos to look complete, but a staffer can still add real
// photographs via the admin editor at any time.
func historyBlocks() models.Blocks {
	heading := func(level int, text string) models.Block {
		data, _ := json.Marshal(map[string]interface{}{"level": level, "text": text})
		return models.Block{Type: models.BlockHeading, Data: data}
	}
	rich := func(htmlBody string) models.Block {
		data, _ := json.Marshal(map[string]string{"html": strings.TrimSpace(htmlBody)})
		return models.Block{Type: models.BlockRichText, Data: data}
	}
	divider := func() models.Block {
		data, _ := json.Marshal(map[string]string{})
		return models.Block{Type: models.BlockDivider, Data: data}
	}

	// Hero banner: inline SVG stained-glass window motif over a deep-navy
	// gradient, with the 1843–2026 jubilee framing. Styles are inline so no
	// CSS change is required.
	hero := rich(`
<div class="parish-history-hero" style="position:relative;background:linear-gradient(135deg,#1e3a5f 0%,#0f1e30 100%);color:#faf6ef;padding:3.5rem 2rem;margin:1.5rem 0 2.5rem;border-radius:6px;text-align:center;overflow:hidden;box-shadow:0 10px 30px rgba(15,30,48,0.25);">
    <svg viewBox="0 0 100 140" width="88" height="124" aria-hidden="true" style="display:block;margin:0 auto 1.25rem;opacity:0.95;">
        <defs>
            <linearGradient id="olvGlass" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0" stop-color="#d4af37"/>
                <stop offset="1" stop-color="#9c7a1f"/>
            </linearGradient>
        </defs>
        <path d="M50 5 A45 45 0 0 1 95 50 L95 135 L5 135 L5 50 A45 45 0 0 1 50 5 Z" fill="url(#olvGlass)" opacity="0.18"/>
        <path d="M50 15 A35 35 0 0 1 85 50 L85 125 L15 125 L15 50 A35 35 0 0 1 50 15 Z" fill="none" stroke="#d4af37" stroke-width="1.5"/>
        <line x1="50" y1="20" x2="50" y2="120" stroke="#d4af37" stroke-width="1"/>
        <line x1="20" y1="70" x2="80" y2="70" stroke="#d4af37" stroke-width="1"/>
        <circle cx="50" cy="50" r="7" fill="none" stroke="#d4af37" stroke-width="1.5"/>
        <path d="M50 43 L50 57 M43 50 L57 50" stroke="#d4af37" stroke-width="1.5"/>
    </svg>
    <p style="font-family:'Inter',sans-serif;letter-spacing:0.32em;font-size:0.78rem;margin:0;text-transform:uppercase;opacity:0.85;">Our Lady of the Valley Parish</p>
    <h2 style="font-family:'Cormorant Garamond',serif;font-size:clamp(2.25rem,5vw,3.25rem);line-height:1.1;margin:0.5rem 0 0.75rem;font-weight:500;color:#faf6ef;">A History of Faith</h2>
    <p style="font-family:'Cormorant Garamond',serif;font-size:1.3rem;margin:0;letter-spacing:0.05em;opacity:0.95;">
        <span style="color:#d4af37;font-weight:600;">1843</span>
        <span style="margin:0 0.5rem;opacity:0.6;">·</span>
        <span style="color:#d4af37;font-weight:600;">2026</span>
    </p>
    <p style="font-size:1rem;margin:0.75rem 0 0;color:#ffffff;letter-spacing:0.04em;font-weight:500;">175 Years in the Southern Tier</p>
</div>
`)

	// Milestone tile: a small decorative card we drop between sections to
	// mark a key date. Styles are inline for the same self-containedness.
	milestone := func(year, title, body string) models.Block {
		return rich(fmt.Sprintf(`
<div style="display:flex;gap:1.5rem;align-items:flex-start;background:#faf6ef;border-left:4px solid #d4af37;padding:1.25rem 1.5rem;margin:2rem 0;border-radius:0 4px 4px 0;box-shadow:0 2px 8px rgba(30,58,95,0.08);">
    <div style="font-family:'Cormorant Garamond',serif;font-size:2.75rem;line-height:1;color:#1e3a5f;font-weight:600;flex-shrink:0;min-width:5rem;">%s</div>
    <div>
        <p style="margin:0 0 0.35rem;font-weight:600;color:#1e3a5f;font-size:1.05rem;">%s</p>
        <p style="margin:0;color:#4a4a4a;line-height:1.55;">%s</p>
    </div>
</div>
`, year, title, body))
	}

	// Illustration banner: a section-opener card with an inline SVG scene,
	// caption, and a note that staff can replace with a real photograph via
	// the admin editor. Each scene is themed to its section.
	illustration := func(svg, caption string) models.Block {
		return rich(fmt.Sprintf(`
<figure style="margin:1.5rem 0 2rem;background:linear-gradient(135deg,#1e3a5f 0%%,#0f1e30 100%%);border-radius:6px;padding:1.75rem 1.5rem 1.25rem;text-align:center;box-shadow:0 8px 24px rgba(15,30,48,0.18);">
    %s
    <figcaption style="color:#faf6ef;font-family:'Cormorant Garamond',serif;font-style:italic;font-size:1.05rem;margin-top:1rem;letter-spacing:0.02em;">%s</figcaption>
</figure>
`, svg, caption))
	}

	// Four themed SVG scenes. All sit in a 320x160 viewBox with navy sky
	// behind so they feel like a series, and the parish's gold accent for
	// highlights.
	svgFarmhouse := `<svg viewBox="0 0 320 160" width="100%" height="160" aria-hidden="true" style="display:block;margin:0 auto;max-width:420px;">
    <circle cx="260" cy="40" r="18" fill="#d4af37" opacity="0.3"/>
    <circle cx="260" cy="40" r="12" fill="#d4af37" opacity="0.6"/>
    <path d="M60 130 L60 90 L100 90 L100 130 Z M100 90 L100 70 L140 70 L140 130 L100 130 Z" fill="#faf6ef" opacity="0.14"/>
    <path d="M60 90 L80 70 L100 90 Z M100 70 L120 55 L140 70 Z" fill="#d4af37" opacity="0.75"/>
    <rect x="112" y="95" width="16" height="35" fill="#1e3a5f" opacity="0.7" stroke="#d4af37" stroke-width="0.5"/>
    <rect x="70" y="105" width="12" height="12" fill="#d4af37" opacity="0.6"/>
    <path d="M120 50 L120 60 M115 55 L125 55" stroke="#d4af37" stroke-width="2"/>
    <path d="M10 130 L310 130" stroke="#d4af37" stroke-width="0.5" opacity="0.3"/>
    <path d="M20 135 Q40 132 60 135 M240 135 Q270 133 300 135" stroke="#d4af37" stroke-width="0.5" opacity="0.4" fill="none"/>
</svg>`

	svgChurch := `<svg viewBox="0 0 320 160" width="100%" height="160" aria-hidden="true" style="display:block;margin:0 auto;max-width:420px;">
    <rect x="80" y="90" width="160" height="50" fill="#faf6ef" opacity="0.12"/>
    <path d="M80 90 L160 35 L240 90 Z" fill="#faf6ef" opacity="0.1"/>
    <rect x="150" y="55" width="20" height="60" fill="#1e3a5f" stroke="#d4af37" stroke-width="1"/>
    <path d="M160 45 L160 55 M155 50 L165 50" stroke="#d4af37" stroke-width="1.5"/>
    <path d="M110 115 L110 95 Q110 85 120 85 Q130 85 130 95 L130 115 Z" fill="url(#churchGlass)" opacity="0.85" stroke="#faf6ef" stroke-width="0.75"/>
    <path d="M190 115 L190 95 Q190 85 200 85 Q210 85 210 95 L210 115 Z" fill="url(#churchGlass)" opacity="0.85" stroke="#faf6ef" stroke-width="0.75"/>
    <circle cx="160" cy="95" r="9" fill="none" stroke="#d4af37" stroke-width="1.25"/>
    <path d="M160 88 L160 102 M153 95 L167 95" stroke="#d4af37" stroke-width="1"/>
    <rect x="152" y="118" width="16" height="22" fill="#2c1810" stroke="#d4af37" stroke-width="0.5"/>
    <path d="M10 140 L310 140" stroke="#d4af37" stroke-width="0.5" opacity="0.3"/>
    <defs><linearGradient id="churchGlass" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#d4af37"/><stop offset="1" stop-color="#8a6a1e"/></linearGradient></defs>
</svg>`

	svgFederation := `<svg viewBox="0 0 320 160" width="100%" height="160" aria-hidden="true" style="display:block;margin:0 auto;max-width:420px;">
    <path d="M70 60 L160 30 L250 60 Z" fill="#d4af37" opacity="0.25"/>
    <path d="M70 60 L250 60" stroke="#d4af37" stroke-width="1.5"/>
    <path d="M70 60 L160 30 L250 60" fill="none" stroke="#d4af37" stroke-width="1.5"/>
    <path d="M100 60 L100 125 M135 60 L135 125 M170 60 L170 125 M205 60 L205 125" stroke="#d4af37" stroke-width="2.5" opacity="0.9"/>
    <rect x="70" y="125" width="180" height="12" fill="#d4af37" opacity="0.6"/>
    <rect x="60" y="137" width="200" height="4" fill="#d4af37" opacity="0.5"/>
    <path d="M100 60 L100 55 L105 55 L105 60 M130 60 L130 55 L140 55 L140 60 M165 60 L165 55 L175 55 L175 60 M200 60 L200 55 L210 55 L210 60" stroke="#d4af37" stroke-width="1" fill="none"/>
    <path d="M160 20 L160 32" stroke="#d4af37" stroke-width="1.5"/>
    <path d="M156 24 L164 24" stroke="#d4af37" stroke-width="1.5"/>
    <path d="M10 141 L310 141" stroke="#d4af37" stroke-width="0.5" opacity="0.3"/>
</svg>`

	svgJubilee := `<svg viewBox="0 0 320 160" width="100%" height="160" aria-hidden="true" style="display:block;margin:0 auto;max-width:420px;">
    <g transform="translate(160 80)">
        <g stroke="#d4af37" stroke-width="1" opacity="0.5">
            <line x1="0" y1="-60" x2="0" y2="-45"/>
            <line x1="30" y1="-52" x2="22" y2="-38"/>
            <line x1="52" y1="-30" x2="38" y2="-22"/>
            <line x1="60" y1="0" x2="45" y2="0"/>
            <line x1="52" y1="30" x2="38" y2="22"/>
            <line x1="30" y1="52" x2="22" y2="38"/>
            <line x1="0" y1="60" x2="0" y2="45"/>
            <line x1="-30" y1="52" x2="-22" y2="38"/>
            <line x1="-52" y1="30" x2="-38" y2="22"/>
            <line x1="-60" y1="0" x2="-45" y2="0"/>
            <line x1="-52" y1="-30" x2="-38" y2="-22"/>
            <line x1="-30" y1="-52" x2="-22" y2="-38"/>
        </g>
        <circle r="30" fill="none" stroke="#d4af37" stroke-width="1.5"/>
        <circle r="22" fill="#faf6ef" opacity="0.95"/>
        <text y="4" text-anchor="middle" font-family="Cormorant Garamond, serif" font-size="14" font-weight="600" fill="#1e3a5f" letter-spacing="0.05em">IHS</text>
        <line x1="0" y1="-30" x2="0" y2="-42" stroke="#d4af37" stroke-width="1.5"/>
        <line x1="-5" y1="-37" x2="5" y2="-37" stroke="#d4af37" stroke-width="1.5"/>
    </g>
    <text x="160" y="150" text-anchor="middle" font-family="Cormorant Garamond, serif" font-size="20" font-weight="600" fill="#d4af37" letter-spacing="0.1em">175 · 1843–2026</text>
</svg>`

	return models.Blocks{
		heading(1, "A History of Faith in the Southern Tier"),
		rich(`
<p class="parish-prose-lede">For more than 175 years, Catholics in Hornell and the surrounding Southern Tier have gathered to pray, serve, and raise their families in the faith. What began with a single Mass in a farmhouse in 1843 is today Our Lady of the Valley Parish — two historic churches, one community, in the Diocese of Rochester.</p>
`),

		hero,

		milestone("1843", "The First Mass",
			"Father Benedict Bayer celebrates Mass in the home of Thomas Doorley, near Webb's Crossing. Doorley will later mortgage his farm to help build the church."),

		heading(2, "Early Origins (1843–1851)"),
		illustration(svgFarmhouse, "A farmhouse near Webb's Crossing — the first home of the parish, 1843"),
		rich(`
<p>Catholic missionary activity in the Hornellsville area began in the 1840s, when <strong>Redemptorist priests</strong> rode circuit to minister to the Irish immigrant laborers building the railroads of western New York. In <strong>1843</strong>, Father Benedict Bayer celebrated the first Mass in the area in the home of Thomas Doorley, near Webb's Crossing — a farmhouse that would be remembered as the cradle of the parish.</p>
<p>Doorley became the first subscriber to the church building fund in 1849, mortgaging his farm to help put a roof over the altar. That same year — after the <strong>Diocese of Buffalo</strong> was established in 1847 and Bishop John Timon appointed <strong>Reverend Michael O'Brien</strong> as first pastor — the parish was formally organized and a first church was built at the northwest corner of Cass and Buffalo Streets.</p>
<p>Regular weekly Sunday Masses began in <strong>1851</strong> under Reverend Daniel Delacy Moore. That date is the one the parish counts from, which is why 2026 marks the <strong>175th anniversary</strong>.</p>
`),

		milestone("1869", "The Present Church Dedicated",
			"On April 11, Father Michael Creedon dedicates the French Gothic Revival Church of St. Ann on Erie Avenue. Its 1870s pipe organ still sounds today."),

		heading(2, "A New Church and a Parish School (1860s–1890s)"),
		illustration(svgChurch, "The Church of St. Ann in the French Gothic Revival style — dedicated April 11, 1869"),
		rich(`
<p>By 1860 the congregation had outgrown its first home. <strong>Father Michael Creedon</strong> arrived as pastor and set about building a church to match the parish's growth. The <strong>cornerstone was laid in 1868</strong>, and on <strong>April 11, 1869</strong>, the present Church of St. Ann was dedicated. The building is designed in the <em>French Gothic Revival</em> style — soaring pointed arches, a steep slate roof, a bell tower rising above Erie Avenue. A major addition was completed in <strong>1889</strong>, giving the church its current footprint. The sanctuary still houses its original <strong>1870s pipe organ</strong>.</p>
<p>Father Creedon also founded <strong>St. Ann School</strong> in 1863 — the beginning of Catholic education in Hornell. A larger school building opened in 1890 with capacity for roughly 300 students; a wing was added in 1962. The school served generations of families until it closed for financial reasons in 2012. The parish responded by opening <strong>St. Ann's Academy</strong> in 2013, continuing the educational mission in a new form.</p>
<p>In <strong>1897</strong> the parish was transferred from the Diocese of Buffalo to the newly expanded <strong>Diocese of Rochester</strong>, where it has remained ever since.</p>
`),

		milestone("1912", "The Federation Building",
			"Parishioners complete the Neoclassical community hall designed by Otis Dockstader. It is added to the National Register of Historic Places in 2001."),

		heading(2, "A Parish in a Growing City (1910–2000)"),
		illustration(svgFederation, "The St. Ann's Federation Building — Neoclassical, 1910–1912 — added to the National Register in 2001"),
		rich(`
<p>As Hornell's railroad economy flourished, so did the parish. Between <strong>1910 and 1912</strong>, parishioners built the <strong>St. Ann's Federation Building</strong> — a community center with an auditorium and dance hall designed by architect Otis Dockstader in the Neoclassical style. The building, which still stands near the church, was added to the <strong>National Register of Historic Places in 2001</strong>.</p>
<p>In 1998, St. Ann's was clustered with three neighboring parishes: <strong>St. Mary in Rexville</strong>, St. Ignatius Loyola in Hornell, and St. Joachim in Canisteo. In <strong>2004</strong>, those four communities were reorganized into a single parish — <strong>Our Lady of the Valley</strong>. St. Joachim and St. Ignatius Loyola were later closed; the parish today encompasses the historic churches of St. Ann in Hornell and St. Mary in Rexville. A <strong>perpetual adoration chapel</strong> has operated on the St. Ann's campus since 2001.</p>
<p>In April 2004, a fire — caused by an improperly discarded charcoal incense briquet after Benediction — damaged the sacristy and left smoke damage throughout the church. Masses were temporarily relocated while cleanup proceeded; the historic organ was carefully disassembled and cleaned by hand. In the early 2020s a further round of exterior restoration — brick repointing and steeple work — addressed moisture issues and preserved the building for the next generation.</p>
`),

		milestone("2004", "Our Lady of the Valley",
			"St. Ann (Hornell), St. Mary (Rexville), St. Ignatius Loyola, and St. Joachim merge into a single parish. A perpetual adoration chapel opens on the St. Ann campus."),

		heading(2, "The 175th Jubilee (2025–2026)"),
		illustration(svgJubilee, "The 175th Jubilee monstrance — 1843–2026"),
		rich(`
<p>The parish observed a <strong>year-long 175th Jubilee</strong>, culminating on <strong>July 26, 2026</strong> — the Feast of Saints Joachim and Anne — with a Mass celebrated by <strong>Bishop John S. Bonnici</strong> of the Diocese of Rochester.</p>
<p>Jubilee events included:</p>
<ul>
    <li>A prayer service near Webb's Crossing on the ground where the first Mass was celebrated in 1843.</li>
    <li>A gathering at the corner of Cass and Buffalo Streets, where the first church building stood from 1849.</li>
    <li>Guided tours of the church, a sand sculpture of the church façade, and a time capsule to be opened by a future generation.</li>
    <li>Legislative resolutions from the New York State Senate and Assembly honoring the parish's 175 years of service.</li>
</ul>
<p>Today, Our Lady of the Valley reports about <strong>600 active members</strong>. The Church of St. Ann at <strong>27 Erie Avenue</strong> and St. Mary's in Rexville continue a story that is older than the Diocese of Rochester itself — and, God willing, far from finished.</p>
`),

		milestone("2026", "175 Years",
			"Bishop John S. Bonnici celebrates the Jubilee Mass on the Feast of Saints Joachim and Anne. About 600 active members carry the story forward."),

		divider(),
		rich(`
<p class="parish-prose-small"><em>Sources: Hornell Sun — "St. Ann's Church and School: A Pillar of Faith and Community in Hornell"; Catholic Courier; Evening Tribune; WENY — "Saint Ann's Church in Hornell Celebrates Jubilee"; NYS Senate archives; National Register of Historic Places (Federation Building, listed 2001).</em></p>
<p class="parish-prose-small"><em>Historic photographs can be added to any section via the admin editor. The parish archives, public-domain postcards, and Diocese of Rochester records are all good sources.</em></p>
`),
	}
}
