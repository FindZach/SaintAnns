package models

import (
	"time"

	"gorm.io/gorm"
)

// CourierHeadline is a cached headline from catholiccourier.com. A background
// job refreshes the table hourly (see Phase 3 wiring). Keeping them in the DB
// instead of just an in-memory cache means the home page still shows the last
// known headlines right after a restart.
type CourierHeadline struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	URL         string         `json:"url" gorm:"uniqueIndex;not null"`
	Title       string         `json:"title" gorm:"not null"`
	Byline      string         `json:"byline"`
	Category    string         `json:"category"`
	PublishedAt time.Time      `json:"published_at" gorm:"index"`
	FetchedAt   time.Time      `json:"fetched_at" gorm:"not null"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (CourierHeadline) TableName() string { return "courier_headlines" }
