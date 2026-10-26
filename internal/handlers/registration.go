package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"olv-parish/internal/config"
	"olv-parish/internal/email"
	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxRegFieldLength = 254
	maxRegNotesLength = 5000
)

// RegistrationHandler owns the public parish-registration form and its submit
// endpoint. Submissions land in the DB and (if SMTP is configured) also email
// the parish office.
type RegistrationHandler struct {
	db     *gorm.DB
	mailer *email.Mailer
}

func NewRegistrationHandler(db *gorm.DB, cfg *config.Config) *RegistrationHandler {
	return &RegistrationHandler{
		db:     db,
		mailer: email.New(cfg.Email),
	}
}

// FormPage renders /about/parish-registration/. This handler is registered
// explicitly in routes.go so it wins against the CMS catch-all.
func (h *RegistrationHandler) FormPage(c *gin.Context) {
	c.HTML(http.StatusOK, "parish-registration.html", gin.H{
		"title":   "Parish Registration — Our Lady of the Valley Parish",
		"page":    "parish-registration",
		"version": assetVersion,
	})
}

type registrationRequest struct {
	Intent string `json:"intent"`

	HeadFirstName string `json:"head_first_name"`
	HeadLastName  string `json:"head_last_name"`
	HeadEmail     string `json:"head_email"`
	HeadPhone     string `json:"head_phone"`

	SpouseFirstName string `json:"spouse_first_name"`
	SpouseLastName  string `json:"spouse_last_name"`
	SpouseEmail     string `json:"spouse_email"`
	SpousePhone     string `json:"spouse_phone"`

	Address1   string `json:"address1"`
	Address2   string `json:"address2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`

	HouseholdMembers string `json:"household_members"`

	PrimaryChurch      string   `json:"primary_church"`
	WantsEnvelopes     bool     `json:"wants_envelopes"`
	EnvelopeNumber     string   `json:"envelope_number"`
	ContactPreferences []string `json:"contact_preferences"`

	Notes string `json:"notes"`

	Website string `json:"website"` // honeypot
}

// Submit validates and persists a registration submission.
func (h *RegistrationHandler) Submit(c *gin.Context) {
	var req registrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please check the form and try again."})
		return
	}

	if req.Website != "" {
		// Honeypot tripped. Pretend it worked.
		c.JSON(http.StatusCreated, gin.H{"message": "Registration received"})
		return
	}

	intent := strings.TrimSpace(req.Intent)
	switch intent {
	case models.RegistrationNew, models.RegistrationUpdate, models.RegistrationChange:
	case "":
		intent = models.RegistrationNew
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please pick a valid request type."})
		return
	}

	reg := models.ParishRegistration{
		Intent:          intent,
		HeadFirstName:   strings.TrimSpace(req.HeadFirstName),
		HeadLastName:    strings.TrimSpace(req.HeadLastName),
		HeadEmail:       strings.TrimSpace(req.HeadEmail),
		HeadPhone:       strings.TrimSpace(req.HeadPhone),
		SpouseFirstName: strings.TrimSpace(req.SpouseFirstName),
		SpouseLastName:  strings.TrimSpace(req.SpouseLastName),
		SpouseEmail:     strings.TrimSpace(req.SpouseEmail),
		SpousePhone:     strings.TrimSpace(req.SpousePhone),
		Address1:        strings.TrimSpace(req.Address1),
		Address2:        strings.TrimSpace(req.Address2),
		City:            strings.TrimSpace(req.City),
		State:           orDefault(strings.TrimSpace(req.State), "NY"),
		PostalCode:      strings.TrimSpace(req.PostalCode),
		HouseholdMembers: strings.TrimSpace(req.HouseholdMembers),
		PrimaryChurch:   strings.TrimSpace(req.PrimaryChurch),
		WantsEnvelopes:  req.WantsEnvelopes,
		EnvelopeNumber:  strings.TrimSpace(req.EnvelopeNumber),
		Notes:           strings.TrimSpace(req.Notes),
	}
	if len(req.ContactPreferences) > 0 {
		prefs := make([]string, 0, len(req.ContactPreferences))
		for _, p := range req.ContactPreferences {
			switch p {
			case models.ContactPrefEmail, models.ContactPrefPhone, models.ContactPrefMail:
				prefs = append(prefs, p)
			}
		}
		reg.ContactPreferences = strings.Join(prefs, ",")
	}

	if problem := validateRegistration(reg); problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}

	if err := h.db.Create(&reg).Error; err != nil {
		log.Printf("Failed to save parish registration: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "We couldn't save your registration. Please try again."})
		return
	}

	// Fire-and-forget notification. We never fail the submission over email.
	go h.notifyOffice(reg)

	c.JSON(http.StatusCreated, gin.H{"message": "Registration received"})
}

