package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

// icsReader returns an io.Reader containing a minimal iCalendar representation.
// If categories is non‑empty, a CATEGORIES line is added.
func icsReader(categories []string) io.Reader {
	var catLine string
	if len(categories) > 0 {
		catLine = "CATEGORIES:" + strings.Join(categories, ",") + "\r\n"
	}
	ics := "BEGIN:VCALENDAR\r\n" +
		"VERSION:2.0\r\n" +
		"PRODID:-//icalconv test//EN\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:test\r\n" +
		"TZID:Europe/Berlin\r\n" +
		"DTSTART:20240101T090000Z\r\n" +
		"DTEND:20240101T100000Z\r\n" +
		"DTSTAMP:20240101T000000Z\r\n" +
		"SUMMARY:Test Event\r\n" +
		catLine +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	return strings.NewReader(ics)
}

// Test loading events from an io.Reader.
func TestLoadEventsFromReader(t *testing.T) {
	r := icsReader([]string{"catA", "catB"})
	evs, err := loadEventsFromReader(r)
	if err != nil {
		t.Fatalf("loadEventsFromReader failed: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	if got, want := evs[0].Categories, []string{"catA", "catB"}; !equalStringSlices(got, want) {
		t.Fatalf("categories mismatch: got %v want %v", got, want)
	}
}

// Test the category‑filter helper.
func TestApplyCategoryFilters(t *testing.T) {
	evs := Events{{Categories: []string{"cat1", "cat2"}}}
	// +cat3, -cat1, =cat2  => result should be ["cat2"] (limit cuts to cat2)
	filtered := applyCategoryFilters(evs, "+cat3,-cat1,=cat2")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event after filtering")
	}
	want := []string{"cat2"}
	if got := filtered[0].Categories; !equalStringSlices(got, want) {
		t.Fatalf("categories after filter: got %v want %v", got, want)
	}
}

// Test merging of several event slices.
func TestMergeEventSlices(t *testing.T) {
	ev1 := Events{{Summary: "A"}}
	ev2 := Events{{Summary: "B"}, {Summary: "C"}}
	merged := mergeEventSlices(ev1, ev2)
	if len(merged) != 3 {
		t.Fatalf("expected 3 events after merge, got %d", len(merged))
	}
	expected := []string{"A", "B", "C"}
	for i, ev := range merged {
		if ev.Summary != expected[i] {
			t.Fatalf("merged[%d] = %q, want %q", i, ev.Summary, expected[i])
		}
	}
}

// End‑to‑end test that loads two readers, applies per‑file filters and merges.
func TestFullWorkflow(t *testing.T) {
	// First file: catX, catY
	r1 := icsReader([]string{"catX", "catY"})
	// Second file: catY, catZ
	r2 := icsReader([]string{"catY", "catZ"})

	evs1, err := loadEventsFromReader(r1)
	if err != nil {
		t.Fatalf("load first reader: %v", err)
	}
	evs2, err := loadEventsFromReader(r2)
	if err != nil {
		t.Fatalf("load second reader: %v", err)
	}

	// Apply per‑file filters.
	evs1 = applyCategoryFilters(evs1, "=catX") // keep only catX
	evs2 = applyCategoryFilters(evs2, "+catA") // add catA

	merged := mergeEventSlices(evs1, evs2)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged events, got %d", len(merged))
	}
	if got, want := merged[0].Categories, []string{"catX"}; !equalStringSlices(got, want) {
		t.Fatalf("first event categories: got %v want %v", got, want)
	}
	if got, want := merged[1].Categories, []string{"catA", "catY", "catZ"}; !equalStringSlices(got, want) {
		t.Fatalf("second event categories: got %v want %v", got, want)
	}
}

// Verify that the start time is parsed correctly by goics.
func TestEventTimeParsing(t *testing.T) {
	r := icsReader(nil)
	evs, err := loadEventsFromReader(r)
	if err != nil {
		t.Fatalf("loadEventsFromReader failed: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event")
	}
	expected := time.Date(2024, time.January, 1, 9, 0, 0, 0, time.UTC)
	if !evs[0].Start.Equal(expected) {
		t.Logf("%v#", evs)
		t.Fatalf("start time mismatch: got %v want %v", evs[0].Start, expected)
	}
}

// Helper to compare two slices of strings (order matters).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
