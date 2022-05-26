package db

import "sort"

type sortedList []uint64

func (s sortedList) searchIndex(x uint64) int {
	return sort.Search(len(s), func(i int) bool {
		return x < s[i]
	})
}

func (s sortedList) Search(x uint64) (uint64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	i := s.searchIndex(x)
	if i == len(s) {
		return s[len(s)-1], true
	}
	if i == 0 {
		return 0, false
	}
	return s[i-1], true
}

func (s *sortedList) Add(x uint64) {
	i := s.searchIndex(x)
	if i == 0 {
		*s = append([]uint64{x}, *s...)
		return
	}
	if (*s)[i-1] == x {
		return
	}
	if i == len(*s) {
		*s = append(*s, x)
		return
	}
	*s = append((*s)[:i+1], (*s)[i:]...)
	(*s)[i] = x
}
