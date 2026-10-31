package handlers

import (
	"testing"
	"time"
)

func TestLoginLimiterLocksEmailAfterFiveFailures(t *testing.T) {
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	l := newLoginLimiter()
	l.now = func() time.Time { return clock }

	for i := 0; i < maxFailsPerEmail-1; i++ {
		l.recordFailure("1.1.1.1", "Admin@Parish.org")
	}
	if l.blockedFor("1.1.1.1", "admin@parish.org") != 0 {
		t.Fatal("locked too early")
	}
	l.recordFailure("2.2.2.2", "admin@parish.org") // different IP, same email
	if l.blockedFor("3.3.3.3", "ADMIN@parish.org") == 0 {
		t.Fatal("email not locked after 5 failures from mixed IPs")
	}

	clock = clock.Add(16 * time.Minute)
	if l.blockedFor("3.3.3.3", "admin@parish.org") != 0 {
		t.Fatal("lock did not expire")
	}
}

func TestLoginLimiterSuccessClearsEmail(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < maxFailsPerEmail-1; i++ {
		l.recordFailure("1.1.1.1", "a@b.org")
	}
	l.recordSuccess("1.1.1.1", "a@b.org")
	l.recordFailure("1.1.1.1", "a@b.org")
	if l.blockedFor("1.1.1.1", "a@b.org") != 0 {
		t.Fatal("success should reset the email counter")
	}
}

func TestLoginLimiterLocksIPAcrossEmails(t *testing.T) {
	l := newLoginLimiter()
	for i := 0; i < maxFailsPerIP; i++ {
		l.recordFailure("9.9.9.9", "user"+string(rune('a'+i%26))+"@x.org")
	}
	if l.blockedFor("9.9.9.9", "someone-new@x.org") == 0 {
		t.Fatal("IP not locked after spraying many emails")
	}
}
