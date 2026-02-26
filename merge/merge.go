// Package merge provides a simple, text‑based iCalendar merging utility.
// It concatenates the inner contents of multiple iCalendar files, preserving
// CRLF line endings as required by the iCalendar RFC. Nested VCALENDAR
// sections are considered malformed and cause an error.
package merge

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// crlfWrite writes a string followed by a CRLF sequence.
func crlfWrite(err error, w io.Writer, s string) error {
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, s, "\r\n")
	return err
}

// MergeCalendars reads iCalendar data from each supplied reader, extracts the
// inner contents of each calendar (everything between the outermost
// BEGIN:VCALENDAR and END:VCALENDAR), and writes a single merged calendar
// to the provided writer. The output uses CRLF line endings.
func MergeCalendars(w io.Writer, timezone string, readers ...io.Reader) error {
	var err error
	err = crlfWrite(err, w, "BEGIN:VCALENDAR")
	err = crlfWrite(err, w, "PRODID:-//icalmerge//EN")
	err = crlfWrite(err, w, "VERSION:2.0")
	if timezone != "" {
		err = crlfWrite(err, w, "X-WR-TIMEZONE:"+timezone)
	}
	if err != nil {
		return err
	}
	for _, r := range readers {
		if err := copyCalendarBody(r, w); err != nil {
			return err
		}
	}
	return crlfWrite(nil, w, "END:VCALENDAR")
}

// copyCalendarBody extracts the body of a single iCalendar document (the lines
// that appear after BEGIN:VCALENDAR and before END:VCALENDAR) and writes them
// to w using CRLF line endings. It preserves all inner BEGIN/END blocks and
// any other lines while omitting the outer BEGIN:VCALENDAR and END:VCALENDAR
// markers. Nested VCALENDAR sections are treated as an error.
func copyCalendarBody(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// depth == 0 : outside any calendar
	// depth == 1 : directly inside the outer VCALENDAR
	depth := 0

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		// BEGIN handling
		case strings.HasPrefix(line, "BEGIN:"):
			if line == "BEGIN:VCALENDAR" {
				if depth != 0 {
					return errors.New("nested VCALENDAR block detected")
				}
				// Enter outer calendar – do not write this line.
			}
			// Regular BEGIN for inner components.
			depth++
		// END handling
		case strings.HasPrefix(line, "END:"):
			switch {
			case line == "END:VCALENDAR":
				if depth != 1 {
					return errors.New("unexpected END:VCALENDAR without matching BEGIN")
				}
				// Leaving outer calendar – do not write this line.
			case depth > 1:
				if err := crlfWrite(nil, w, line); err != nil {
					return err
				}
			case depth < 0:
				return errors.New("unexpected END without matching BEGIN")
			}
			depth--
		}

		// Regular line – write only if we are inside the outer calendar.
		if depth > 1 {
			if err := crlfWrite(nil, w, line); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
