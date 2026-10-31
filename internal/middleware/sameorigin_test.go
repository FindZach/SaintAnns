package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSameOriginWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOriginWrites("http://localhost:8080"))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.POST("/api/v1/admin/pages", ok)
	r.GET("/api/v1/admin/assets", ok)
	r.POST("/contact-form", ok)

	cases := []struct {
		name, method, path, origin, referer string
		want                                int
	}{
		{"same origin write", "POST", "/api/v1/admin/pages", "https://parish.test", "", 200},
		{"foreign origin write", "POST", "/api/v1/admin/pages", "https://evil.parish.test", "", 403},
		{"null origin write", "POST", "/api/v1/admin/pages", "null", "", 403},
		{"foreign referer, no origin", "POST", "/api/v1/admin/pages", "", "https://evil.example/x", 403},
		{"same-site referer, no origin", "POST", "/api/v1/admin/pages", "", "https://parish.test/admin", 200},
		{"no headers (script/curl)", "POST", "/api/v1/admin/pages", "", "", 200},
		{"trusted dev origin", "POST", "/api/v1/admin/pages", "http://localhost:8080", "", 200},
		{"foreign origin read is allowed", "GET", "/api/v1/admin/assets", "https://evil.example", "", 200},
		{"non-api path untouched", "POST", "/contact-form", "https://evil.example", "", 200},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, "https://parish.test"+tc.path, nil)
		req.Host = "parish.test"
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.referer != "" {
			req.Header.Set("Referer", tc.referer)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, w.Code, tc.want)
		}
	}
}
