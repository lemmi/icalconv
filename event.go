package main

import (
	"sort"
	"strings"
	"time"

	"github.com/jordic/goics"
	"github.com/pkg/errors"
)

var days = [...]string{
	"So",
	"Mo",
	"Di",
	"Mi",
	"Do",
	"Fr",
	"Sa",
}

var months = [...]string{
	"",
	"Januar",
	"Februar",
	"März",
	"April",
	"Mai",
	"Juni",
	"Juli",
	"August",
	"September",
	"Oktober",
	"November",
	"Dezember",
}

func sameDay(t1, t2 time.Time) bool {
	sy, sm, sd := t1.Date()
	ey, em, ed := t2.Date()
	return sy == ey && sm == em && sd == ed
}

type Event struct {
	Start, End                         time.Time
	Id, Description, Location, Summary string
	Categories                         []string // sorted
}

func (e Event) HasStart() bool {
	return !e.Start.IsZero()
}

func (e Event) HasEnd() bool {
	return !e.End.IsZero()
}

func (e Event) HasStartClock() bool {
	h, m, s := e.Start.Clock()
	return !(h == 0 && m == 0 && s == 0)
}

func (e Event) HasEndClock() bool {
	h, m, s := e.End.Clock()
	return !(h == 0 && m == 0 && s == 0)
}

func (e Event) DeepCopy() Event {
	var catCopy []string
	if e.Categories != nil {
		copy(catCopy, e.Categories)
	}
	e.Categories = catCopy
	return e
}

// hacky german translations

func (e Event) StartWeekday() string {
	return days[e.Start.Weekday()]
}
func (e Event) EndWeekday() string {
	// fix end date for midnight
	return days[e.End.Weekday()]
}
func (e Event) StartMonth() string {
	return months[e.Start.Month()]
}

type Events []Event

func (evs Events) SortBy(less ...lessFunc) Events {
	sort.Sort(EventSorter{evs, less})
	return evs
}

// Default sort
func (evs Events) Sort() Events {
	return evs.SortBy(
		ByStart,
		ByDescription,
		ByLocation,
		ByCategories,
		BySummary,
		ByEnd,
		ById,
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

func (evs Events) SplitMonths() []Events {
	ret := make([]Events, 12)
	for _, ev := range evs {
		m := int(ev.Start.Month()) - 1
		ret[m] = append(ret[m], ev)
	}
	return ret
}

// split events that span more than a day to make rendering easier
func (evs Events) SplitLongEvents(prependStart, prependEnd string) Events {
	var ret Events

	for _, e := range evs.Sort() {
		if !e.HasEnd() || sameDay(e.Start, e.End) {
			ret = append(ret, e)
		} else {
			splitStart := e.DeepCopy()
			splitEnd := e.DeepCopy()

			splitStart.End = time.Time{}
			splitEnd.End = time.Time{}

			splitEnd.Start = e.End

			splitStart.Summary = prependStart + splitStart.Summary
			splitEnd.Summary = prependEnd + splitEnd.Summary

			ret = append(ret, splitStart, splitEnd)
		}
	}

	return ret.Sort()
}

func (evs Events) Categories() []string {
	ss := make(stringSet)
	for _, e := range evs {
		ss.AddSlice(e.Categories)
	}
	return ss.Slice()
}

func (evs Events) SplitDays() []Events {
	var ret []Events

	var bucket Events
	for _, e := range evs.Sort() {
		if len(bucket) == 0 || sameDay(bucket[0].Start, e.Start) {
			bucket = append(bucket, e)
		} else {
			ret = append(ret, bucket)
			bucket = Events{e}
		}
	}

	if len(bucket) > 0 {
		ret = append(ret, bucket)
	}

	return ret
}

// extra categories
func (evs Events) OpCategories(f strSliceBoolOp, cats ...string) Events {
	if len(cats) == 0 {
		return evs
	}
	for i := range evs {
		evs[i].Categories = f(evs[i].Categories, cats)
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

var replacer = strings.NewReplacer(`\,`, `,`, `\\`, `\`)

func getVal(props map[string]*goics.IcsNode, key string) string {
	return replacer.Replace(getProp(props, key).Val)
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
		} else if !d.HasEndClock() {
			d.End = d.End.AddDate(0, 0, -1) // End times are exclusive, fixes midnight problem
		}
		d.Categories = splitText(getVal(props, "CATEGORIES"))
		sort.StringSlice(d.Categories).Sort()
		d.Id = getVal(props, "UID")
		d.Summary = getVal(props, "SUMMARY")
		d.Description = getVal(props, "DESCRIPTION")
		d.Location = getVal(props, "LOCATION")
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

func ByStart(e1, e2 *Event) bool {
	return e1.Start.Before(e2.Start)
}
func ByEnd(e1, e2 *Event) bool {
	return e1.End.Before(e2.End)
}
func ById(e1, e2 *Event) bool {
	return e1.Id < e2.Id
}
func BySummary(e1, e2 *Event) bool {
	return e1.Summary < e2.Summary
}
func ByDescription(e1, e2 *Event) bool {
	return e1.Description < e2.Description
}
func ByLocation(e1, e2 *Event) bool {
	return e1.Description < e2.Description
}
func ByCategories(e1, e2 *Event) bool {
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
