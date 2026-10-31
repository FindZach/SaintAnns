package models

import (
	"time"

	"gorm.io/gorm"
)

// Event is a parish calendar entry. Body is a short description (longer prose
// should live on a dedicated Page). Featured = true to pin it on the home page.
type Event struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Slug         string         `json:"slug" gorm:"uniqueIndex;not null"`
	Title        string         `json:"title" gorm:"not null"`
	Location     string         `json:"location"`
	StartsAt     time.Time      `json:"starts_at" gorm:"not null;index"`
	EndsAt       *time.Time     `json:"ends_at"`
	Body         string         `json:"body" gorm:"type:text"`
	FlyerAssetID *uint          `json:"flyer_asset_id"`
	FlyerAsset   *Asset         `json:"flyer_asset" gorm:"foreignKey:FlyerAssetID"`
	Featured     bool           `json:"featured" gorm:"default:false;index"`
	Tag          string         `json:"tag"` // optional, e.g. "faith-formation", so event_list blocks can filter
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Event) TableName() string { return "events" }

// ParishZone is the parish's local time zone. Event times are stored as
// absolute instants and shown in this zone.
var ParishZone = func() *time.Location {
	if z, err := time.LoadLocation("America/New_York"); err == nil {
		return z
	}
	return time.Local
}()

// LocalStart is StartsAt in parish time, for display.
func (e Event) LocalStart() time.Time { return e.StartsAt.In(ParishZone) }

// LocalEnd is EndsAt in parish time, or nil.
func (e Event) LocalEnd() *time.Time {
	if e.EndsAt == nil {
		return nil
	}
	t := e.EndsAt.In(ParishZone)
	return &t
}

// NewsPost is a parish announcement or longer-form news item. Body uses the
// same block system as a Page so staff can lay it out with the drag-and-drop
// editor from Phase 4.
type NewsPost struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Slug        string         `json:"slug" gorm:"uniqueIndex;not null"`
	Title       string         `json:"title" gorm:"not null"`
	Excerpt     string         `json:"excerpt"`
	HeroAssetID *uint          `json:"hero_asset_id"`
	HeroAsset   *Asset         `json:"hero_asset" gorm:"foreignKey:HeroAssetID"`
	Body        Blocks         `json:"body" gorm:"type:jsonb;not null;default:'[]'"`
	PublishedAt *time.Time     `json:"published_at" gorm:"index"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (NewsPost) TableName() string { return "news_posts" }

// IsPublished mirrors Page.IsPublished.
func (n NewsPost) IsPublished() bool {
	return n.PublishedAt != nil && !n.PublishedAt.After(time.Now())
}
