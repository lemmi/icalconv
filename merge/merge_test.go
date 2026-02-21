package merge

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestMergeCalendars(t *testing.T) {
	cal1 := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:1
SUMMARY:Event 1
END:VEVENT
END:VCALENDAR`
	cal2 := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:2
SUMMARY:Event 2
END:VEVENT
END:VCALENDAR`

	readers := []io.Reader{
		strings.NewReader(cal1),
		strings.NewReader(cal2),
	}

	var out bytes.Buffer
	if err := MergeCalendars(readers, &out); err != nil {
		t.Fatalf("MergeCalendars returned error: %v", err)
	}

	expected := "BEGIN:VCALENDAR\r\nPRODID:-//icalmerge//EN\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:1\r\nSUMMARY:Event 1\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:2\r\nSUMMARY:Event 2\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

	if out.String() != expected {
		t.Errorf("Merged output does not match expected.\nGot:\n%s\nExpected:\n%s", out.String(), expected)
	}
}

func TestMergeCalendars_FoldedBegin(t *testing.T) {
	cal1 := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:1
SUMMARY:Event 1
END:VEVENT
END:VCALENDAR`

	// This calendar contains a line that starts with a space followed by BEGIN:
	// It should be treated as a normal line, not as a block delimiter.
	cal2 := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:3
DESCRIPTION:First part
 BEGIN:XYZ
END:VEVENT
END:VCALENDAR`

	readers := []io.Reader{
		strings.NewReader(cal1),
		strings.NewReader(cal2),
	}

	var out bytes.Buffer
	if err := MergeCalendars(readers, &out); err != nil {
		t.Fatalf("MergeCalendars returned error: %v", err)
	}

	expected := "BEGIN:VCALENDAR\r\nPRODID:-//icalmerge//EN\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:1\r\nSUMMARY:Event 1\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:3\r\nDESCRIPTION:First part\r\n BEGIN:XYZ\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

	if out.String() != expected {
		t.Errorf("Merged output does not match expected.\nGot:\n%s\nExpected:\n%s", out.String(), expected)
	}
}
