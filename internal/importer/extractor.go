package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"olv-parish/internal/models"

	"golang.org/x/net/html"
)

// fetchHTML gets a page and parses it. 15-second timeout to keep imports from
// hanging on slow pages.
func fetchHTML(target string) (*html.Node, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OLV-Parish-Importer/1.0 (+https://github.com/)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, target)
	}
	return html.Parse(resp.Body)
}

// extractor builds a Blocks list from a parsed HTML document.
type extractor struct {
	imp    *importer
	page   pageSpec
	source *url.URL
}

func newExtractor(imp *importer, p pageSpec) *extractor {
	source, _ := url.Parse(imp.base + p.URL)
	return &extractor{imp: imp, page: p, source: source}
}

// extract walks the main content container tree and returns an ordered Blocks
// list. We walk recursively looking for "content leaves" — p, h1-h6, img,
// iframe, pdf links — rather than relying on children of a single container,
// because this site wraps its text 4+ levels deep in <ul class="moduleRegion">
// / <li class="moduleInner"> / <div class="moduleBody"> / <p>.
func (e *extractor) extract(root *html.Node) (models.Blocks, error) {
	container := findMainContent(root)
	if container == nil {
		return nil, fmt.Errorf("no main content container found")
	}
	var out models.Blocks
	e.walk(container, &out)
	out = mergeRichText(out)
	return out, nil
}

// walk descends into a node, emitting blocks for content leaves. Containers
// (div, section, ul, li, figure, main, article, header, …) are recursed into;
// leaves (p, h2, img, iframe, …) emit a block and stop.
func (e *extractor) walk(n *html.Node, out *models.Blocks) {
	if n == nil {
		return
	}
	if n.Type != html.ElementNode {
		// Skip top-level text / comments outside a <p> — the site wraps text
		// in <p> anyway, so loose text is almost always whitespace.
		return
	}
	tag := strings.ToLower(n.Data)

	// Hard skips.
	if _, skip := skipTags[tag]; skip {
		return
	}
	class := getAttr(n, "class")
	if isBoilerplateClass(class) {
		return
	}

	// Leaf cases — emit a block and stop recursing.
	if block, isLeaf := e.leafBlock(n, tag); isLeaf {
		if block != nil {
			*out = append(*out, *block)
		}
		return
	}

	// Container — recurse into children.
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.walk(c, out)
	}
}

var skipTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {},
	"nav": {}, "header": {}, "footer": {}, "aside": {},
	"form": {}, "button": {}, "input": {}, "select": {}, "textarea": {},
	"svg": {}, "canvas": {}, "video": {}, "audio": {},
}

func isBoilerplateClass(class string) bool {
	for _, c := range strings.Fields(class) {
		switch c {
		case "sharedaddy", "jp-relatedposts", "wp-block-social-links",
			"social-share", "sharing", "crumbs", "breadcrumbs",
			"nav", "navbar", "navigation", "menu", "mainMenu",
			"footer", "siteFooter", "siteHeader", "socialLinks",
			"skipToContent", "screen-reader-text",
			"pageTitle", "siteTitle":
			return true
		}
	}
	return false
}

