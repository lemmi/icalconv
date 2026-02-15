package main

import (
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/jordic/goics"
	"github.com/lemmi/icalconv"
	"github.com/pkg/errors"
)

func printDebug(out io.Writer, evs icalconv.Events) error {
	for _, ev := range evs {
		fmt.Fprintf(out, "%+v\n", ev)
	}
	return nil
}

func debug(format string, a ...any) {
	if opts.debug {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

var opts struct {
	debug    bool
	year     int
	month    int
	chcat    string
	template string
}

func main() {
	var err error

	flag.BoolVar(&opts.debug, "d", false, "print debug info")
	flag.IntVar(&opts.year, "y", math.MinInt64, "limit output to year")
	flag.IntVar(&opts.month, "m", math.MinInt64, "limit output to month")
	flag.StringVar(&opts.chcat, "c", "", "append (+), remove (-) or limit (=) categories \"+cat1,-cat2,=cat3...\"")
	flag.StringVar(&opts.template, "t", "", "template file to render output")
	flag.Parse()

	args := flag.Args()

	if opts.template == "" {
		log.Fatal("No template specified. Use -t templatefile")
	}

	var evs icalconv.Events

	if len(args) == 0 {
		// Fallback to stdin as before
		d := goics.NewDecoder(os.Stdin)
		if err = d.Decode(&evs); err != nil {
			log.Fatalf("%#v\n", err)
		}
	} else {
		for file_filter := range slices.Chunk(args, 2) {
			if len(file_filter) != 2 {
				continue
			}
			filePath, filter := file_filter[0], file_filter[1]

			fevs, loadErr := icalconv.LoadEventsFromFile(filePath)
			if loadErr != nil {
				log.Fatalf("Failed to load %s: %v", filePath, loadErr)
			}
			fevs = icalconv.ApplyCategoryFilters(fevs, filter)
			evs = append(evs, fevs...)
		}
	}

	evs = slices.CompactFunc(evs.SortBy(icalconv.ById), func(e1, e2 icalconv.Event) bool {
		// only deduplicate events with non-empty UID
		if e1.UID == e2.UID {
			return e1.UID != ""
		}
		return false
	})

	if opts.year != math.MinInt64 {
		debug("Filtering year %d\n", opts.year)
		start := time.Date(opts.year, time.January, 1, 0, 0, 0, 0, time.Local)
		evs = evs.ExpandBetween(start, start.AddDate(1, 0, 0))
	}
	if opts.month != math.MinInt64 {
		debug("Filtering month %d\n", opts.month)
		evs = evs.FilterTime(func(t time.Time) int { return int(time.Time.Month(t)) }, opts.month)
	}

	evs = evs.Sort()
	evs = icalconv.ApplyCategoryFilters(evs, opts.chcat)

	if opts.debug {
		icalconv.PrinterFunc(printDebug).Print(os.Stderr, evs)
	}

	var t interface {
		Execute(io.Writer, any) error
	}

	if strings.HasPrefix(strings.ToLower(filepath.Ext(opts.template)), ".html") {
		t, err = htmltemplate.New(filepath.Base(opts.template)).Funcs(sprig.FuncMap()).ParseFiles(opts.template)
	} else {
		t, err = template.New(filepath.Base(opts.template)).Funcs(sprig.FuncMap()).ParseFiles(opts.template)
	}
	if err != nil {
		log.Fatalf("%+v", errors.Wrapf(err, "Error parsing %q", opts.template))
	}
	err = t.Execute(os.Stdout, evs)
	if err != nil {
		log.Fatalf("%+v", errors.Wrapf(err, "Error executing %q", opts.template))
	}
}
