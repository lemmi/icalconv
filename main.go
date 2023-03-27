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

func debug(format string, a ...interface{}) {
	if opts.debug {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

var opts struct {
	debug bool
	year  int
	month int
	chcat string
}

func main() {
	var err error

	flag.BoolVar(&opts.debug, "d", false, "print debug info")
	flag.IntVar(&opts.year, "y", math.MinInt64, "limit output to year")
	flag.IntVar(&opts.month, "m", math.MinInt64, "limit output to month")
	flag.StringVar(&opts.chcat, "c", "", "append (+), remove (-) or limit (=) categories \"+cat1,-cat2,=cat3...\"")
	flag.Parse()

	d := goics.NewDecoder(os.Stdin)
	var evs Events
	err = d.Decode(&evs)
	if err != nil {
		log.Fatalf("%#v\n", err)

	}

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

	for _, tmpl := range flag.Args() {
		var t interface {
			Execute(io.Writer, any) error
		}
		var err error

		if strings.HasPrefix(strings.ToLower(filepath.Ext(tmpl)), ".html") {
			t, err = htmltemplate.New(filepath.Base(tmpl)).Funcs(sprig.FuncMap()).ParseFiles(tmpl)
		} else {
			t, err = template.New(filepath.Base(tmpl)).Funcs(sprig.FuncMap()).ParseFiles(tmpl)
		}
		if err != nil {
			log.Fatalf("%+v", errors.Wrapf(err, "Error parsing %q", tmpl))
		}
		err = t.Execute(os.Stdout, evs)
		if err != nil {
			log.Fatalf("%+v", errors.Wrapf(err, "Error executing %q", tmpl))
		}
	}
}
