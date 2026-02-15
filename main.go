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
	"github.com/pkg/errors"
)

func printDebug(out io.Writer, evs Events) error {
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

	var evs Events

	if len(args) == 0 {
		// Fallback to stdin as before
		d := goics.NewDecoder(os.Stdin)
		if err = d.Decode(&evs); err != nil {
			log.Fatalf("%#v\n", err)
		}
	} else {
		var fileSlices []Events
		i := 0
		// Process files with optional per‑file category filters.
		for i < len(args) {
			filePath := args[i]
			i++

			var filter string
			if i < len(args) && (strings.HasPrefix(args[i], "+") ||
				strings.HasPrefix(args[i], "-") ||
				strings.HasPrefix(args[i], "=")) {
				filter = args[i]
				i++
			}

			fevs, loadErr := loadEventsFromFile(filePath)
			if loadErr != nil {
				log.Fatalf("Failed to load %s: %v", filePath, loadErr)
			}
			if filter != "" {
				fevs = applyCategoryFilters(fevs, filter)
			}
			fileSlices = append(fileSlices, fevs)
		}
		evs = mergeEventSlices(fileSlices...)
	}

	evs = slices.CompactFunc(evs.SortBy(ById), func(e1, e2 Event) bool {
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
	var limitCategories []string

	for i, catopt := range splitText(opts.chcat) {
		if len(catopt) <= 1 {
			debug("Ignoring empty category change #%d\n", i)
			continue
		}

		opt, cat := catopt[0], catopt[1:]
		var optf strSliceBoolOp

		switch opt {
		case '+':
			optf = strUnion
		case '-':
			optf = strSub
		case '=':
			limitCategories = append(limitCategories, cat)
			continue
		default:
			debug("Unknown operation %q\n", opt)
		}

		evs.OpCategories(optf, cat)
	}
	evs.OpCategories(strCut, limitCategories...)

	if opts.debug {
		PrinterFunc(printDebug).Print(os.Stderr, evs)
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
