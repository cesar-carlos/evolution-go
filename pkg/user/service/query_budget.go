package user_service

import (
	"errors"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

const queryOverLimitCooldown = 45 * time.Second

type queryRateLimitedError struct {
	retryAfter time.Duration
}

func (e queryRateLimitedError) Error() string {
	return whatsmeow.ErrIQRateOverLimit.Error()
}

func (e queryRateLimitedError) Unwrap() error {
	return whatsmeow.ErrIQRateOverLimit
}

// QueryRetryAfter reports remaining backoff for a WhatsApp rate-limit error.
// Unknown remaining time uses queryOverLimitCooldown. Non-rate-limit errors return 0.
func QueryRetryAfter(err error) time.Duration {
	var limited queryRateLimitedError
	if errors.As(err, &limited) && limited.retryAfter > 0 {
		return limited.retryAfter
	}
	if errors.Is(err, whatsmeow.ErrIQRateOverLimit) {
		return queryOverLimitCooldown
	}
	return 0
}

type instanceQueryLimit struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func newInstanceQueryLimit() *instanceQueryLimit {
	return &instanceQueryLimit{until: make(map[string]time.Time)}
}

func (u *userService) cooldownDuration() time.Duration {
	if u != nil && u.queryCooldown > 0 {
		return u.queryCooldown
	}
	return queryOverLimitCooldown
}

func (u *userService) errIfQueryCoolingDown(instanceID string) error {
	if u == nil || u.queryLimit == nil || instanceID == "" {
		return nil
	}
	remaining := u.queryLimit.remaining(instanceID, u.currentTime())
	if remaining <= 0 {
		return nil
	}
	return queryRateLimitedError{retryAfter: remaining}
}

func (u *userService) markAndWrapOverLimit(instanceID string) error {
	cooldown := u.cooldownDuration()
	if u != nil && u.queryLimit != nil && instanceID != "" {
		u.queryLimit.mark(instanceID, u.currentTime().Add(cooldown))
	}
	return queryRateLimitedError{retryAfter: cooldown}
}

func (l *instanceQueryLimit) mark(instanceID string, until time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.until == nil {
		l.until = make(map[string]time.Time)
	}
	l.until[instanceID] = until
}

func (l *instanceQueryLimit) remaining(instanceID string, now time.Time) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.until[instanceID]
	if !ok {
		return 0
	}
	remaining := until.Sub(now)
	if remaining <= 0 {
		delete(l.until, instanceID)
		return 0
	}
	return remaining
}
