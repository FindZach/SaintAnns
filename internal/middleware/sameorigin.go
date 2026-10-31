package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// SameOriginWrites rejects state-changing /api requests sent by another site.
// Browsers attach Origin to every cross-site POST/PUT/PATCH/DELETE (and
// Referer as a fallback), so a write whose Origin/Referer host differs from
// this server's host — or from an explicitly trusted origin — is a forged
// cross-site request riding on the admin's auth cookie. Requests with neither
// header come from non-browser clients, which can't carry a victim's cookie,
// so they're allowed through to the normal auth checks.
func SameOriginWrites(trustedOrigins ...string) gin.HandlerFunc {
	trusted := map[string]bool{}
	for _, o := range trustedOrigins {
		if u, err := url.Parse(strings.TrimSpace(o)); err == nil && u.Host != "" {
			trusted[strings.ToLower(u.Host)] = true
		}
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		source := c.GetHeader("Origin")
		if source == "" {
			source = c.GetHeader("Referer")
		}
		if source == "" {
			c.Next()
			return
		}
		u, err := url.Parse(source)
		host := ""
		if err == nil {
			host = strings.ToLower(u.Host)
		}
		if host == "" || (host != strings.ToLower(c.Request.Host) && !trusted[host]) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Cross-site request blocked"})
			return
		}
		c.Next()
	}
}
