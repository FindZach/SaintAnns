package models

import (
	"time"

	"gorm.io/gorm"
)

// ContactMessage is a message sent from the Contact page. Parish staff read
// and answer them from the admin dashboard.
type ContactMessage struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    *uint          `json:"user_id" gorm:"index"` // set when the sender was signed in (e.g. a staff member testing)
	FirstName string         `json:"first_name" gorm:"not null"`
	LastName  string         `json:"last_name" gorm:"not null"`
	Email     string         `json:"email" gorm:"not null"`
	Phone     string         `json:"phone"`
	Subject   string         `json:"subject" gorm:"not null"` // label the message was sent under, e.g. "Mass Intention Request"
	Message   string         `json:"message" gorm:"type:text;not null"`
	ReadAt    *time.Time     `json:"read_at"` // nil until a staff member reads it
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type ContactSubject struct {
	Value string
	Label string
}

// ContactSubjects is the Contact page's subject list. The page renders its
// options from here, and the server only accepts these values.
var ContactSubjects = []ContactSubject{
	{"general", "General Inquiry"},
	{"mass-intention", "Mass Intention Request"},
	{"sacrament", "Sacrament Preparation"},
	{"registration", "Parish Registration"},
	{"bulletin", "Bulletin Submission"},
	{"ministry", "Ministry Volunteer"},
	{"pastoral", "Pastoral Care Request"},
	{"other", "Other"},
}

func ContactSubjectLabel(value string) (string, bool) {
	for _, subject := range ContactSubjects {
		if subject.Value == value {
			return subject.Label, true
		}
	}
	return "", false
}
