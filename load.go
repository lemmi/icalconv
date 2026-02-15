package icalconv

import (
	"io"
	"os"

	"github.com/jordic/goics"
)

// loadEventsFromFile opens the file at the given path and decodes the iCalendar data.
// It is a thin wrapper around loadEventsFromReader.
func LoadEventsFromFile(path string) (Events, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadEventsFromReader(f)
}

// loadEventsFromReader decodes iCalendar data from any io.Reader and returns the events.
func LoadEventsFromReader(r io.Reader) (Events, error) {
	var evs Events
	return evs, goics.NewDecoder(r).Decode(&evs)
}

// applyCategoryFilters mutates the provided Events according to a filter specification.
// The filter string is a comma‑separated list of operations:
//
//	+cat   – add category `cat` to each event (union)
//	-cat   – remove category `cat` from each event (subtract)
//	=cat   – limit the final categories to the intersection with `cat` (cut)
//
// The function mirrors the logic that previously lived in `main.go`.
func ApplyCategoryFilters(evs Events, filter string) Events {
	var limitCategories []string

	for _, catopt := range splitText(filter) {
		if len(catopt) <= 1 {
			continue
		}
		opt, cat := catopt[0], catopt[1:]

		switch opt {
		case '+':
			evs = evs.OpCategories(StrUnion, cat)
		case '-':
			evs = evs.OpCategories(StrSub, cat)
		case '=':
			limitCategories = append(limitCategories, cat)
		}
	}
	// Apply the final cut operation if any =‑filters were supplied.
	if len(limitCategories) > 0 {
		evs = evs.OpCategories(StrCut, limitCategories...)
	}
	return evs
}

// mergeEventSlices concatenates a variadic list of Events slices into a single Events slice.
// It does not perform any deduplication; callers can sort or deduplicate afterwards if needed.
func MergeEventSlices(slices ...Events) Events {
	var merged Events
	for _, s := range slices {
		merged = append(merged, s...)
	}
	return merged
}
