package audit

import (
	"testing"
	"time"
)

func TestSpend(t *testing.T) {
	q := &Requests{Budget: 1000}
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	steps := []struct {
		after          time.Duration
		userID         int64
		size, bodySize int
		keep, keepBody bool
		unrecorded     int
	}{
		{0, 1, 100, 700, true, true, 0},              // 200 left
		{0, 1, 100, 700, true, false, 0},             // no room for the body: 100 left
		{0, 1, 200, 0, false, false, 1},              // no room at all
		{0, 1, 200, 0, false, false, 2},              // still none
		{0, 2, 900, 0, true, true, 0},                // someone else has their own
		{12 * time.Second, 1, 250, 0, true, true, 2}, // a fifth of a minute gives 200 back: 300 left
		{time.Hour, 1, 1000, 0, true, true, 0},       // never more than a minute's worth
		{0, 1, 1, 0, false, false, 1},
	}
	for i, s := range steps {
		now = now.Add(s.after)
		keep, keepBody, unrecorded := q.spend(s.userID, s.size, s.bodySize, now)
		if keep != s.keep || keepBody != s.keepBody || unrecorded != s.unrecorded {
			t.Errorf("step %d: got %v, %v, %d, want %v, %v, %d", i, keep, keepBody, unrecorded, s.keep, s.keepBody, s.unrecorded)
		}
	}
}
