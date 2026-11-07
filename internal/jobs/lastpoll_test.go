package jobs

import (
	"sync"
	"testing"
	"time"
)

var lastPollBase = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

func TestLastPoll_ZeroValueNeverPolled(t *testing.T) {
	var lp LastPoll
	at, ok := lp.At()
	if ok || !at.IsZero() {
		t.Fatalf("At() = %v, %v; want zero, false", at, ok)
	}
	age, ok := lp.Age(lastPollBase)
	if ok || age != 0 {
		t.Fatalf("Age() = %v, %v; want 0, false", age, ok)
	}
}

func TestLastPoll_SetAndAge(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{"same instant", lastPollBase, 0},
		{"90 seconds later", lastPollBase.Add(90 * time.Second), 90 * time.Second},
		{"one day later", lastPollBase.Add(24 * time.Hour), 24 * time.Hour},
		{"clock behind", lastPollBase.Add(-time.Minute), -time.Minute},
		{"other zone same instant", lastPollBase.In(time.FixedZone("CET", 3600)), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var lp LastPoll
			lp.Set(lastPollBase)
			at, ok := lp.At()
			if !ok || !at.Equal(lastPollBase) {
				t.Fatalf("At() = %v, %v; want %v, true", at, ok, lastPollBase)
			}
			age, ok := lp.Age(tc.now)
			if !ok || age != tc.want {
				t.Fatalf("Age() = %v, %v; want %v, true", age, ok, tc.want)
			}
		})
	}
}

func TestLastPoll_LastSetWins(t *testing.T) {
	var lp LastPoll
	lp.Set(lastPollBase.Add(time.Hour))
	lp.Set(lastPollBase)
	at, ok := lp.At()
	if !ok || !at.Equal(lastPollBase) {
		t.Fatalf("At() = %v, %v; want %v", at, ok, lastPollBase)
	}
}

func TestLastPoll_SetZeroResets(t *testing.T) {
	var lp LastPoll
	lp.Set(lastPollBase)
	lp.Set(time.Time{})
	if _, ok := lp.At(); ok {
		t.Fatal("setting the zero time must report no poll")
	}
	if _, ok := lp.Age(lastPollBase); ok {
		t.Fatal("Age must report no poll after zero Set")
	}
}

func TestLastPoll_ConcurrentUse(t *testing.T) {
	var lp LastPoll
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			lp.Set(lastPollBase.Add(time.Duration(i+1) * time.Second))
		}(i)
		go func() {
			defer wg.Done()
			_, _ = lp.Age(lastPollBase)
		}()
	}
	wg.Wait()
	at, ok := lp.At()
	if !ok {
		t.Fatal("expected a poll to be recorded")
	}
	if at.Before(lastPollBase.Add(time.Second)) || at.After(lastPollBase.Add(16*time.Second)) {
		t.Fatalf("At() = %v outside the set range", at)
	}
}