// leafBlock recognizes the element types we treat as content leaves.
func (e *extractor) leafBlock(n *html.Node, tag string) (*models.Block, bool) {
	switch tag {
	case "h1":
		if sameAsTitle(textOf(n), e.page.Title) {
			return nil, true // skip the page title
		}
		return makeHeading(2, textOf(n)), true
	case "h2":
		return makeHeading(2, textOf(n)), true
	case "h3":
		return makeHeading(3, textOf(n)), true
	case "h4", "h5", "h6":
		return makeHeading(4, textOf(n)), true

	case "img":
		return e.imageBlock(n), true

	case "iframe":
		src := getAttr(n, "src")
		if src == "" {
			return nil, true
		}
		return makeVideo(detectProvider(src), src), true

	case "p":
		// Pure-image <p> → image block.
		if img := singleImage(n); img != nil {
			return e.imageBlock(img), true
		}
		// Pure-PDF-link <p> → pdf_link block.
		if link := singlePDFLink(n); link != nil {
			if block := e.pdfLinkBlock(link); block != nil {
				return block, true
			}
		}
		// Otherwise emit the <p>'s rendered HTML as a rich_text block.
		if h := strings.TrimSpace(renderInnerHTML(n)); h != "" {
			data, _ := json.Marshal(richTextData{HTML: "<p>" + h + "</p>"})
			return &models.Block{Type: models.BlockRichText, Data: data}, true
		}
		return nil, true

	case "ul", "ol":
		// A simple list (every <li> is text / inline elements only) is a leaf
		// — emit as rich_text so the bullet formatting survives. A layout-ish
		// list (<li> with nested <div>, <ul>, etc.) is a container; recurse.
		if !isSimpleList(n) {
			return nil, false
		}
		var buf bytes.Buffer
		if err := html.Render(&buf, n); err == nil {
			if h := strings.TrimSpace(buf.String()); h != "" {
				data, _ := json.Marshal(richTextData{HTML: h})
				return &models.Block{Type: models.BlockRichText, Data: data}, true
			}
		}
		return nil, true

	case "blockquote", "table", "pre":
		var buf bytes.Buffer
		if err := html.Render(&buf, n); err == nil {
			if h := strings.TrimSpace(buf.String()); h != "" {
				data, _ := json.Marshal(richTextData{HTML: h})
				return &models.Block{Type: models.BlockRichText, Data: data}, true
			}
		}
		return nil, true

	case "figure", "picture":
		// Treat picture/figure as a container holding an img.
		if img := firstTag(n, "img"); img != nil {
			return e.imageBlock(img), true
		}
		return nil, false // let walker recurse
	}
	return nil, false // not a leaf, keep walking
}

// inlineTags lists elements allowed inside a "simple" list item without
// disqualifying the list from being a leaf.
var inlineTags = map[string]struct{}{
	"a": {}, "b": {}, "strong": {}, "i": {}, "em": {}, "span": {}, "br": {},
	"u": {}, "small": {}, "sup": {}, "sub": {}, "code": {}, "mark": {},
	"abbr": {}, "cite": {}, "q": {}, "s": {}, "time": {},
}

func isSimpleList(ul *html.Node) bool {
	for c := ul.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if strings.ToLower(c.Data) != "li" {
			return false // non-<li> child (whitespace is a TextNode, ok)
		}
		if !isInlineOnly(c) {
			return false
		}
	}
	return true
}

func isInlineOnly(li *html.Node) bool {
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			continue
		}
		if c.Type != html.ElementNode {
			continue
		}
		tag := strings.ToLower(c.Data)
		if _, ok := inlineTags[tag]; !ok {
			return false
		}
		// Also check recursively — inline elements can nest other inline
		// elements, but any block element nested inside disqualifies.
		if !isInlineOnly(c) {
			return false
		}
	}
	return true
}

// renderInnerHTML returns the HTML of n's children (not n itself).
func renderInnerHTML(n *html.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&buf, c)
	}
	return buf.String()
}

type richTextData struct {
	HTML string `json:"html"`
}

type headingData struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type imageData struct {
	AssetID uint   `json:"asset_id"`
	Alt     string `json:"alt"`
	Caption string `json:"caption"`
	Size    string `json:"size"`
}

type pdfLinkData struct {
	AssetID uint   `json:"asset_id"`
	Label   string `json:"label"`
}

type videoEmbedData struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

// --------- Image / PDF helpers (asset download goes through imp) ---------

func (e *extractor) imageBlock(img *html.Node) *models.Block {
	src := absoluteURL(e.source, getAttr(img, "src"))
	alt := strings.TrimSpace(getAttr(img, "alt"))
	if src == "" {
		return nil
	}
	assetID, err := e.imp.downloadAsset(src, models.AssetImage)
	if err != nil {
		if e.imp.verbose {
			fmt.Printf("[asset-fail] %s: %v\n", src, err)
		}
		// Fall back to a rich_text block that just keeps the original image URL.
		h := fmt.Sprintf(`<p><img src=%q alt=%q></p>`, src, alt)
		data, _ := json.Marshal(richTextData{HTML: h})
		return &models.Block{Type: models.BlockRichText, Data: data}
	}
	data, _ := json.Marshal(imageData{AssetID: assetID, Alt: alt, Size: "medium"})
	return &models.Block{Type: models.BlockImage, Data: data}
}

