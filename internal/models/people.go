package models

import (
	"time"

	"gorm.io/gorm"
)

// Staff is one entry on the parish Staff page (clergy, deacons, directors,
// administrators). Not the same as a User account; a Staff row can exist for
// someone who never signs into the site.
type Staff struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Sort         int            `json:"sort" gorm:"not null;default:0;index"`
	Name         string         `json:"name" gorm:"not null"`
	Role         string         `json:"role" gorm:"not null"` // e.g. "Pastor", "Director of Faith Formation"
	Email        string         `json:"email"`
	Phone        string         `json:"phone"`
	PhotoAssetID *uint          `json:"photo_asset_id"`
	PhotoAsset   *Asset         `json:"photo_asset" gorm:"foreignKey:PhotoAssetID"`
	Bio          string         `json:"bio" gorm:"type:text"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Staff) TableName() string { return "staff" }

// Ministry is a parish ministry (Lector, Usher, Bereavement, Children's
// Liturgy of the Word …). Pages for ministries live in the pages table under
// slug "minstries/<slug>"; this table is for the structured bits (lead,
// contact email, parent ministry).
type Ministry struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Slug         string         `json:"slug" gorm:"uniqueIndex;not null"`
	ParentSlug   string         `json:"parent_slug" gorm:"index"` // e.g. "bereavement" for Grief Support
	Name         string         `json:"name" gorm:"not null"`
	LeadName     string         `json:"lead_name"`
	ContactEmail string         `json:"contact_email"`
	ContactPhone string         `json:"contact_phone"`
	Blurb        string         `json:"blurb" gorm:"type:text"` // short description for the ministries index
	Sort         int            `json:"sort" gorm:"not null;default:0"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Ministry) TableName() string { return "ministries" }
