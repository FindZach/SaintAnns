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
// re-running overwrites the Blocks with the latest canonical body but leaves any
// manual admin edits to Title / NavLabel / MetaDescription intact.
//
// Content is drawn from the Hornell Sun / Catholic Courier / Evening Tribune /
// WENY coverage of St. Ann's 175th Jubilee, cross-checked against the Diocese of
// Buffalo's 1847 founding, the Diocese of Rochester's 1897 transfer, and the
// National Register of Historic Places listing of the St. Ann Federation
// Building (added 2001).
func SeedHistoryPage(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("seed history: no database connection")
	}
	log.Println("Seeding parish history page at /about/our-history/ ...")

	// Hero is always a placeholder slot: WordPress filenames don't tell us
	// which St. Ann image is the church exterior vs. the Perpetual Adoration
	// Chapel sign, and automatic matches have been wrong. Public page silently
	// omits the figure until admin opens /admin/pages/about/our-history/,
	// clicks the hero image slot, and picks a true exterior from the asset
	// picker (or uploads a new one — public-domain postcards work well).
	heroAssetID := uint(0)

	blocks := historyBlocks(heroAssetID)

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
		log.Printf("Created /about/our-history/ (id=%d, hero_asset=%d)", page.ID, heroAssetID)
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
	log.Printf("Updated /about/our-history/ (id=%d, hero_asset=%d, %d blocks)", existing.ID, heroAssetID, len(blocks))
	return nil
}


