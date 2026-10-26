package middleware

import (
	"net/http"
	"strings"

	"olv-parish/internal/auth"
	"olv-parish/internal/config"
	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthMiddleware hard-fails with JSON 401 on missing/invalid Bearer. Use on APIs.
func AuthMiddleware(cfg config.JWTConfig, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateJWT(tokenString, cfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}
		if !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Account is inactive"})
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Next()
	}
}

// OptionalAuthMiddleware checks Bearer header then `auth_token` cookie,
// sets `user` / `user_id` on success, never aborts.
func OptionalAuthMiddleware(cfg config.JWTConfig, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := ""

		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				tokenString = ""
			}
		}
		if tokenString == "" {
			if cookie, err := c.Cookie("auth_token"); err == nil {
				tokenString = cookie
			}
		}
		if tokenString == "" {
			c.Next()
			return
		}

		claims, err := auth.ValidateJWT(tokenString, cfg)
		if err != nil {
			c.Next()
			return
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			c.Next()
			return
		}
		if user.IsActive {
			c.Set("user", user)
			c.Set("user_id", user.ID)
		}
		c.Next()
	}
}

// AdminMiddleware returns 403 JSON for the admin API when the caller isn't an admin.
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		if !user.(models.User).IsAdmin() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminPageMiddleware guards the admin HTML pages. Unauthenticated visitors
// are sent to /admin/login; signed-in non-admins are sent to the home page.
func AdminPageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}
		if !user.(models.User).IsAdmin() {
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}
		c.Next()
	}
}
