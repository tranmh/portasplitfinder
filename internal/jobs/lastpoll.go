package jobs

import (
	"sync"
	"time"
)

// LastPoll tracks the timestamp of the most recent completed poll cycle, for the
// poller's /healthz endpoint. Safe for concurrent use.
type LastPoll struct {
	mu sync.RWMutex
	at time.Time
}

// Set records the time of the latest poll.
func (l *LastPoll) Set(t time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.at = t
}

// At returns the latest poll time and whether a poll has happened.
func (l *LastPoll) At() (time.Time, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.at, !l.at.IsZero()
}

// Age returns how long ago the last poll happened, and whether one has occurred.
func (l *LastPoll) Age(now time.Time) (time.Duration, bool) {
	at, ok := l.At()
	if !ok {
		return 0, false
	}
	return now.Sub(at), true
}
