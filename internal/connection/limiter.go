package connection

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type AdaptiveLimiter struct {
	mu               sync.Mutex
	dailyLimit       int
	weeklyLimit      int
	hourlyLimit      int
	requestsSent     map[string]int // key: "2024-01-15", "2024-01-15:14", "2024-W03"
	failures         int
	consecutiveFails int
	lastFailureTime  time.Time
	backoffDuration  time.Duration
	circuitOpen      bool
	circuitOpenUntil time.Time
}

func NewAdaptiveLimiter(daily, weekly, hourly int) *AdaptiveLimiter {
	return &AdaptiveLimiter{
		dailyLimit:      daily,
		weeklyLimit:     weekly,
		hourlyLimit:     hourly,
		requestsSent:    make(map[string]int),
		backoffDuration: 30 * time.Second,
	}
}

func (al *AdaptiveLimiter) CanSend() (bool, string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Check circuit breaker
	if al.circuitOpen {
		if time.Now().Before(al.circuitOpenUntil) {
			remaining := time.Until(al.circuitOpenUntil)
			return false, fmt.Sprintf("circuit breaker open, retry in %v", remaining)
		}
		// Reset circuit breaker
		al.circuitOpen = false
		al.consecutiveFails = 0
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	thisHour := now.Format("2006-01-02:15")
	_, week := now.ISOWeek()
	thisWeek := fmt.Sprintf("%d-W%02d", now.Year(), week)

	// Check daily limit
	if al.requestsSent[today] >= al.dailyLimit {
		return false, "daily limit reached"
	}

	// Check hourly limit
	if al.requestsSent[thisHour] >= al.hourlyLimit {
		return false, "hourly limit reached"
	}

	// Check weekly limit
	if al.requestsSent[thisWeek] >= al.weeklyLimit {
		return false, "weekly limit reached"
	}

	return true, ""
}

func (al *AdaptiveLimiter) RecordSuccess() {
	al.mu.Lock()
	defer al.mu.Unlock()

	now := time.Now()
	today := now.Format("2006-01-02")
	thisHour := now.Format("2006-01-02:15")
	_, week := now.ISOWeek()
	thisWeek := fmt.Sprintf("%d-W%02d", now.Year(), week)

	al.requestsSent[today]++
	al.requestsSent[thisHour]++
	al.requestsSent[thisWeek]++

	// Reset failure tracking on success
	al.consecutiveFails = 0
	al.backoffDuration = 30 * time.Second

	// Cleanup old entries (keep last 30 days)
	al.cleanup(now)
}

func (al *AdaptiveLimiter) RecordFailure(err error) {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.failures++
	al.consecutiveFails++
	al.lastFailureTime = time.Now()

	// Exponential backoff: 30s -> 60s -> 120s -> 240s -> max 5min
	al.backoffDuration *= 2
	if al.backoffDuration > 5*time.Minute {
		al.backoffDuration = 5 * time.Minute
	}

	// Circuit breaker: 3 consecutive failures = pause for 1 hour
	if al.consecutiveFails >= 3 {
		al.circuitOpen = true
		al.circuitOpenUntil = time.Now().Add(1 * time.Hour)
	}
}

func (al *AdaptiveLimiter) GetBackoffDuration() time.Duration {
	al.mu.Lock()
	defer al.mu.Unlock()
	return al.backoffDuration
}

func (al *AdaptiveLimiter) Wait() {
	al.mu.Lock()
	duration := al.backoffDuration
	al.mu.Unlock()

	// Add jitter (±20%)
	jitter := time.Duration(float64(duration) * 0.2 * (rand.Float64()*2 - 1))
	actualDuration := duration + jitter

	time.Sleep(actualDuration)
}

func (al *AdaptiveLimiter) cleanup(now time.Time) {
	cutoff := now.AddDate(0, 0, -30).Format("2006-01-02")

	for key := range al.requestsSent {
		if key < cutoff { // Lexicographical comparison works for YYYY-MM-DD
			delete(al.requestsSent, key)
		}
	}
}

func (al *AdaptiveLimiter) ResetDaily() {
	al.mu.Lock()
	defer al.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	delete(al.requestsSent, today)
}
