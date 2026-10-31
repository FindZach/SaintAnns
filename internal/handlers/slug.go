package handlers

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var slugSegmentRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validSlugPath accepts "about" or "about/our-history": one or more
// lowercase segments joined by single slashes.
func validSlugPath(s string) bool {
	if s == "" {
		return false
	}
	for _, seg := range strings.Split(s, "/") {
		if !slugSegmentRE.MatchString(seg) {
			return false
		}
	}
	return true
}

// slugify turns a title like "St. Ann's Fall Festival!" into
// "st-anns-fall-festival".
func slugify(s string) string {
	s = norm.NFKD.String(strings.ToLower(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r == '\'' || r == '’' || unicode.Is(unicode.Mn, r):
			// drop apostrophes and accent marks
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out
}
