package main

import (
	"sort"
	"strings"
	"time"

	"github.com/jordic/goics"
	"github.com/pkg/errors"
)

type Event struct {
	Start, End                         time.Time
	Id, Description, Location, Summary string
	Categories                         []string // sorted
}

func (e Event) HasEnd() bool {
	return !e.End.IsZero()
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
