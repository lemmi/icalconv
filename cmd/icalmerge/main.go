package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/lemmi/icalconv/merge"
)

func main() {
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
	if err := merge.MergeCalendars(readers, os.Stdout); err != nil {
		log.Fatalf("Merge error: %v", err)
	}

	// Ensure a trailing newline.
	fmt.Fprintln(os.Stdout)
}
