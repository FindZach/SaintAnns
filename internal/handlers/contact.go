package handlers

import (
	"log"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	maxContactFieldLength   = 254
	maxContactMessageLength = 5000
)

// MessageHandler saves messages sent from the Contact page. Staff read them on
// the admin dashboard.
type MessageHandler struct {
	db *gorm.DB
}

type contactRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	Website   string `json:"website"` // honeypot: hidden on the page, so only bots fill it in
}

func NewMessageHandler(db *gorm.DB) *MessageHandler {
	return &MessageHandler{db: db}
}

func (h *MessageHandler) Submit(c *gin.Context) {
	var req contactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please fill in the form and try again."})
		return
	}

	if req.Website != "" {
		// Answer as if it worked, so the bot has nothing to learn from
		c.JSON(http.StatusCreated, gin.H{"message": "Message sent"})
		return
	}

	message := models.ContactMessage{
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Email:     strings.TrimSpace(req.Email),
		Phone:     strings.TrimSpace(req.Phone),
		Message:   strings.TrimSpace(req.Message),
	}
	subject, validSubject := models.ContactSubjectLabel(req.Subject)
	message.Subject = subject

	var problem string
	switch {
	case message.FirstName == "" || message.LastName == "" || message.Email == "" || message.Message == "":
		problem = "Please fill in your name, email and message."
	case !validSubject:
		problem = "Please choose a subject."
	case !validEmail(message.Email):
		problem = "Please enter a valid email address."
	case len(message.FirstName) > maxContactFieldLength || len(message.LastName) > maxContactFieldLength ||
		len(message.Email) > maxContactFieldLength || len(message.Phone) > maxContactFieldLength:
		problem = "Some of your details are too long. Please shorten them and try again."
	case utf8.RuneCountInString(message.Message) > maxContactMessageLength:
		problem = "Please keep your message under 5,000 characters."
	}
	if problem != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": problem})
		return
	}

	if userID := c.GetUint("user_id"); userID != 0 {
		message.UserID = &userID
	}

	if err := h.db.Create(&message).Error; err != nil {
		log.Printf("Failed to save contact message: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "We couldn’t send your message. Please try again."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Message sent"})
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}
