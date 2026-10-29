package routes

import (
	"net/http"
	"strings"

	"olv-parish/internal/config"
	"olv-parish/internal/handlers"
	"olv-parish/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Static files
	static := router.Group("/static")
	if gin.IsDebugging() {
		static.Use(func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache")
			c.Next()
		})
	}
	static.Static("/", "./web/static")

	// Template funcMap closes over the DB so {{nav}} can query live.
	router.SetFuncMap(handlers.TemplateFuncs(db))
	router.LoadHTMLGlob("web/templates/*")

	publicHandler := handlers.NewPublicHandler(db)
	scheduleHandler := handlers.NewScheduleHandler(db)
	calendarHandler := handlers.NewCalendarHandler(db)
	registrationHandler := handlers.NewRegistrationHandler(db, cfg)

	// ---------- Public ----------
	router.GET("/", publicHandler.Home)
	router.GET("/contact", publicHandler.Contact)
	router.GET("/contact/", publicHandler.Contact)

	// Parish registration — explicit routes shadow the CMS catch-all.
	router.GET("/about/parish-registration", registrationHandler.FormPage)
	router.GET("/about/parish-registration/", registrationHandler.FormPage)

	// Parish calendar — reads from our Events table, editable via /admin/events.
	router.GET("/events-news/calendar", calendarHandler.Calendar)
	router.GET("/events-news/calendar/", calendarHandler.Calendar)

	// Mass + confession pages are their own authoritative source.
	router.GET("/liturgy-and-sacraments/mass-times", scheduleHandler.MassTimesPage)
	router.GET("/liturgy-and-sacraments/mass-times/", scheduleHandler.MassTimesPage)
	router.GET("/liturgy-and-sacraments/confession-times", scheduleHandler.ConfessionTimesPage)
	router.GET("/liturgy-and-sacraments/confession-times/", scheduleHandler.ConfessionTimesPage)

	// Admin login page is public so unauth visitors reach it from the
	// AdminPageMiddleware redirect.
	router.GET("/admin/login", publicHandler.AdminLogin)

	// ---------- Redirects (preserve current-site typos) ----------
	// `/ministries/*any` → `/minstries/*any` (current site uses the typo).
	router.GET("/ministries", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/minstries/")
	})
	router.GET("/ministries/*rest", func(c *gin.Context) {
		rest := strings.TrimPrefix(c.Param("rest"), "/")
		target := "/minstries/"
		if rest != "" {
			target += rest
		}
		c.Redirect(http.StatusMovedPermanently, target)
	})
	// `/perpetual-adoration/` → `/liturgy-and-sacraments/perpetural-adoration/`.
	router.GET("/perpetual-adoration", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/liturgy-and-sacraments/perpetural-adoration/")
	})
	router.GET("/perpetual-adoration/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/liturgy-and-sacraments/perpetural-adoration/")
	})

	// ---------- Database-dependent routes ----------
	if db != nil {
		// Public API (contact form)
		messageHandler := handlers.NewMessageHandler(db)
		api := router.Group("/api/v1")
		{
			auth := api.Group("/auth")
			{
				authHandler := handlers.NewAuthHandler(db, cfg)
				auth.POST("/login", authHandler.Login)
				auth.POST("/logout", authHandler.Logout)
			}
			api.POST("/contact", middleware.OptionalAuthMiddleware(cfg.JWT, db), messageHandler.Submit)
			api.POST("/parish-registration", registrationHandler.Submit)
		}

		// Admin HTML
		adminHandler := handlers.NewAdminHandler(db)
		adminPagesHandler := handlers.NewAdminPagesHandler(db)
		adminAssetsHandler := handlers.NewAdminAssetsHandler(db)
		adminStructured := handlers.NewAdminStructuredHandler(db)

		adminPages := router.Group("/admin")
		adminPages.Use(middleware.OptionalAuthMiddleware(cfg.JWT, db), middleware.AdminPageMiddleware())
		{
			adminPages.GET("", adminHandler.Dashboard)
			adminPages.GET("/messages", adminHandler.Messages)

			adminPages.GET("/pages", adminPagesHandler.List)
			adminPages.GET("/pages/*slug", adminPagesHandler.Edit)

			adminPages.GET("/homilies", adminStructured.HomiliesPage)
			adminPages.GET("/bulletins", adminStructured.BulletinsPage)
			adminPages.GET("/events", adminStructured.EventsPage)
			adminPages.GET("/mass-times", adminStructured.MassTimesPage)
			adminPages.GET("/confessions", adminStructured.ConfessionsPage)
			adminPages.GET("/registrations", adminStructured.RegistrationsPage)
			adminPages.GET("/assets", adminAssetsHandler.LibraryPage)
		}

		// Admin API. OptionalAuthMiddleware reads either the Bearer header or
		// the auth_token cookie, so the admin UI (cookie-based) and any
		// external script (Bearer-based) both work. AdminMiddleware then
		// gates on role.
		authHandler := handlers.NewAuthHandler(db, cfg)
		userHandler := handlers.NewUserHandler(db, cfg)
		adminAPI := router.Group("/api/v1/admin")
		adminAPI.Use(middleware.OptionalAuthMiddleware(cfg.JWT, db), middleware.AdminMiddleware())
		{
			adminAPI.POST("/auth/register", authHandler.Register)
			adminAPI.POST("/auth/refresh", authHandler.RefreshToken)

			users := adminAPI.Group("/users")
			{
				users.GET("", userHandler.GetAllUsers)
				users.PUT("/:id/status", userHandler.UpdateUserStatus)
				users.PUT("/:id/role", userHandler.UpdateUserRole)
			}

			messages := adminAPI.Group("/messages")
			{
				messages.PUT("/:id/read", adminHandler.MarkMessageRead)
				messages.DELETE("/:id", adminHandler.DeleteMessage)
			}

			// Pages CRUD. Save uses its own prefix because /pages/*slug would
			// otherwise shadow /pages/:id/publish in Gin's route tree.
			adminAPI.POST("/pages", adminPagesHandler.APICreate)
			adminAPI.PUT("/page-content/*slug", adminPagesHandler.APISave)
			adminAPI.DELETE("/pages/:id", adminPagesHandler.APIDelete)
			adminAPI.PUT("/pages/:id/publish", adminPagesHandler.APIPublish)

			// Assets
			adminAPI.GET("/assets", adminAssetsHandler.APIList)
			adminAPI.POST("/assets", adminAssetsHandler.APIUpload)
			adminAPI.DELETE("/assets/:id", adminAssetsHandler.APIDelete)

			// Mass schedules
			adminAPI.POST("/mass-schedules", adminStructured.MassTimeAPICreate)
			adminAPI.PUT("/mass-schedules/:id", adminStructured.MassTimeAPIUpdate)
			adminAPI.DELETE("/mass-schedules/:id", adminStructured.MassTimeAPIDelete)
			adminAPI.POST("/mass-schedules/reorder", adminStructured.MassTimesReorder)

			// Confession times
			adminAPI.POST("/confession-times", adminStructured.ConfessionAPICreate)
			adminAPI.PUT("/confession-times/:id", adminStructured.ConfessionAPIUpdate)
			adminAPI.DELETE("/confession-times/:id", adminStructured.ConfessionAPIDelete)

			// Homilies
			adminAPI.POST("/homilies", adminStructured.HomilyAPICreate)
			adminAPI.PUT("/homilies/:id", adminStructured.HomilyAPIUpdate)
			adminAPI.DELETE("/homilies/:id", adminStructured.HomilyAPIDelete)

			// Bulletins
			adminAPI.POST("/bulletins", adminStructured.BulletinAPICreate)
			adminAPI.DELETE("/bulletins/:id", adminStructured.BulletinAPIDelete)

			// Events
			adminAPI.POST("/events", adminStructured.EventAPICreate)
			adminAPI.PUT("/events/:id", adminStructured.EventAPIUpdate)
			adminAPI.DELETE("/events/:id", adminStructured.EventAPIDelete)

			// Parish registrations
			adminAPI.PUT("/registrations/:id/read", adminStructured.RegistrationMarkRead)
			adminAPI.DELETE("/registrations/:id", adminStructured.RegistrationDelete)
		}
	}

	// ---------- CMS catch-all (any slug that didn't match above) ----------
	pageHandler := handlers.NewPageHandler(db)
	router.NoRoute(pageHandler.Render)
}
