package middleware

import (
	"net/http"
	"strings"
	"time"

	"olv-parish/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var store *sessions.CookieStore

func SetupMiddleware(router *gin.Engine, cfg *config.Config) {
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.Frontend.URL, "http://localhost:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	store = sessions.NewCookieStore([]byte(cfg.Session.Secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	}

	router.Use(SessionMiddleware())

	router.Use(func(c *gin.Context) {
		if accept := c.Request.Header.Get("Accept"); strings.Contains(accept, "text/html") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0, private")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
			c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			c.Header("X-Content-Type-Options", "nosniff")
		}
		c.Next()
	})
}

func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := store.Get(c.Request, "olv-parish-session")
		c.Set("session", session)
		c.Next()
	}
}

func GetSession(c *gin.Context) *sessions.Session {
	session, _ := c.Get("session")
	return session.(*sessions.Session)
}

func SaveSession(c *gin.Context) error {
	session := GetSession(c)
	return session.Save(c.Request, c.Writer)
}
