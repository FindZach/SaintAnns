package models

import (
	"time"

	"gorm.io/gorm"
)

// NavItem is one entry in the main nav tree. Items with ParentID != nil are
// children shown under the parent dropdown. External=true opens in a new tab.
type NavItem struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	ParentID  *uint          `json:"parent_id" gorm:"index"`
	Sort      int            `json:"sort" gorm:"not null;default:0"`
	Label     string         `json:"label" gorm:"not null"`
	Href      string         `json:"href" gorm:"not null"`
	External  bool           `json:"external" gorm:"default:false"`
	Enabled   bool           `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (NavItem) TableName() string { return "nav_items" }