// historyBlocks is the canonical ordered block list for the history page.
// Kept here (not in a template) so it round-trips through the admin editor:
// a staffer can reorder or edit any block and re-save with no code change.
func historyBlocks(heroAssetID uint) models.Blocks {
	heading := func(level int, text string) models.Block {
		data, _ := json.Marshal(map[string]interface{}{"level": level, "text": text})
		return models.Block{Type: models.BlockHeading, Data: data}
	}
	rich := func(htmlBody string) models.Block {
		data, _ := json.Marshal(map[string]string{"html": strings.TrimSpace(htmlBody)})
		return models.Block{Type: models.BlockRichText, Data: data}
	}
	image := func(assetID uint, alt, caption, size string) models.Block {
		data, _ := json.Marshal(map[string]interface{}{
			"asset_id": assetID,
			"alt":      alt,
			"caption":  caption,
			"size":     size,
		})
		return models.Block{Type: models.BlockImage, Data: data}
	}
	divider := func() models.Block {
		data, _ := json.Marshal(map[string]string{})
		return models.Block{Type: models.BlockDivider, Data: data}
	}

	out := models.Blocks{
		heading(1, "A History of Faith in the Southern Tier"),
		rich(`
<p class="parish-prose-lede">For more than 175 years, Catholics in Hornell and the surrounding Southern Tier have gathered to pray, serve, and raise their families in the faith. What began with a single Mass in a farmhouse in 1843 is today Our Lady of the Valley Parish — two historic churches, one community, in the Diocese of Rochester.</p>
`),
	}

	// Hero: the modern exterior from the Asset library, if we found a true
	// exterior shot. If nothing matched, leave a placeholder slot so admin
	// sees an asset-picker in the editor and can upload the right photo.
	if heroAssetID != 0 {
		out = append(out, image(heroAssetID, "St. Ann's Church, Hornell", "St. Ann's Church, 27 Erie Avenue, Hornell — dedicated April 11, 1869.", "wide"))
	} else {
		out = append(out, image(0, "St. Ann's Church, Hornell", "Upload an exterior photograph of St. Ann's Church here — the hero image for the history page.", "wide"))
	}

	out = append(out,
		heading(2, "Early Origins (1843–1851)"),
		rich(`
<p>Catholic missionary activity in the Hornellsville area began in the 1840s, when <strong>Redemptorist priests</strong> rode circuit to minister to the Irish immigrant laborers building the railroads of western New York. In <strong>1843</strong>, Father Benedict Bayer celebrated the first Mass in the area in the home of Thomas Doorley, near Webb's Crossing — a farmhouse that would be remembered as the cradle of the parish.</p>
<p>Doorley became the first subscriber to the church building fund in 1849, mortgaging his farm to help put a roof over the altar. That same year — after the <strong>Diocese of Buffalo</strong> was established in 1847 and Bishop John Timon appointed <strong>Reverend Michael O'Brien</strong> as first pastor — the parish was formally organized and a first church was built at the northwest corner of Cass and Buffalo Streets.</p>
<p>Regular weekly Sunday Masses began in <strong>1851</strong> under Reverend Daniel Delacy Moore. That date is the one the parish counts from, which is why 2026 marks the <strong>175th anniversary</strong>.</p>
`),
		// Placeholder slot for an early postcard of the first church (Cass & Buffalo).
		// asset_id=0 means the public page skips it silently; the admin editor shows
		// it as an asset-picker slot so staff can drop a historic photo in.
		image(0, "Early photograph of St. Ann's Church", "Upload a historic photograph here — e.g. an early postcard view of St. Ann's.", "wide"),

		heading(2, "A New Church and a Parish School (1860s–1890s)"),
		rich(`
<p>By 1860 the congregation had outgrown its first home. <strong>Father Michael Creedon</strong> arrived as pastor and set about building a church to match the parish's growth. The <strong>cornerstone was laid in 1868</strong>, and on <strong>April 11, 1869</strong>, the present Church of St. Ann was dedicated. The building is designed in the <em>French Gothic Revival</em> style — soaring pointed arches, a steep slate roof, a bell tower rising above Erie Avenue. A major addition was completed in <strong>1889</strong>, giving the church its current footprint. The sanctuary still houses its original <strong>1870s pipe organ</strong>.</p>
<p>Father Creedon also founded <strong>St. Ann School</strong> in 1863 — the beginning of Catholic education in Hornell. A larger school building opened in 1890 with capacity for roughly 300 students; a wing was added in 1962. The school served generations of families until it closed for financial reasons in 2012. The parish responded by opening <strong>St. Ann's Academy</strong> in 2013, continuing the educational mission in a new form.</p>
<p>In <strong>1897</strong> the parish was transferred from the Diocese of Buffalo to the newly expanded <strong>Diocese of Rochester</strong>, where it has remained ever since.</p>
`),
		// Placeholder slot for the historic St. Ann's School / Academy postcard.
		image(0, "St. Ann's School, c. 1910", "Upload a historic photograph of St. Ann's School here — e.g. the 1890 school building postcard.", "wide"),

		heading(2, "A Parish in a Growing City (1910–2000)"),
		rich(`
<p>As Hornell's railroad economy flourished, so did the parish. Between <strong>1910 and 1912</strong>, parishioners built the <strong>St. Ann's Federation Building</strong> — a community center with an auditorium and dance hall designed by architect Otis Dockstader in the Neoclassical style. The building, which still stands near the church, was added to the <strong>National Register of Historic Places in 2001</strong>.</p>
<p>In 1998, St. Ann's was clustered with three neighboring parishes: <strong>St. Mary in Rexville</strong>, St. Ignatius Loyola in Hornell, and St. Joachim in Canisteo. In <strong>2004</strong>, those four communities were reorganized into a single parish — <strong>Our Lady of the Valley</strong>. St. Joachim and St. Ignatius Loyola were later closed; the parish today encompasses the historic churches of St. Ann in Hornell and St. Mary in Rexville. A <strong>perpetual adoration chapel</strong> has operated on the St. Ann's campus since 2001.</p>
<p>In April 2004, a fire — caused by an improperly discarded charcoal incense briquet after Benediction — damaged the sacristy and left smoke damage throughout the church. Masses were temporarily relocated while cleanup proceeded; the historic organ was carefully disassembled and cleaned by hand. In the early 2020s a further round of exterior restoration — brick repointing and steeple work — addressed moisture issues and preserved the building for the next generation.</p>
`),
		// Placeholder slot for the Federation Building (NRHP 2001).
		image(0, "St. Ann's Federation Building, Hornell", "Upload a photograph of the Federation Building here (Otis Dockstader, 1910–1912; added to the National Register of Historic Places in 2001).", "medium"),

		heading(2, "The 175th Jubilee (2025–2026)"),
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
		// Placeholder slot for a Jubilee photo (Bishop's Mass, sand sculpture, etc.).
		image(0, "175th Jubilee celebration, July 2026", "Upload a Jubilee photograph here — e.g. Bishop Bonnici's Mass, the sand sculpture of the church, or the Webb's Crossing prayer service.", "wide"),

		divider(),
		rich(`
<p class="parish-prose-small"><em>Sources: Hornell Sun — "St. Ann's Church and School: A Pillar of Faith and Community in Hornell"; Catholic Courier; Evening Tribune; WENY — "Saint Ann's Church in Hornell Celebrates Jubilee"; NYS Senate archives; National Register of Historic Places (Federation Building, listed 2001).</em></p>
`),
	)

	return out
}
