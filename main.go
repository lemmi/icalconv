package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jordic/goics"
	"github.com/lemmi/closer"
	"github.com/pkg/errors"
)

type stringSet map[string]struct{}

func (set stringSet) Add(s string) stringSet {
	set[s] = struct{}{}
	return set
}
func (set stringSet) AddSlice(s []string) stringSet {
	for _, k := range s {
		set.Add(k)
	}
	return set
}
func (set stringSet) Sub(s string) stringSet {
	delete(set, s)
	return set
}
func (set stringSet) Slice() []string {
	ret := make([]string, 0, len(set))
	for s := range set {
		ret = append(ret, s)
	}
	sort.StringSlice(ret).Sort()
	return ret
}

// bool operations

func strUnion(s1, s2 []string) []string {
	set := make(stringSet, len(s1)+len(s2))
	for _, s := range s1 {
		set.Add(s)
	}
	for _, s := range s2 {
		set.Add(s)
	}
	return set.Slice()
}
func strSub(s1, s2 []string) []string {
	set := make(stringSet, len(s1)+len(s2))
	for _, s := range s1 {
		set.Add(s)
	}
	for _, s := range s2 {
		set.Sub(s)
	}
	return set.Slice()
}
func strCut(s1, s2 []string) []string {
	set2 := stringSet{}.AddSlice(s2)
	ret := stringSet{}
	for _, s := range s1 {
		if _, ok := set2[s]; ok {
			ret.Add(s)
		}
	}
	return ret.Slice()
}

// Event type

type Event struct {
	Start, End                         time.Time
	Id, Description, Location, Summary string
	Categories                         []string // sorted
}

type Events []Event

func (evs Events) SortBy(less ...lessFunc) Events {
	sort.Sort(EventSorter{evs, less})
	return evs
}

// Default sort
func (evs Events) Sort() Events {
	return evs.SortBy(
		byStart,
		byDescription,
		byLocation,
		byCategories,
		bySummary,
		byEnd,
		byId,
	)
}

// filters
func (evs Events) FilterTime(f func(time.Time) int, val int) Events {
	var ret Events
	for _, e := range evs {
		if f(e.Start) == val {
			ret = append(ret, e)
		}
	}
	return ret
}

// extra categories
func (evs Events) OpCategories(f func([]string, []string) []string, cats string) Events {
	if cats == "" {
		return evs
	}
	c := splitText(cats)
	for i := range evs {
		evs[i].Categories = f(evs[i].Categories, c)
	}
	return evs
}

// utils parsing

func splitText(s string) []string {
	var ret []string
	for _, t := range strings.Split(s, ",") {
		if token := strings.TrimSpace(t); len(token) > 0 {
			ret = append(ret, token)
		}
	}
	return ret
}

func trimRightZ(node *goics.IcsNode) *goics.IcsNode {
	node.Val = strings.TrimRight(node.Val, "Z")
	return node
}

func getProp(props map[string]*goics.IcsNode, key string) *goics.IcsNode {
	if prop := props[key]; prop != nil {
		return prop
	}
	return &goics.IcsNode{}
}

func getVal(props map[string]*goics.IcsNode, key string) string {
	return getProp(props, key).Val
}

// parse

func (e *Events) ConsumeICal(c *goics.Calendar, err error) error {
	for _, el := range c.Events {
		var d Event
		var err error

		props := el.Data
		if props == nil {
			return errors.Errorf("Event has no properties: %+v", el)
		}
		d.Start, err = trimRightZ(getProp(props, "DTSTART")).DateDecode()
		if err != nil {
			return errors.Wrap(err, "failed to get start date")
		}
		d.End, err = trimRightZ(getProp(props, "DTEND")).DateDecode()
		if err != nil {
			return errors.Wrap(err, "failed to get end date")
		}
		if d.Start.Equal(d.End) {
			d.End = time.Time{}
		}
		d.Categories = splitText(getVal(props, "CATEGORIES"))
		sort.StringSlice(d.Categories).Sort()
		d.Id = getVal(props, "UID")
		d.Summary = getVal(props, "SUMMARY")
		d.Description = getVal(props, "DESCRIPTION")
		*e = append(*e, d)
	}
	return nil
}

// sorter

type lessFunc func(*Event, *Event) bool

