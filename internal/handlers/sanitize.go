package handlers

import (
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Inline style properties page content may use (the Our History page's hero,
// milestone tiles and photo frames rely on these).
var allowedStyleProps = []string{
	"align-items", "background", "background-color", "border", "border-bottom",
	"border-left", "border-radius", "border-right", "border-top", "box-shadow",
	"color", "display", "flex", "flex-direction", "flex-shrink", "flex-wrap",
	"font-family", "font-size", "font-style", "font-weight", "gap", "height",
	"justify-content", "letter-spacing", "line-height", "list-style",
	"margin", "margin-bottom", "margin-left", "margin-right", "margin-top",
	"max-height", "max-width", "min-height", "min-width", "object-fit",
	"opacity", "overflow", "padding", "padding-bottom", "padding-left",
	"padding-right", "padding-top", "position", "text-align",
	"text-decoration", "text-shadow", "text-transform", "vertical-align",
	"white-space", "width",
}

var (
	styleURLRE     = regexp.MustCompile(`(?i)url\(\s*(['"]?)([^'")]*)['"]?\s*\)`)
	idValueRE      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
	targetBlankRE  = regexp.MustCompile(`^_blank$`)
	relValueRE     = regexp.MustCompile(`^[a-z ]{1,40}$`)
	embedSrcHostRE = regexp.MustCompile(`(?i)^https://(www\.)?(youtube(-nocookie)?\.com|player\.vimeo\.com|facebook\.com|google\.com/maps|calendar\.google\.com|docs\.google\.com)/`)
)

// safeStyleValue rejects CSS values that can execute script or load
// off-site resources. url(...) is only allowed for files we serve.
func safeStyleValue(v string) bool {
	lv := strings.ToLower(v)
	for _, bad := range []string{"expression", "javascript", "vbscript", "behavior", "-moz-binding", "@import", "\\"} {
		if strings.Contains(lv, bad) {
			return false
		}
	}
	for _, m := range styleURLRE.FindAllStringSubmatch(v, -1) {
		if !strings.HasPrefix(strings.TrimSpace(m[2]), "/static/") {
			return false
		}
	}
	return true
}

func basePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoFollowOnLinks(false)
	p.RequireNoFollowOnFullyQualifiedLinks(false)
	p.AllowURLSchemes("tel")
	p.AllowElements("figure", "figcaption", "time", "address", "section", "article", "header")
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("id").Matching(idValueRE).Globally()
	p.AllowAttrs("aria-hidden", "aria-label", "role", "title").Globally()
	p.AllowAttrs("target").Matching(targetBlankRE).OnElements("a")
	p.AllowAttrs("rel").Matching(relValueRE).OnElements("a")
	p.AllowAttrs("loading", "decoding").OnElements("img", "iframe")
	p.AllowAttrs("datetime").OnElements("time")
	p.AllowStyles(allowedStyleProps...).MatchingHandler(safeStyleValue).Globally()
	return p
}

var (
	contentPolicy = basePolicy()
	// The "Embedded HTML" block additionally allows iframes from known
	// embed providers (YouTube, Vimeo, Facebook, Google Maps/Calendar/Docs).
	embedPolicy = func() *bluemonday.Policy {
		p := basePolicy()
		p.AllowElements("iframe")
		p.AllowAttrs("src").Matching(embedSrcHostRE).OnElements("iframe")
		p.AllowAttrs("width", "height", "frameborder", "allow", "allowfullscreen", "scrolling").OnElements("iframe")
		return p
	}()
)
