package core

import (
	"sync"
	"time"
)

// 为了简单, 直接把速率限制写死在这里
const (
	maxCalls = 20
	period   = time.Second
)

type RateLimiter struct {
	mu    sync.Mutex
	calls []time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		calls: make([]time.Time, 0, maxCalls),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if len(r.calls) < maxCalls {
		r.calls = append(r.calls, now)
		return true
	}

	if now.Sub(r.calls[0]) >= period {
		r.calls = append(r.calls[1:], now)
		return true
	}

	return false
}
