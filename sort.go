package golib

import "sort"

type RankedItem struct {
	Rank float64
	Item interface{}
}

type RankedSlice []RankedItem

func (r RankedSlice) Len() int {
	return len(r)
}

func (r RankedSlice) Less(i, j int) bool {
	return r[i].Rank < r[j].Rank
}

func (r RankedSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r RankedSlice) Sort() {
	sort.Sort(r)
}

func (r RankedSlice) SortReverse() {
	sort.Sort(sort.Reverse(r))
}

func (r *RankedSlice) Append(rank float64, item interface{}) {
	*r = append(*r, RankedItem{Rank: rank, Item: item})
}

func (r RankedSlice) Items() []interface{} {
	result := make([]interface{}, len(r))
	for i, item := range r {
		result[i] = item.Item
	}
	return result
}

func (r RankedSlice) ItemsSorted() []interface{} {
	r.Sort()
	return r.Items()
}

func (r RankedSlice) ItemsSortedReverse() []interface{} {
	r.SortReverse()
	return r.Items()
}
