package models

import (
	"time"

	"gorm.io/gorm"
)

// Bulletin is a weekly parish bulletin PDF uploaded via the admin dashboard.
// The actual file lives as an Asset row (AssetID); Label and PublishedOn are
// what shows on the public bulletin list.
type Bulletin struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	PublishedOn time.Time      `json:"published_on" gorm:"not null;index"`
	Label       string         `json:"label" gorm:"not null"`
	AssetID     uint           `json:"asset_id" gorm:"not null;index"`
	Asset       *Asset         `json:"asset" gorm:"foreignKey:AssetID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Bulletin) TableName() string { return "bulletins" }

// Homily is a recorded weekend homily. Body is the written text (plain or
// markdown); AudioURL / VideoURL are optional external links.
type Homily struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	PreachedOn time.Time      `json:"preached_on" gorm:"not null;index"`
	Title      string         `json:"title" gorm:"not null"`
	Author     string         `json:"author"`
	Body       string         `json:"body" gorm:"type:text"`
	AudioURL   string         `json:"audio_url"`
	VideoURL   string         `json:"video_url"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Homily) TableName() string { return "homilies" }
