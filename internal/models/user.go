package models

import (
	"time"

	"gorm.io/gorm"
)

// Roles a user can have. Admins can open the admin dashboard; promote the
// first one by setting users.role to "admin" in the database.
const (
	RoleStaff = "staff"
	RoleAdmin = "admin"
)

// User is a parish staff account. The public site has no sign-up; accounts are
// created from the admin dashboard. The first admin is promoted manually by
// setting users.role to 'admin' directly in the database.
type User struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	Email     string `json:"email" gorm:"uniqueIndex;not null"`
	Password  string `json:"-" gorm:"not null"`
	FirstName string `json:"first_name" gorm:"not null"`
	LastName  string `json:"last_name" gorm:"not null"`
	Phone     string `json:"phone"`
	IsActive  bool   `json:"is_active" gorm:"default:true"`
	Role      string `json:"role" gorm:"not null;default:'staff'"`
	// LastLoginAt is set on each successful sign-in; nil means never.
	LastLoginAt *time.Time     `json:"last_login_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsStaff reports whether the user can use the admin panel at all. Staff can
// edit all parish content; only admins can manage user accounts.
func (u User) IsStaff() bool {
	return u.Role == RoleStaff || u.Role == RoleAdmin
}

// LastLoginLabel describes LastLoginAt in parish time for the Users page.
func (u User) LastLoginLabel() string {
	if u.LastLoginAt == nil {
		return "Never"
	}
	return u.LastLoginAt.In(ParishZone).Format("Jan 2, 2006 3:04 PM")
}

func (u User) DisplayName() string {
	name := u.FirstName
	if u.LastName != "" {
		if name != "" {
			name += " "
		}
		name += u.LastName
	}
	if name == "" {
		return u.Email
	}
	return name
}
