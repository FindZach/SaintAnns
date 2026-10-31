package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"olv-parish/internal/auth"
	"olv-parish/internal/config"
	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const minPasswordLen = 10

// UsersPage renders /admin/users (admins only).
func (h *UserHandler) UsersPage(c *gin.Context) {
	user, _ := c.Get("user")
	var unread int64
	h.db.Model(&models.ContactMessage{}).Where("read_at IS NULL").Count(&unread)
	var users []models.User
	h.db.Order("is_active DESC, role ASC, last_name ASC, first_name ASC").Find(&users)
	c.HTML(http.StatusOK, "admin-users.html", gin.H{
		"title":          "Users — Admin — OLV Parish",
		"page":           "admin-users",
		"user":           user,
		"unreadMessages": unread,
		"version":        assetVersion,
		"users":          users,
		"currentUserID":  c.GetUint("user_id"),
	})
}

// CreateUser adds an account with a temporary password the admin passes on.
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name"`
		Password  string `json:"password" binding:"required"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, first name and a temporary password are required"})
		return
	}
	if len(req.Password) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 10 characters"})
		return
	}
	if req.Role != models.RoleAdmin {
		req.Role = models.RoleStaff
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var existing models.User
	if err := h.db.Unscoped().Where("LOWER(email) = ?", email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An account with that email already exists"})
		return
	}
	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	u := models.User{
		Email: email, Password: hashed, Role: req.Role, IsActive: true,
		FirstName: strings.TrimSpace(req.FirstName), LastName: strings.TrimSpace(req.LastName),
	}
	if err := h.db.Create(&u).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Account created", "id": u.ID})
}

// SetPassword lets an admin set a new temporary password for another account.
func (h *UserHandler) SetPassword(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Password) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 10 characters"})
		return
	}
	hashed, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set password"})
		return
	}
	result := h.db.Model(&models.User{}).Where("id = ?", id).Update("password", hashed)
	if result.Error != nil || result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}

// ChangeOwnPassword lets any signed-in staff member change their password.
func (h *UserHandler) ChangeOwnPassword(c *gin.Context) {
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if len(req.New) < minPasswordLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password must be at least 10 characters"})
		return
	}
	var u models.User
	if err := h.db.First(&u, c.GetUint("user_id")).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please sign in again"})
		return
	}
	if !auth.CheckPasswordHash(req.Current, u.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}
	hashed, err := auth.HashPassword(req.New)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}
	if err := h.db.Model(&u).Update("password", hashed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password changed"})
}

// UserHandler holds admin-only user management endpoints. The public site has
// no user-facing account features in Phase 1.
type UserHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewUserHandler(db *gorm.DB, cfg *config.Config) *UserHandler {
	return &UserHandler{db: db, cfg: cfg}
}

func (h *UserHandler) GetAllUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	for i := range users {
		users[i].Password = ""
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if uint(id) == c.GetUint("user_id") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can't change the status of your own account"})
		return
	}
	if err := h.db.Model(&models.User{}).Where("id = ?", id).Update("is_active", req.IsActive).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User status updated"})
}

func (h *UserHandler) UpdateUserRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Role != models.RoleAdmin && req.Role != models.RoleStaff {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role must be admin or staff"})
		return
	}
	if uint(id) == c.GetUint("user_id") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You can't change your own role"})
		return
	}
	result := h.db.Model(&models.User{}).Where("id = ?", id).Update("role", req.Role)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User role updated"})
}