type EventSorter struct {
	evs  Events
	less []lessFunc
}

func (es EventSorter) Less(i, j int) bool {
	e1, e2 := &es.evs[i], &es.evs[j]
	k := 0
	for ; k < len(es.less)-1; k++ {
		l := es.less[k]
		switch {
		case l(e1, e2):
			return true
		case l(e2, e1):
			return false
		}
		// no decision yet
	}

	return es.less[k](e1, e2)
}

func (es EventSorter) Len() int {
	return len(es.evs)
}

func (es EventSorter) Swap(i, j int) {
	es.evs[i], es.evs[j] = es.evs[j], es.evs[i]
}

func byStart(e1, e2 *Event) bool {
	return e1.Start.Before(e2.Start)
}
func byEnd(e1, e2 *Event) bool {
	return e1.End.Before(e2.End)
}
func byId(e1, e2 *Event) bool {
	return e1.Id < e2.Id
}
func bySummary(e1, e2 *Event) bool {
	return e1.Summary < e2.Summary
}
func byDescription(e1, e2 *Event) bool {
	return e1.Description < e2.Description
}
func byLocation(e1, e2 *Event) bool {
	return e1.Description < e2.Description
}
func byCategories(e1, e2 *Event) bool {
	cats1 := e1.Categories
	cats2 := e2.Categories
	for i := range cats1 {
		if i >= len(cats2) {
			return false
		}
		if cats1[i] < cats2[i] {
			return true
		}
	}
	return len(cats1) < len(cats2)
}

type Printer func(io.Writer, Events) error

func Out(p Printer, fname string, evs Events) error {
	var out io.WriteCloser
	var err error
	switch fname {
	case "":
		return nil
	case "-":
		out = os.Stdout
	default:
		out, err = os.Create(fname)
		if err != nil {
			return errors.Wrap(err, "Can't open file for Output")
		}
		defer closer.WithStackTrace(out)()
	}

	return p(out, evs)
}

func printLatex(out io.Writer, evs Events) error {
	const latexTmpl = `
{{- range . -}}
\event
{{- with .Categories}}*{{end}}
{{- .Start.Format "{2006-01-02}"}}
{{- printf "{%s}" .Summary}}
{{- with .Categories}}[color={{index . 0}}]{{end}}
{{end}}`

	var t = template.Must(template.New("latex").Parse(latexTmpl))
	return t.Execute(out, evs)
}

func printDebug(out io.Writer, evs Events) error {
	for _, ev := range evs {
		fmt.Printf("%+v\n", ev)
	}
	return nil
}

func main() {
	var opts struct {
		ifname           string
		ofname           string
		lfname           string
		debug            bool
		year             int
		appendcategories string
		removecategories string
		limitcategories  string
	}

	var ifile io.ReadCloser
	var err error

	ifile = os.Stdin

	flag.StringVar(&opts.ifname, "i", "-", "input filename")
	flag.StringVar(&opts.ofname, "o", "", "output filename")
	flag.StringVar(&opts.lfname, "l", "", "latex output filename")
	flag.BoolVar(&opts.debug, "d", false, "print debug info")
	flag.IntVar(&opts.year, "y", time.Now().Year(), "limit output to year")
	flag.StringVar(&opts.appendcategories, "ca", "", "append categories \"cat1,cat2,cat3...\"")
	flag.StringVar(&opts.removecategories, "cr", "", "remove categories \"cat1,cat2,cat3...\"")
	flag.StringVar(&opts.limitcategories, "cl", "", "limit categories \"cat1,cat2,cat3...\"")
	flag.Parse()

	if opts.ifname != "-" {
		ifile, err = os.Open(opts.ifname)
		if err != nil {
			log.Panic(err)
		}
	}
	defer ifile.Close()

	d := goics.NewDecoder(ifile)
	var evs Events
	err = d.Decode(&evs)
	if err != nil {
		fmt.Printf("%#v\n", err)
	}

	evs.OpCategories(strSub, opts.removecategories)
	evs.OpCategories(strUnion, opts.appendcategories)
	evs.OpCategories(strCut, opts.limitcategories)
	evs = evs.Sort().FilterTime(time.Time.Year, opts.year)

	if opts.debug {
		Out(printDebug, "-", evs)
	}
	err = Out(printLatex, opts.lfname, evs)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}
