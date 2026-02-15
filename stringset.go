package icalconv

import (
	"sort"
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

type StrSliceBoolOp func([]string, []string) []string

func StrUnion(s1, s2 []string) []string {
	set := make(stringSet, len(s1)+len(s2))
	set = set.AddSlice(s1)
	set = set.AddSlice(s2)
	return set.Slice()
}
func StrSub(s1, s2 []string) []string {
	set := make(stringSet, len(s1)+len(s2))
	for _, s := range s1 {
		set.Add(s)
	}
	for _, s := range s2 {
		set.Sub(s)
	}
	return set.Slice()
}
func StrCut(s1, s2 []string) []string {
	set2 := stringSet{}.AddSlice(s2)
	ret := stringSet{}
	for _, s := range s1 {
		if _, ok := set2[s]; ok {
			ret.Add(s)
		}
	}
	return ret.Slice()
}
