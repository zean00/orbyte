package httpx

import (
	"time"

	"clinic/internal/platform/identity"
)

type loginRateLimiter struct {
	ident    *identity.Service
	window   time.Duration
	attempts int
	now      func() time.Time
}

func newLoginRateLimiter(ident *identity.Service, attempts int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		ident:    ident,
		window:   window,
		attempts: attempts,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (l *loginRateLimiter) Allow(key string) bool {
	if l == nil || l.ident == nil || l.attempts <= 0 || l.window <= 0 {
		return true
	}
	now := l.now()
	_ = l.ident.CleanupLoginFailures(now.Add(-l.window))
	return l.ident.CountRecentLoginFailures(key, now.Add(-l.window)) < l.attempts
}

func (l *loginRateLimiter) AddFailure(key string) {
	if l == nil || l.ident == nil || l.attempts <= 0 || l.window <= 0 {
		return
	}
	now := l.now()
	_ = l.ident.CleanupLoginFailures(now.Add(-l.window))
	_ = l.ident.RecordLoginFailure(key, now)
}

func (l *loginRateLimiter) Reset(key string) {
	if l == nil || l.ident == nil {
		return
	}
	_ = l.ident.ClearLoginFailures(key)
}
