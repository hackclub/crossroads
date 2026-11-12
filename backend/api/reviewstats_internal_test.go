package api

import (
	"testing"
	"time"
)

func TestReviewWeek(t *testing.T) {
	cases := []struct{ now, start, end string }{
		{"2026-11-15T23:00:00-05:00", "2026-11-09T00:00:00-05:00", "2026-11-16T00:00:00-05:00"}, // Sunday night
		{"2026-11-16T00:30:00-05:00", "2026-11-16T00:00:00-05:00", "2026-11-23T00:00:00-05:00"}, // just after Monday starts
		{"2026-11-16T05:30:00Z", "2026-11-16T00:00:00-05:00", "2026-11-23T00:00:00-05:00"},      // UTC, and Monday in New York too
		{"2026-11-16T04:30:00Z", "2026-11-09T00:00:00-05:00", "2026-11-16T00:00:00-05:00"},      // UTC Monday, still Sunday in New York
		{"2026-11-01T12:00:00-05:00", "2026-10-26T00:00:00-04:00", "2026-11-02T00:00:00-05:00"}, // the week clocks went back
	}
	for _, c := range cases {
		now, _ := time.Parse(time.RFC3339, c.now)
		wantStart, _ := time.Parse(time.RFC3339, c.start)
		wantEnd, _ := time.Parse(time.RFC3339, c.end)
		start, end := reviewWeek(now)
		if !start.Equal(wantStart) || !end.Equal(wantEnd) {
			t.Errorf("reviewWeek(%s) = %s to %s, want %s to %s", c.now, start, end, wantStart, wantEnd)
		}
	}
}

func TestCellsDefuseFormulas(t *testing.T) {
	got := cells("=HYPERLINK(\"x\")", "+1", "-2", "@SUM(A1)", "Racer", 3, "")
	want := []string{"'=HYPERLINK(\"x\")", "'+1", "'-2", "'@SUM(A1)", "Racer", "3", ""}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %q, want %q", i, got[i], want[i])
		}
	}
}
