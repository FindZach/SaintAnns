package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"olv-parish/internal/auth"
	"olv-parish/internal/config"
	"olv-parish/internal/database"
	"olv-parish/internal/handlers"
	"olv-parish/internal/importer"
	"olv-parish/internal/middleware"
	"olv-parish/internal/models"
	"olv-parish/internal/routes"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "Run database migrations and exit")
	forceMigrate := flag.Bool("force-migrate", false, "Force run database migrations (development mode)")
	seedOnly := flag.Bool("seed", false, "Insert baseline parish content (Mass times, nav, starter pages) and exit")
	createAdmin := flag.Bool("create-admin", false, "Upsert the user named by $ADMIN_EMAIL + $ADMIN_PASSWORD as role=admin and exit")
	runImport := flag.Bool("import", false, "Scrape pages and assets from the live olv14843.org site into this DB, then exit")
	importURL := flag.String("import-url", "", "With --import: import just this URL (e.g. /about/) instead of the whole list")
	importLimit := flag.Int("import-limit", 0, "With --import: stop after N pages (0 = all)")
	seedHistory := flag.Bool("seed-history", false, "Upsert the /about/our-history/ parish-history page and exit")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		gin.SetMode(ginMode)
	} else {
		gin.SetMode(cfg.Server.GinMode)
	}

	router := gin.Default()

	log.Println("Initializing database connection...")
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		if *migrateOnly || *forceMigrate {
			log.Printf("Cannot run migrations: Failed to connect to database: %v", err)
			os.Exit(1)
		}
		log.Printf("Failed to connect to database: %v", err)
		log.Println("Application will continue in degraded mode (no admin, no contact form)")
	}

	router.GET("/health", func(c *gin.Context) {
		status := "healthy"
		if db == nil {
			status = "degraded"
		}
		c.JSON(200, gin.H{
			"status":   status,
			"database": db != nil,
			"time":     time.Now().Unix(),
		})
	})

	if *migrateOnly {
		log.Println("Running migrations only...")
		if err := database.Migrate(db); err != nil {
			log.Printf("Failed to run database migrations: %v", err)
			os.Exit(1)
		}
		log.Println("Database migrations completed successfully")
		return
	}

	if *seedOnly {
		if db == nil {
			log.Fatal("Cannot seed: no database connection")
		}
		if err := database.EnsureAdminSchema(db); err != nil {
			log.Fatalf("Cannot seed: ensure admin schema: %v", err)
		}
		if err := database.EnsureContentSchema(db); err != nil {
			log.Fatalf("Cannot seed: ensure content schema: %v", err)
		}
		if err := database.Seed(db); err != nil {
			log.Fatalf("Seed failed: %v", err)
		}
		return
	}

	if *createAdmin {
		if db == nil {
			log.Fatal("Cannot create admin: no database connection")
		}
		if err := database.EnsureAdminSchema(db); err != nil {
			log.Fatalf("Cannot create admin: ensure admin schema: %v", err)
		}
		if err := bootstrapAdmin(db); err != nil {
			log.Fatalf("Create admin failed: %v", err)
		}
		return
	}

	if *runImport {
		if db == nil {
			log.Fatal("Cannot import: no database connection")
		}
		if err := database.EnsureContentSchema(db); err != nil {
			log.Fatalf("Cannot import: ensure content schema: %v", err)
		}
		if err := importer.Run(db, importer.Options{
			SingleURL: *importURL,
			Limit:     *importLimit,
			Verbose:   true,
		}); err != nil {
			log.Fatalf("Import failed: %v", err)
		}
		return
	}

	if *seedHistory {
		if db == nil {
			log.Fatal("Cannot seed history: no database connection")
		}
		if err := database.EnsureContentSchema(db); err != nil {
			log.Fatalf("Cannot seed history: ensure content schema: %v", err)
		}
		if err := database.SeedHistoryPage(db); err != nil {
			log.Fatalf("Seed history failed: %v", err)
		}
		return
	}

	if *forceMigrate {
		log.Println("Force running database migrations...")
		if err := database.Migrate(db); err != nil {
			log.Printf("Failed to run database migrations: %v", err)
			log.Fatal("Cannot continue without database migrations")
		}
		log.Println("Database migrations completed")
	} else if db != nil {
		log.Println("Skipping database migrations (use --force-migrate to run them)")
	}

	// Additive schema that every startup can safely re-apply.
	if db != nil {
		if err := database.EnsureAdminSchema(db); err != nil {
			log.Printf("Warning: %v; the admin dashboard and contact form may not work", err)
		}
		if err := database.EnsureContentSchema(db); err != nil {
			log.Printf("Warning: %v; some content tables may be missing", err)
		}
	}

	middleware.SetupMiddleware(router, cfg)
	routes.SetupRoutes(router, db, cfg)

	// Catholic Courier RSS fetcher. One goroutine, hourly refresh, cancels on
	// SIGINT/SIGTERM. If the DB is nil (degraded mode) the fetcher is a no-op.
	fetcherCtx, cancelFetcher := context.WithCancel(context.Background())
	defer cancelFetcher()
	if db != nil {
		courierURL := os.Getenv("COURIER_FEED_URL")
		fetcher := handlers.NewCourierFetcher(db, courierURL)
		go fetcher.Start(fetcherCtx)
	}

	// Clean shutdown on Ctrl-C / SIGTERM so the fetcher stops promptly.
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopCh
		cancelFetcher()
	}()

	log.Printf("Starting OLV Parish server on port %s", cfg.Server.Port)
	if err := router.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// bootstrapAdmin upserts a staff account with role=admin from env vars
// $ADMIN_EMAIL and $ADMIN_PASSWORD. Useful for first deploy where there's no
// existing admin to invite others. If the user exists, this resets their
// password and ensures role=admin + is_active=true. Password must be ≥ 8 chars.
func bootstrapAdmin(db *gorm.DB) error {
	email := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	first := strings.TrimSpace(os.Getenv("ADMIN_FIRST_NAME"))
	last := strings.TrimSpace(os.Getenv("ADMIN_LAST_NAME"))

	if email == "" || password == "" {
		log.Fatal("--create-admin needs ADMIN_EMAIL and ADMIN_PASSWORD env vars")
	}
	if len(password) < 8 {
		log.Fatal("ADMIN_PASSWORD must be at least 8 characters")
	}
	if first == "" {
		first = "Parish"
	}
	if last == "" {
		last = "Admin"
	}

	hashed, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	var user models.User
	err = db.Where("email = ?", email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		user = models.User{
			Email:     email,
			Password:  hashed,
			FirstName: first,
			LastName:  last,
			IsActive:  true,
			Role:      models.RoleAdmin,
		}
		if err := db.Create(&user).Error; err != nil {
			return err
		}
		log.Printf("Created admin user %s (id=%d). Password set.", email, user.ID)
		return nil
	}
	if err != nil {
		return err
	}
	// Existing user — update password and promote to admin.
	user.Password = hashed
	user.Role = models.RoleAdmin
	user.IsActive = true
	if first != "Parish" {
		user.FirstName = first
	}
	if last != "Admin" {
		user.LastName = last
	}
	if err := db.Save(&user).Error; err != nil {
		return err
	}
	log.Printf("Updated existing user %s (id=%d) → role=admin, password reset", email, user.ID)
	return nil
}
