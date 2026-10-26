package models

import (
	"time"

	"gorm.io/gorm"
)

// Asset kinds. The admin asset picker filters by these.
const (
	AssetImage = "image"
	AssetPDF   = "pdf"
	AssetAudio = "audio"
	AssetVideo = "video"
)

// Asset is a file uploaded via the admin dashboard — image, PDF, audio or
// video. The file itself is stored under web/static/uploads/, keyed by Path.
type Asset struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Kind         string         `json:"kind" gorm:"not null;index"`
	Path         string         `json:"path" gorm:"not null"`         // relative to web/static, e.g. "uploads/2026/10/bulletin.pdf"
	OriginalName string         `json:"original_name"`
	MIMEType     string         `json:"mime_type"`
	ByteSize     int64          `json:"byte_size"`
	Width        int            `json:"width"`  // images only
	Height       int            `json:"height"` // images only
	AltText      string         `json:"alt_text"`
	UploadedBy   *uint          `json:"uploaded_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Asset) TableName() string { return "assets" }

// URL returns the public URL the asset is served at.
func (a Asset) URL() string {
	if a.Path == "" {
		return ""
	}
	return "/static/" + a.Path
}

// HeroSlide is one slide in the home page hero carousel. The drag-reorder in
// admin writes to Sort.
type HeroSlide struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Sort          int            `json:"sort" gorm:"not null;default:0;index"`
	ImageAssetID  uint           `json:"image_asset_id" gorm:"not null"`
	ImageAsset    *Asset         `json:"image_asset" gorm:"foreignKey:ImageAssetID"`
	Headline      string         `json:"headline"`
	Subhead       string         `json:"subhead"`
	CTALabel      string         `json:"cta_label"`
	CTAHref       string         `json:"cta_href"`
	PublishedAt   *time.Time     `json:"published_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (HeroSlide) TableName() string { return "hero_slides" }

// Album is a photo gallery (e.g. "Confirmation 2026"). The cover is one asset
// pulled from the album's photos.
type Album struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	Slug          string         `json:"slug" gorm:"uniqueIndex;not null"`
	Title         string         `json:"title" gorm:"not null"`
	Description   string         `json:"description" gorm:"type:text"`
	CoverAssetID  *uint          `json:"cover_asset_id"`
	CoverAsset    *Asset         `json:"cover_asset" gorm:"foreignKey:CoverAssetID"`
	PublishedAt   *time.Time     `json:"published_at" gorm:"index"`
	Photos        []AlbumPhoto   `json:"photos" gorm:"foreignKey:AlbumID"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Album) TableName() string { return "albums" }

// AlbumPhoto joins an Album to its Assets with order and optional caption.
type AlbumPhoto struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	AlbumID   uint           `json:"album_id" gorm:"not null;index"`
	AssetID   uint           `json:"asset_id" gorm:"not null;index"`
	Asset     *Asset         `json:"asset" gorm:"foreignKey:AssetID"`
	Caption   string         `json:"caption"`
	Sort      int            `json:"sort" gorm:"not null;default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (AlbumPhoto) TableName() string { return "album_photos" }
