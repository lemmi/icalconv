package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"time"

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
	debug            bool
	year             int64
	month            int64
	appendcategories string
	removecategories string
	limitcategories  string
	chcat            string
}

func main() {
	var err error

	flag.BoolVar(&opts.debug, "d", false, "print debug info")
	flag.Int64Var(&opts.year, "y", math.MinInt64, "limit output to year")
	flag.Int64Var(&opts.year, "m", math.MinInt64, "limit output to month")
	flag.StringVar(&opts.chcat, "c", "", "append (+), remove (-) or limit (=) categories \"+cat1,-cat2,=cat3...\"")
	flag.Parse()

	d := goics.NewDecoder(os.Stdin)
	var evs Events
	err = d.Decode(&evs)
	if err != nil {
		fmt.Printf("%#v\n", err)
	}

	if opts.year != math.MinInt64 {
		evs = evs.FilterTime(time.Time.Year, int(opts.year))
	}
	if opts.month != math.MinInt64 {
		evs = evs.FilterTime(time.Time.Year, int(opts.year))
	}

	evs = evs.Sort()

	for i, catopt := range strings.Split(opts.chcat, ",") {
		catopt = strings.TrimSpace(catopt)
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
			optf = strCut
		default:
			debug("Unknown operation %q\n", opt)
		}

		evs.OpCategories(optf, cat)
	}

	if opts.debug {
		PrinterFunc(printDebug).Print(os.Stderr, evs)
	}

	for _, tmpl := range flag.Args() {
		t, err := template.ParseFiles(tmpl)
		if err != nil {
			log.Fatalf("%+v", errors.Wrapf(err, "Error parsing %q", tmpl))
		}
		err = t.Execute(os.Stdout, evs)
		if err != nil {
			log.Fatalf("%+v", errors.Wrapf(err, "Error executing %q", tmpl))
		}
	}
}
