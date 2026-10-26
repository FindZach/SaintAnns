package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"olv-parish/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ScheduleHandler serves the public Mass Times and Confession Times pages,
// and lends its loaders to the home page + block renderer.
type ScheduleHandler struct {
	db *gorm.DB
}

func NewScheduleHandler(db *gorm.DB) *ScheduleHandler {
	return &ScheduleHandler{db: db}
}

// MassTimesPage renders /liturgy-and-sacraments/mass-times/.
func (h *ScheduleHandler) MassTimesPage(c *gin.Context) {
	rows := LoadMassSchedule(h.db)
	grouped := groupMassByChurch(rows)
	c.HTML(http.StatusOK, "mass-times.html", gin.H{
		"title":      "Mass Times - Our Lady of the Valley Parish",
		"page":       "mass-times",
		"massGroups": grouped,
		"version":    assetVersion,
	})
}

// ConfessionTimesPage renders /liturgy-and-sacraments/confession-times/.
func (h *ScheduleHandler) ConfessionTimesPage(c *gin.Context) {
	rows := LoadConfessionTimes(h.db)
	c.HTML(http.StatusOK, "confession-times.html", gin.H{
		"title":           "Confession Times - Our Lady of the Valley Parish",
		"page":            "confession-times",
		"confessionTimes": rows,
		"version":         assetVersion,
	})
}

// LoadMassSchedule returns every Mass time, ordered by sort then day of week
// then time of day. Degraded mode returns an empty list.
func LoadMassSchedule(db *gorm.DB) []models.MassSchedule {
	if db == nil {
		return nil
	}
	var rows []models.MassSchedule
	if err := db.Order("sort ASC, day_of_week ASC, time_of_day ASC").Find(&rows).Error; err != nil {
		return nil
	}
	return rows
}

// LoadConfessionTimes mirrors LoadMassSchedule for ConfessionTime.
func LoadConfessionTimes(db *gorm.DB) []models.ConfessionTime {
	if db == nil {
		return nil
	}
	var rows []models.ConfessionTime
	if err := db.Order("sort ASC, day_of_week ASC, start_of_day ASC").Find(&rows).Error; err != nil {
		return nil
	}
	return rows
}

// MassGroup is the public page's view of Masses at one church.
type MassGroup struct {
	Church string
	Label  string
	Rows   []models.MassSchedule
}

func groupMassByChurch(rows []models.MassSchedule) []MassGroup {
	byChurch := map[string]*MassGroup{}
	order := []string{}
	for _, r := range rows {
		g, ok := byChurch[r.Church]
		if !ok {
			g = &MassGroup{Church: r.Church, Label: ChurchLabel(r.Church)}
			byChurch[r.Church] = g
			order = append(order, r.Church)
		}
		g.Rows = append(g.Rows, r)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return churchSortKey(order[i]) < churchSortKey(order[j])
	})
	groups := make([]MassGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, *byChurch[key])
	}
	return groups
}

func churchSortKey(church string) int {
	switch church {
	case models.ChurchStAnn:
		return 10
	case models.ChurchStMary:
		return 20
	case models.ChurchEither:
		return 30
	case models.ChurchParish:
		return 40
	default:
		return 90
	}
}

// ChurchLabel is the public-facing name for a Church identifier.
func ChurchLabel(church string) string {
	switch church {
	case models.ChurchStAnn:
		return "St. Ann's, Hornell"
	case models.ChurchStMary:
		return "St. Mary's, Rexville"
	case models.ChurchEither:
		return "Either church"
	case models.ChurchParish:
		return "Parish-wide"
	default:
		return church
	}
}

// FormatTimeOfDay turns "16:30" into "4:30 PM" for display. Garbage in returns
// the raw string.
func FormatTimeOfDay(hhmm string) string {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return hhmm
	}
	return t.Format("3:04 PM")
}

// weekdayLabel is "Sunday" for 0, "Monday" for 1, …
func weekdayLabel(day int) string {
	switch day {
	case 0:
		return "Sunday"
	case 1:
		return "Monday"
	case 2:
		return "Tuesday"
	case 3:
		return "Wednesday"
	case 4:
		return "Thursday"
	case 5:
		return "Friday"
	case 6:
		return "Saturday"
	}
	return strconv.Itoa(day)
}