func validateRegistration(r models.ParishRegistration) string {
	switch {
	case r.HeadFirstName == "" || r.HeadLastName == "":
		return "Please enter the head of household's name."
	case r.HeadEmail == "" && r.HeadPhone == "":
		return "Please give us an email or phone number so we can reach you."
	case r.Address1 == "" || r.City == "" || r.PostalCode == "":
		return "Please enter your street address, city and ZIP."
	case r.HeadEmail != "" && !validEmail(r.HeadEmail):
		return "Please enter a valid email address."
	case r.SpouseEmail != "" && !validEmail(r.SpouseEmail):
		return "Please enter a valid email for the spouse, or leave it blank."
	case len(r.HeadFirstName) > maxRegFieldLength || len(r.HeadLastName) > maxRegFieldLength ||
		len(r.HeadEmail) > maxRegFieldLength || len(r.HeadPhone) > maxRegFieldLength ||
		len(r.Address1) > maxRegFieldLength || len(r.City) > maxRegFieldLength ||
		len(r.PostalCode) > maxRegFieldLength:
		return "Some of your details are too long. Please shorten them and try again."
	case utf8.RuneCountInString(r.Notes) > maxRegNotesLength:
		return "Please keep notes under 5,000 characters."
	}
	return ""
}

func (h *RegistrationHandler) notifyOffice(r models.ParishRegistration) {
	if h.mailer == nil {
		return
	}
	to := strings.TrimSpace(os.Getenv("PARISH_NOTIFY_EMAIL"))
	if to == "" {
		to = strings.TrimSpace(os.Getenv("EMAIL_FROM"))
	}
	if to == "" {
		// Fall back to the siteDetails email so dev-mode logs still show a
		// reasonable recipient.
		to = siteInfo.Email
	}

	subject := fmt.Sprintf("New parish registration — %s", r.DisplayName())
	body := formatRegistrationEmail(r)
	if err := h.mailer.Send(to, subject, body); err != nil {
		log.Printf("Mailer: failed to notify %s: %v", to, err)
	}
}

func formatRegistrationEmail(r models.ParishRegistration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A new parish registration was submitted on %s.\n\n", time.Now().Format("Mon Jan 2, 2006 3:04 PM"))
	fmt.Fprintf(&b, "Type: %s\n\n", r.IntentLabel())
	fmt.Fprintf(&b, "Household head:\n  %s %s\n", r.HeadFirstName, r.HeadLastName)
	if r.HeadEmail != "" {
		fmt.Fprintf(&b, "  Email: %s\n", r.HeadEmail)
	}
	if r.HeadPhone != "" {
		fmt.Fprintf(&b, "  Phone: %s\n", r.HeadPhone)
	}
	if r.SpouseFirstName != "" || r.SpouseLastName != "" {
		fmt.Fprintf(&b, "\nSpouse:\n  %s %s\n", r.SpouseFirstName, r.SpouseLastName)
		if r.SpouseEmail != "" {
			fmt.Fprintf(&b, "  Email: %s\n", r.SpouseEmail)
		}
		if r.SpousePhone != "" {
			fmt.Fprintf(&b, "  Phone: %s\n", r.SpousePhone)
		}
	}
	fmt.Fprintf(&b, "\nAddress:\n  %s\n", r.Address1)
	if r.Address2 != "" {
		fmt.Fprintf(&b, "  %s\n", r.Address2)
	}
	fmt.Fprintf(&b, "  %s, %s %s\n", r.City, r.State, r.PostalCode)

	if r.HouseholdMembers != "" {
		fmt.Fprintf(&b, "\nOther household members:\n%s\n", r.HouseholdMembers)
	}
	fmt.Fprintf(&b, "\nPrimary church: %s\n", ChurchLabel(r.PrimaryChurch))
	fmt.Fprintf(&b, "Wants offering envelopes: %v\n", r.WantsEnvelopes)
	if r.EnvelopeNumber != "" {
		fmt.Fprintf(&b, "Existing envelope #: %s\n", r.EnvelopeNumber)
	}
	if r.ContactPreferences != "" {
		fmt.Fprintf(&b, "Preferred contact: %s\n", r.ContactPreferences)
	}
	if r.Notes != "" {
		fmt.Fprintf(&b, "\nNotes:\n%s\n", r.Notes)
	}
	fmt.Fprintf(&b, "\n— Our Lady of the Valley Parish website\n")
	return b.String()
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// ---------- Admin side ----------

// AdminRegistrationsPage lists registrations, newest first. Filter by ?show=unread.
func (h *AdminStructuredHandler) RegistrationsPage(c *gin.Context) {
	unreadOnly := c.Query("show") == "unread"
	q := h.db.Order("created_at DESC").Limit(adminListLimit)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	var rows []models.ParishRegistration
	if err := q.Find(&rows).Error; err != nil {
		log.Printf("Admin registrations: load failed: %v", err)
	}
	var total, unread int64
	h.db.Model(&models.ParishRegistration{}).Count(&total)
	h.db.Model(&models.ParishRegistration{}).Where("read_at IS NULL").Count(&unread)

	data := h.pageData(c, "admin-registrations", "Registrations — Admin — OLV Parish")
	data["registrations"] = rows
	data["totalRegistrations"] = total
	data["unreadRegistrations"] = unread
	data["unreadOnly"] = unreadOnly
	c.HTML(http.StatusOK, "admin-registrations.html", data)
}

func (h *AdminStructuredHandler) RegistrationMarkRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req struct {
		Read bool `json:"read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var readAt interface{}
	if req.Read {
		readAt = time.Now()
	}
	result := h.db.Model(&models.ParishRegistration{}).Where("id = ?", id).Update("read_at", readAt)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Updated"})
}

func (h *AdminStructuredHandler) RegistrationDelete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := h.db.Delete(&models.ParishRegistration{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

