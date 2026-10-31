package handlers

import (
	"strings"
	"sync"
	"time"
)

// loginLimiter slows password guessing. Failed attempts are counted per email
// (the reliable key — client IPs behind the proxy can be spoofed) and per IP
// (a looser limit, since the parish office may share one address). Hitting a
// limit locks that key until the window passes. State is in memory, so a
// restart clears it; that's acceptable for a single-instance deployment.
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	window  time.Duration
	now     func() time.Time
}

type limiterEntry struct {
	fails       int
	windowStart time.Time
	lockedUntil time.Time
}

const (
	maxFailsPerEmail = 5
	maxFailsPerIP    = 20
)

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{entries: map[string]*limiterEntry{}, window: 15 * time.Minute, now: time.Now}
}

var defaultLoginLimiter = newLoginLimiter()

func limiterKeys(ip, email string) (string, string) {
	return "ip:" + ip, "email:" + strings.ToLower(strings.TrimSpace(email))
}

// blockedFor reports how long the caller must wait, or 0 if allowed.
func (l *loginLimiter) blockedFor(ip, email string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	ipKey, emailKey := limiterKeys(ip, email)
	var wait time.Duration
	for _, k := range []string{ipKey, emailKey} {
		if e, ok := l.entries[k]; ok && now.Before(e.lockedUntil) {
			if d := e.lockedUntil.Sub(now); d > wait {
				wait = d
			}
		}
	}
	return wait
}

func (l *loginLimiter) recordFailure(ip, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.pruneLocked(now)
	ipKey, emailKey := limiterKeys(ip, email)
	l.bumpLocked(ipKey, maxFailsPerIP, now)
	l.bumpLocked(emailKey, maxFailsPerEmail, now)
}

func (l *loginLimiter) recordSuccess(ip, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, emailKey := limiterKeys(ip, email)
	delete(l.entries, emailKey)
}

func (l *loginLimiter) bumpLocked(key string, max int, now time.Time) {
	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) > l.window {
		e = &limiterEntry{windowStart: now}
		l.entries[key] = e
	}
	e.fails++
	if e.fails >= max {
		e.lockedUntil = now.Add(l.window)
	}
}

func (l *loginLimiter) pruneLocked(now time.Time) {
	if len(l.entries) < 1000 {
		return
	}
	for k, e := range l.entries {
		if now.Sub(e.windowStart) > l.window && now.After(e.lockedUntil) {
			delete(l.entries, k)
		}
	}
}
