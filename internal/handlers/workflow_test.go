package handlers

import (
	"testing"
	"time"

	"olv-parish/internal/models"
)

func TestParseFlexibleTimeUsesParishZone(t *testing.T) {
	// 7 pm on Oct 31 2026 in Hornell is 23:00 UTC (EDT, UTC-4).
	got, err := parseFlexibleTime("2026-10-31T19:00")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got.UTC(), want)
	}
	// After the November switch to EST (UTC-5).
	got, _ = parseFlexibleTime("2026-12-24T22:00")
	if want := time.Date(2026, 12, 25, 3, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got.UTC(), want)
	}
	// Explicit offsets are respected as-is.
	got, _ = parseFlexibleTime("2026-10-31T19:00:00Z")
	if want := time.Date(2026, 10, 31, 19, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got.UTC(), want)
	}
	if e := (models.Event{StartsAt: time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)}); e.LocalStart().Format("3:04 PM") != "7:00 PM" {
		t.Errorf("LocalStart shows %s", e.LocalStart().Format("3:04 PM"))
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"St. Ann's Fall Festival!": "st-anns-fall-festival",
		"  Advent  Mission 2026 ":  "advent-mission-2026",
		"Café Night — Señor":       "cafe-night-senor",
		"???":                      "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidSlugPath(t *testing.T) {
	for _, s := range []string{"about", "about/our-history", "minstries/usher-ministry-2"} {
		if !validSlugPath(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []string{"", "About", "about//x", "about/", "a b", "../etc", "x--y", "-x"} {
		if validSlugPath(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestRoles(t *testing.T) {
	staff := models.User{Role: models.RoleStaff}
	admin := models.User{Role: models.RoleAdmin}
	other := models.User{Role: "viewer"}
	if !staff.IsStaff() || staff.IsAdmin() {
		t.Error("staff should be staff but not admin")
	}
	if !admin.IsStaff() || !admin.IsAdmin() {
		t.Error("admin should be both")
	}
	if other.IsStaff() {
		t.Error("unknown roles must not get admin-panel access")
	}
}

func TestAssetDisplayName(t *testing.T) {
	cases := []struct{ original, path, want string }{
		{"https://www.olv14843.org/wp-content/uploads/Quest20Logo.jpg", "", "Quest20Logo.jpg"},
		{"https://example.org/files/Fall%20Festival.pdf", "", "Fall Festival.pdf"},
		{"bulletin.pdf", "", "bulletin.pdf"},
		{`C:\Users\office\Desktop\flyer.png`, "", "flyer.png"},
		{"", "uploads/2026/10/abc-photo.jpg", "abc-photo.jpg"},
	}
	for _, c := range cases {
		if got := assetDisplayName(c.original, c.path); got != c.want {
			t.Errorf("assetDisplayName(%q, %q) = %q, want %q", c.original, c.path, got, c.want)
		}
	}
}