func (e *extractor) pdfLinkBlock(a *html.Node) *models.Block {
	href := absoluteURL(e.source, getAttr(a, "href"))
	label := strings.TrimSpace(textOf(a))
	if href == "" {
		return nil
	}
	assetID, err := e.imp.downloadAsset(href, models.AssetPDF)
	if err != nil {
		if e.imp.verbose {
			fmt.Printf("[asset-fail] %s: %v\n", href, err)
		}
		// Fall back to a plain link.
		h := fmt.Sprintf(`<p><a href=%q target="_blank" rel="noopener">%s</a></p>`, href, escText(label))
		data, _ := json.Marshal(richTextData{HTML: h})
		return &models.Block{Type: models.BlockRichText, Data: data}
	}
	if label == "" {
		label = "View PDF"
	}
	data, _ := json.Marshal(pdfLinkData{AssetID: assetID, Label: label})
	return &models.Block{Type: models.BlockPDFLink, Data: data}
}

// --------- HTML tree helpers ---------

func findMainContent(root *html.Node) *html.Node {
	// WordPress-ish: .entry-content > main-article > body
	candidates := []string{"entry-content", "post-content", "page-content", "mainContent", "pageContent"}
	for _, cls := range candidates {
		if n := findByClass(root, cls); n != nil {
			return n
		}
	}
	// Try <main>, <article>, <body>.
	if n := firstTag(root, "main"); n != nil {
		return n
	}
	if n := firstTag(root, "article"); n != nil {
		return n
	}
	return firstTag(root, "body")
}

func findByClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && hasClass(n, class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := findByClass(c, class); r != nil {
			return r
		}
	}
	return nil
}

func firstTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && strings.ToLower(n.Data) == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if r := firstTag(c, tag); r != nil {
			return r
		}
	}
	return nil
}

func firstMatching(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			return c
		}
		if r := firstMatching(c, tag); r != nil {
			return r
		}
	}
	return nil
}

func hasClass(n *html.Node, cls string) bool {
	for _, c := range strings.Fields(getAttr(n, "class")) {
		if c == cls {
			return true
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(buf.String())
}

func singlePDFLink(p *html.Node) *html.Node {
	var link *html.Node
	// A <p> that contains one <a href="*.pdf"> and no other substantive content.
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == "a" {
			if strings.HasSuffix(strings.ToLower(getAttr(c, "href")), ".pdf") {
				if link != nil {
					return nil
				}
				link = c
				continue
			}
		}
		// Any other substantive node disqualifies.
		if c.Type == html.ElementNode || (c.Type == html.TextNode && strings.TrimSpace(c.Data) != "") {
			return nil
		}
	}
	return link
}

func singleImage(p *html.Node) *html.Node {
	var img *html.Node
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == "img" {
			if img != nil {
				return nil
			}
			img = c
			continue
		}
		// Allow <a><img></a>
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == "a" {
			inner := firstTag(c, "img")
			if inner != nil && img == nil {
				img = inner
				continue
			}
		}
		if c.Type == html.ElementNode || (c.Type == html.TextNode && strings.TrimSpace(c.Data) != "") {
			return nil
		}
	}
	return img
}

func sameAsTitle(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func detectProvider(src string) string {
	s := strings.ToLower(src)
	switch {
	case strings.Contains(s, "youtube.com") || strings.Contains(s, "youtu.be"):
		return "youtube"
	case strings.Contains(s, "vimeo.com"):
		return "vimeo"
	case strings.Contains(s, "facebook.com"):
		return "facebook"
	}
	return ""
}

func absoluteURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	if base == nil {
		return ref.String()
	}
	return base.ResolveReference(ref).String()
}

// --------- Block constructors ---------

func makeHeading(level int, text string) *models.Block {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	data, _ := json.Marshal(headingData{Level: level, Text: text})
	return &models.Block{Type: models.BlockHeading, Data: data}
}

func makeVideo(provider, src string) *models.Block {
	data, _ := json.Marshal(videoEmbedData{Provider: provider, URL: src})
	return &models.Block{Type: models.BlockVideoEmbed, Data: data}
}

// mergeRichText collapses runs of adjacent rich_text blocks into one block.
func mergeRichText(in models.Blocks) models.Blocks {
	out := make(models.Blocks, 0, len(in))
	for _, b := range in {
		if len(out) > 0 && out[len(out)-1].Type == models.BlockRichText && b.Type == models.BlockRichText {
			var prev, cur richTextData
			_ = json.Unmarshal(out[len(out)-1].Data, &prev)
			_ = json.Unmarshal(b.Data, &cur)
			merged, _ := json.Marshal(richTextData{HTML: prev.HTML + "\n" + cur.HTML})
			out[len(out)-1].Data = merged
			continue
		}
		out = append(out, b)
	}
	return out
}

func escText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
