package api

import (
	"maps"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/github"
	"github.com/scherepi/crossroads-backend/hackatime"
)

func TestCommitTimes(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, time.UTC) }
	span := func(h1, m1, h2, m2 int) hackatime.Span { return hackatime.Span{Start: at(h1, m1), End: at(h2, m2)} }
	spans := []hackatime.Span{
		span(9, 0, 10, 0),    // an hour before the first commit
		span(10, 30, 11, 30), // split by the second commit at 11:00
		span(13, 0, 13, 20),  // after the last commit
	}
	// Newest first, as GitHub gives them.
	commits := []github.Commit{{SHA: "c", Date: at(12, 0)}, {SHA: "b", Date: at(11, 0)}, {SHA: "a", Date: at(10, 15)}}

	got, after := commitTimes(commits, spans, at(8, 0), at(14, 0), false)
	want := map[string]int{"a": 3600, "b": 1800, "c": 1800}
	if !maps.Equal(got, want) || after != 1200 {
		t.Errorf("got %v and %d after, want %v and 1200", got, after, want)
	}

	// Truncated: the oldest commit's start isn't known, so it's left out.
	got, _ = commitTimes(commits, spans, at(8, 0), at(14, 0), true)
	if _, ok := got["a"]; ok || got["b"] != 1800 {
		t.Errorf("truncated: %v", got)
	}

	// Nothing tracked between two commits is 0, not missing.
	got, after = commitTimes(commits, nil, at(8, 0), at(14, 0), false)
	if len(got) != 3 || got["b"] != 0 || after != 0 {
		t.Errorf("no spans: %v, %d", got, after)
	}
}
