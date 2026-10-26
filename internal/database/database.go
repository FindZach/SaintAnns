package database

import (
	"fmt"
	"log"

	"olv-parish/internal/config"
	"olv-parish/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Initialize(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	log.Println("Database connection established successfully")
	return db, nil
}

// Migrate runs the full schema. Call it via `--migrate` on first deploy and
// after model changes. AutoMigrate only adds; it never drops data.
func Migrate(db *gorm.DB) error {
	log.Println("Running database migrations...")
	all := []interface{}{
		&models.User{},
		&models.ContactMessage{},
	}
	all = append(all, contentModels()...)
	if err := db.AutoMigrate(all...); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	log.Println("Database migrations completed successfully")
	return nil
}

// EnsureAdminSchema is the additive migration path that runs on every startup.
// It adds columns and tables the current build needs so production can roll
// forward without a manual migration step. AutoMigrate only adds.
func EnsureAdminSchema(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&models.User{}, "Role") {
		if err := db.Migrator().AddColumn(&models.User{}, "Role"); err != nil {
			return fmt.Errorf("failed to add users.role: %w", err)
		}
	}
	if err := db.AutoMigrate(&models.ContactMessage{}); err != nil {
		return fmt.Errorf("failed to create contact messages table: %w", err)
	}
	return nil
}
