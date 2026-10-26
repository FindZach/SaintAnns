package handlers

import (
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Parish details shown across the site. These are the only copies, so a change
// here reaches every page.

type parishLocation struct {
	Name    string
	Street  string
	City    string
	Mapped  string // Google Maps URL
	Phone   string
}

type siteDetails struct {
	ParishName   string
	ShortName    string
	Email        string
	OfficePhone  string
	OfficeStreet string
	OfficeCity   string
	Diocese      string
	DioceseURL   string
	Facebook     string
	Locations    []parishLocation
}

var siteInfo = siteDetails{
	ParishName:   "Our Lady of the Valley Parish",
	ShortName:    "OLV Parish",
	Email:        "parish@olv14843.org",
	OfficePhone:  "(607) 324-7698",
	OfficeStreet: "27 Erie Avenue",
	OfficeCity:   "Hornell, NY 14843",
	Diocese:      "Diocese of Rochester",
	DioceseURL:   "https://www.dor.org/",
	Facebook:     "https://www.facebook.com/",
	Locations: []parishLocation{
		{
			Name:   "St. Ann's Church",
			Street: "27 Erie Avenue",
			City:   "Hornell, NY 14843",
		},
		{
			Name:   "St. Mary's Church",
			Street: "Main Street",
			City:   "Rexville, NY 14877",
		},
	},
}

// photoDir holds optional parish photos keyed by slot name; see the README there.
const photoDir = "web/static/images/photos"

var photoExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// photoURL returns the URL of the photo for a slot such as "st-anns", or ""
// until one is added. The file's mtime is in the URL so a replaced photo isn't
// served from cache.
func photoURL(slot string) string {
	for _, ext := range photoExts {
		info, err := os.Stat(filepath.Join(photoDir, slot+ext))
		if err == nil {
			return "/static/images/photos/" + slot + ext + "?v=" + strconv.FormatInt(info.ModTime().Unix(), 10)
		}
	}
	return ""
}

// TemplateFuncs are available to every page template. We close over `db` so
// the shared public nav can be rendered inline from base.html without every
// handler having to pass it explicitly. db may be nil in degraded mode; the
// nav loader falls back to a built-in list.
func TemplateFuncs(db *gorm.DB) template.FuncMap {
	return template.FuncMap{
		"site":        func() siteDetails { return siteInfo },
		"photo":       photoURL,
		"currentYear": func() int { return time.Now().Year() },
		"nav":          func() []NavEntry { return LoadPublicNav(db) },
		"formatTime":   FormatTimeOfDay,
		"churchLabel":  ChurchLabel,
	}
}
