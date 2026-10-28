// Command import is a dev-side wrapper around internal/importer. For
// in-container imports on a running deploy, use `./main --import` instead;
// that path reuses the same package and the server's env vars.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"olv-parish/internal/importer"
	"olv-parish/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	var (
		dryRun    = flag.Bool("dry-run", false, "scrape and log; no DB writes or downloads")
		singleURL = flag.String("url", "", "import just this URL (e.g. /about/)")
		limit     = flag.Int("limit", 0, "stop after N pages (0 = all)")
		baseFlag  = flag.String("base", importer.DefaultBaseURL, "live site base URL")
		verbose   = flag.Bool("verbose", false, "chattier logs")
	)
	flag.Parse()

	db, err := openDB()
	if err != nil {
		log.Fatalf("DB: %v", err)
	}
	opts := importer.Options{
		DryRun:    *dryRun,
		SingleURL: *singleURL,
		Limit:     *limit,
		BaseURL:   *baseFlag,
		Verbose:   *verbose,
	}
	if err := importer.Run(db, opts); err != nil {
		log.Fatalf("import: %v", err)
	}
}

func openDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		envOr("DB_PASSWORD", ""),
		envOr("DB_NAME", "olv_parish"),
		envOr("DB_SSLMODE", "disable"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("gorm.Open: %w", err)
	}
	if err := db.AutoMigrate(&models.Page{}, &models.Asset{}); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}
	return db, nil
}

func envOr(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}
