package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/lemmi/icalconv/merge"
)

func main() {
	timezone := flag.String("tz", time.Local.String(), "Timezone for X-WR-TIMEZONE")
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		log.Fatal("No input files specified. Usage: icalmerge file1.ics [file2.ics ...]")
	}

	// Open each file and collect readers.
	readers := make([]io.Reader, 0, len(args))
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("Failed to open %s: %v", path, err)
		}
		// Defer close after the loop to keep files open while reading.
		defer f.Close()
		readers = append(readers, f)
	}

	// Merge all calendars and write to stdout.
	if err := merge.MergeCalendars(os.Stdout, *timezone, readers...); err != nil {
		log.Fatalf("Merge error: %v", err)
	}

	// Ensure a trailing newline.
	fmt.Fprint(os.Stdout, "\r\n")
}
