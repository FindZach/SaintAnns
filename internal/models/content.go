package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Block is one entry in a page's ordered body. Type names it (e.g. "heading",
// "rich_text", "image"); Data holds the per-type payload as JSON. Keeping Data
// as a raw JSON blob lets us add new block types without a schema migration —
// the renderer and editor are the ones that interpret it.
type Block struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Blocks is an ordered list of blocks persisted as a single JSONB column.
type Blocks []Block

// Value implements driver.Valuer so GORM can store Blocks as JSONB.
func (b Blocks) Value() (driver.Value, error) {
	if b == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(b)
}

// Scan implements sql.Scanner so GORM can read Blocks from JSONB.
func (b *Blocks) Scan(value interface{}) error {
	if value == nil {
		*b = Blocks{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("blocks: unsupported scan source")
	}
	if len(data) == 0 {
		*b = Blocks{}
		return nil
	}
	return json.Unmarshal(data, b)
}

// GormDataType hints the generic SQL type; the Postgres override below picks
// the right native type on the Postgres driver.
func (Blocks) GormDataType() string { return "jsonb" }

// Known block types the editor knows how to render. Keeping them as constants
// here keeps the Go side and the editor's palette in sync.
const (
	BlockHeading       = "heading"
	BlockRichText      = "rich_text"
	BlockImage         = "image"
	BlockImageRow      = "image_row"
	BlockPDFLink       = "pdf_link"
	BlockVideoEmbed    = "video_embed"
	BlockMassTimesCard = "mass_times_card"
	BlockConfessions   = "confession_times_card"
	BlockEventList     = "event_list"
	BlockBulletinList  = "bulletin_list"
	BlockStaffGrid     = "staff_grid"
	BlockCTAButton     = "cta_button"
	BlockDivider       = "divider"
	BlockHTMLRaw       = "html_raw"
)

// Page is a freeform page composed of ordered blocks. The slug matches the
// public URL (without leading slash — e.g. "about" or
// "liturgy-and-sacraments/sacraments/baptism").
type Page struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	Slug            string         `json:"slug" gorm:"uniqueIndex;not null"`
	ParentSlug      string         `json:"parent_slug" gorm:"index"`
	Title           string         `json:"title" gorm:"not null"`
	NavLabel        string         `json:"nav_label"`
	HeroImageSlot   string         `json:"hero_image_slot"`
	MetaDescription string         `json:"meta_description"`
	Blocks          Blocks         `json:"blocks" gorm:"type:jsonb;not null;default:'[]'"`
	PublishedAt     *time.Time     `json:"published_at"`
	UpdatedByID     *uint          `json:"updated_by_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (Page) TableName() string { return "pages" }

// IsPublished reports whether the page should be served on the public site.
func (p Page) IsPublished() bool {
	return p.PublishedAt != nil && !p.PublishedAt.After(time.Now())
}
