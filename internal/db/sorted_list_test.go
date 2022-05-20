package db

import (
	"testing"
)

func TestSortedList_Search(t *testing.T) {
	tests := []struct {
		SortedList   sortedList
		Search, Want uint64
		Ok           bool
	}{
		{
			SortedList: []uint64{1, 2, 3, 4, 5},
			Search:     3,
			Want:       3,
			Ok:         true,
		},
		{
			SortedList: []uint64{1, 2, 3, 4, 5},
			Search:     1,
			Want:       1,
			Ok:         true,
		},
		{
			SortedList: []uint64{1, 2, 3, 4, 5},
			Search:     5,
			Want:       5,
			Ok:         true,
		},
		{
			SortedList: []uint64{1, 2, 4, 5},
			Search:     3,
			Want:       2,
			Ok:         true,
		},
		{
			SortedList: []uint64{2, 4, 5},
			Search:     1,
			Want:       0,
			Ok:         false,
		},
		{
			SortedList: []uint64{2, 4, 5},
			Search:     100,
			Want:       5,
			Ok:         true,
		},
		{
			SortedList: []uint64{},
			Search:     100,
			Want:       0,
			Ok:         false,
		},
	}
	for _, test := range tests {
		result, ok := test.SortedList.Search(test.Search)
		if result != test.Want || ok != test.Ok {
			t.Errorf("%+v.Search(%d) = %d, %t, want: %d, %t", test.SortedList, test.Search, result, ok, test.Want, test.Ok)
		}
	}
}

func TestSortedList_Add(t *testing.T) {
	tests := []struct {
		SortedList sortedList
		Value      uint64
		Want       sortedList
	}{
		{
			SortedList: []uint64{1, 2, 3, 4, 5},
			Value:      3,
			Want:       []uint64{1, 2, 3, 4, 5},
		},
		{
			SortedList: []uint64{1, 2, 4, 5},
			Value:      3,
			Want:       []uint64{1, 2, 3, 4, 5},
		},
		{
			SortedList: []uint64{1, 2, 4, 5},
			Value:      0,
			Want:       []uint64{0, 1, 2, 4, 5},
		},
		{
			SortedList: []uint64{1, 2, 4, 5},
			Value:      6,
			Want:       []uint64{1, 2, 4, 5, 6},
		},
		{
			SortedList: []uint64{},
			Value:      3,
			Want:       []uint64{3},
		},
	}
	for _, test := range tests {
		l := make([]uint64, len(test.SortedList))
		copy(l, test.SortedList)
		sl := sortedList(l)

		sl.Add(test.Value)
		if !equals(sl, test.Want) {
			t.Errorf("%+v.Add(%d) = %+v, want %+v", test.SortedList, test.Value, sl, test.Want)
		}
	}
}

func equals(x, y sortedList) bool {
	if len(x) != len(y) {
		return false
	}
	for i := 0; i < len(x); i++ {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
