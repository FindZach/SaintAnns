package handlers

import (
	"net/http"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CalendarHandler serves the public /events-news/calendar/ page. It reads
// events from our own Events table — staff add and edit them under
// /admin/events — and groups them by month so the page scans quickly.
type CalendarHandler struct {
	db *gorm.DB
}

func NewCalendarHandler(db *gorm.DB) *CalendarHandler {
	return &CalendarHandler{db: db}
}

// EventMonthGroup is one month in the calendar listing, with every event that
// starts in that month.
type EventMonthGroup struct {
	Label  string // e.g. "October 2026"
	Key    string // e.g. "2026-10" (for anchors)
	Events []models.Event
}

// Calendar renders the calendar page.
func (h *CalendarHandler) Calendar(c *gin.Context) {
	var upcoming []models.Event
	if h.db != nil {
		// Show events whose start date is in the last 7 days or later, so
		// something that started today still shows while the day is in
		// progress. Load the next 60 upcoming events, which is enough for a
		// year's worth even in an active parish.
		since := time.Now().AddDate(0, 0, -7)
		if err := h.db.
			Preload("FlyerAsset").
			Where("starts_at >= ?", since).
			Order("starts_at ASC").
			Limit(60).
			Find(&upcoming).Error; err != nil {
			upcoming = nil
		}
	}

	groups := groupEventsByMonth(upcoming)

	c.HTML(http.StatusOK, "calendar.html", gin.H{
		"title":   "Parish Calendar — Our Lady of the Valley Parish",
		"page":    "calendar",
		"groups":  groups,
		"version": assetVersion,
	})
}

// groupEventsByMonth bins events into month-sized groups in start-date order,
// preserving event order within each month.
func groupEventsByMonth(events []models.Event) []EventMonthGroup {
	if len(events) == 0 {
		return nil
	}
	var out []EventMonthGroup
	byKey := make(map[string]int)
	for _, e := range events {
		key := e.StartsAt.Format("2006-01")
		if idx, ok := byKey[key]; ok {
			out[idx].Events = append(out[idx].Events, e)
			continue
		}
		byKey[key] = len(out)
		out = append(out, EventMonthGroup{
			Label:  e.StartsAt.Format("January 2006"),
			Key:    key,
			Events: []models.Event{e},
		})
	}
	return out
}
