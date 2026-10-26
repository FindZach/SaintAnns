package models

import (
	"time"

	"gorm.io/gorm"
)

// Registration intents.
const (
	RegistrationNew    = "new"
	RegistrationUpdate = "update"
	RegistrationChange = "change"
)

// Preferred communication channels (persisted as a comma-separated string so a
// household can pick more than one without a joining table).
const (
	ContactPrefEmail = "email"
	ContactPrefPhone = "phone"
	ContactPrefMail  = "mail"
)

// ParishRegistration is a submission from the public parish-registration form.
// Staff read and action these from the admin dashboard — the parish office then
// enters the data into whatever system they use for membership tracking.
type ParishRegistration struct {
	ID uint `json:"id" gorm:"primaryKey"`

	// Intent — new registration, info update, or change of registration.
	Intent string `json:"intent" gorm:"not null;default:'new'"`

	// Household head
	HeadFirstName string `json:"head_first_name" gorm:"not null"`
	HeadLastName  string `json:"head_last_name" gorm:"not null"`
	HeadEmail     string `json:"head_email"`
	HeadPhone     string `json:"head_phone"`

	// Spouse (optional)
	SpouseFirstName string `json:"spouse_first_name"`
	SpouseLastName  string `json:"spouse_last_name"`
	SpouseEmail     string `json:"spouse_email"`
	SpousePhone     string `json:"spouse_phone"`

	// Address
	Address1   string `json:"address1" gorm:"not null"`
	Address2   string `json:"address2"`
	City       string `json:"city" gorm:"not null"`
	State      string `json:"state" gorm:"not null;default:'NY'"`
	PostalCode string `json:"postal_code" gorm:"not null"`

	// Other household members — plain-text "Name — relationship — DOB" lines.
	// Phase 7 could promote this to a structured table.
	HouseholdMembers string `json:"household_members" gorm:"type:text"`

	// Preferences
	PrimaryChurch       string `json:"primary_church"` // st-ann | st-mary | either
	WantsEnvelopes      bool   `json:"wants_envelopes"`
	EnvelopeNumber      string `json:"envelope_number"` // if continuing
	ContactPreferences  string `json:"contact_preferences"` // comma-sep: email,phone,mail

	// Free-form notes
	Notes string `json:"notes" gorm:"type:text"`

	// Admin bookkeeping
	ReadAt    *time.Time     `json:"read_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (ParishRegistration) TableName() string { return "parish_registrations" }

// DisplayName is the household head's name as it should show in admin lists.
func (p ParishRegistration) DisplayName() string {
	name := p.HeadFirstName
	if p.HeadLastName != "" {
		if name != "" {
			name += " "
		}
		name += p.HeadLastName
	}
	if p.SpouseFirstName != "" || p.SpouseLastName != "" {
		spouse := p.SpouseFirstName
		if p.SpouseLastName != "" {
			if spouse != "" {
				spouse += " "
			}
			spouse += p.SpouseLastName
		}
		name = name + " & " + spouse
	}
	return name
}

// IntentLabel is a human-readable version of the Intent code.
func (p ParishRegistration) IntentLabel() string {
	switch p.Intent {
	case RegistrationNew:
		return "New registration"
	case RegistrationUpdate:
		return "Information update"
	case RegistrationChange:
		return "Change of registration"
	}
	return p.Intent
}
