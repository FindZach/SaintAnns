package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const minSecretLength = 32

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Session  SessionConfig
	Email    EmailConfig
	Frontend FrontendConfig
}

type ServerConfig struct {
	Port    string
	GinMode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	Secret string
}

type SessionConfig struct {
	Secret string
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
}

type FrontendConfig struct {
	URL string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			GinMode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", ""),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "olv_parish"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		Session: SessionConfig{
			Secret: getEnv("SESSION_SECRET", ""),
		},
		Email: EmailConfig{
			SMTPHost:     getEnv("SMTP_HOST", ""),
			SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromAddress:  getEnv("EMAIL_FROM", ""),
		},
		Frontend: FrontendConfig{
			URL: getEnv("FRONTEND_URL", "http://localhost:8080"),
		},
	}
}

// Validate reports every required setting that is missing or unsafe, so a
// misconfigured deployment fails at startup instead of running on weak defaults.
func (c *Config) Validate() error {
	var problems []string

	if c.Database.Host == "" {
		problems = append(problems, "DB_HOST is required")
	}
	if c.Database.Password == "" {
		problems = append(problems, "DB_PASSWORD is required")
	}
	if p := checkSecret("JWT_SECRET", c.JWT.Secret); p != "" {
		problems = append(problems, p)
	}
	if p := checkSecret("SESSION_SECRET", c.Session.Secret); p != "" {
		problems = append(problems, p)
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func checkSecret(name, value string) string {
	switch {
	case value == "":
		return name + " is required"
	case len(value) < minSecretLength:
		return fmt.Sprintf("%s must be at least %d characters", name, minSecretLength)
	case strings.HasPrefix(value, "your-"):
		return name + " is still the placeholder from env.example"
	}
	return ""
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
