package models

import (
	"time"

	"gorm.io/gorm"
)

// Church identifies which of the parish's two church buildings a schedule
// entry belongs to.
const (
	ChurchStAnn   = "st-ann"   // St. Ann, Hornell
	ChurchStMary  = "st-mary"  // St. Mary, Rexville
	ChurchEither  = "either"   // either / rotating
	ChurchParish  = "parish"   // parish-wide (e.g. combined)
)

// Season tags a schedule row so entries can show up only during Advent or Lent.
const (
	SeasonAll     = ""        // applies year-round (default)
	SeasonAdvent  = "advent"
	SeasonLent    = "lent"
	SeasonOrdinary = "ordinary" // explicitly outside Advent/Lent
)

// MassSchedule is one recurring Mass. DayOfWeek follows time.Weekday
// (0 = Sunday … 6 = Saturday). TimeOfDay is "15:04" (24-hour).
type MassSchedule struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Church      string         `json:"church" gorm:"not null;index"`
	DayOfWeek   int            `json:"day_of_week" gorm:"not null"`
	TimeOfDay   string         `json:"time_of_day" gorm:"not null"` // "HH:MM"
	Label       string         `json:"label"`                       // e.g. "Vigil", "Children's Liturgy of the Word"
	Notes       string         `json:"notes"`
	StreamedURL string         `json:"streamed_url"` // e.g. Facebook page for live stream
	Sort        int            `json:"sort" gorm:"not null;default:0"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MassSchedule) TableName() string { return "mass_schedules" }

// ConfessionTime is one recurring confession slot. StartOfDay/EndOfDay use the
// same "15:04" string format as MassSchedule.
type ConfessionTime struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	Church     string         `json:"church" gorm:"not null;index"`
	DayOfWeek  int            `json:"day_of_week" gorm:"not null"`
	Label      string         `json:"label"` // e.g. "Saturday afternoon"
	StartOfDay string         `json:"start_of_day" gorm:"not null"`
	EndOfDay   string         `json:"end_of_day" gorm:"not null"`
	Season     string         `json:"season"` // SeasonAll | SeasonAdvent | SeasonLent | SeasonOrdinary
	Notes      string         `json:"notes"`
	Sort       int            `json:"sort" gorm:"not null;default:0"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ConfessionTime) TableName() string { return "confession_times" }
