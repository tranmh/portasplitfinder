package jobs

import (
	"sync"
	"testing"
	"time"
)

func TestLastPoll_ZeroValue(t *testing.T) {
	var lp LastPoll
	at, ok := lp.At()
	if ok || !at.IsZero() {
		t.Errorf("At() = %v, %v on zero value, want zero time, false", at, ok)
	}
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	age, ok := lp.Age(now)
	if ok || age != 0 {
		t.Errorf("Age() = %v, %v on zero value, want 0, false", age, ok)
	}
}

func TestLastPoll_SetAtAge(t *testing.T) {
	var lp LastPoll
	t1 := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	lp.Set(t1)

	at, ok := lp.At()
	if !ok || !at.Equal(t1) {
		t.Fatalf("At() = %v, %v, want %v, true", at, ok, t1)
	}

	cases := []struct {
		now  time.Time
		want time.Duration
	}{
		{t1, 0},
		{t1.Add(90 * time.Second), 90 * time.Second},
		{t1.Add(-time.Minute), -time.Minute},
	}
	for _, c := range cases {
		age, ok := lp.Age(c.now)
		if !ok || age != c.want {
			t.Errorf("Age(%v) = %v, %v, want %v, true", c.now, age, ok, c.want)
		}
	}

	// A later Set replaces the earlier value.
	t2 := t1.Add(5 * time.Minute)
	lp.Set(t2)
	if at, _ := lp.At(); !at.Equal(t2) {
		t.Errorf("At() after second Set = %v, want %v", at, t2)
	}
}

func TestLastPoll_SetZeroMeansNoPoll(t *testing.T) {
	var lp LastPoll
	lp.Set(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	lp.Set(time.Time{})
	if _, ok := lp.At(); ok {
		t.Error("At() reports a poll after Set(zero time)")
	}
	if _, ok := lp.Age(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); ok {
		t.Error("Age() reports a poll after Set(zero time)")
	}
}

func TestLastPoll_ConcurrentUse(t *testing.T) {
	var lp LastPoll
	base := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	const writers, readers, iters = 8, 8, 500

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				lp.Set(base.Add(time.Duration(w*iters+i+1) * time.Second))
			}
		}(w)
	}
	errs := make(chan string, readers)
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				at, ok := lp.At()
				if ok && !at.After(base) {
					errs <- "At() returned a value that was never set: " + at.String()
					return
				}
				if age, ok := lp.Age(base.Add(time.Hour * 24)); ok && age <= 0 {
					errs <- "Age() returned a non-positive age for a set value"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if at, ok := lp.At(); !ok || !at.After(base) {
		t.Errorf("final At() = %v, %v", at, ok)
	}
}
