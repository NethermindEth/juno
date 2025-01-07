package trie2

import (
	"maps"
)

type tracer struct {
	inserts map[BitArray]struct{}
	deletes map[BitArray]struct{}
}

func newTracer() *tracer {
	return &tracer{
		inserts: make(map[BitArray]struct{}),
		deletes: make(map[BitArray]struct{}),
	}
}

func (t *tracer) onInsert(key *BitArray) {
	t.inserts[*key] = struct{}{}
}

func (t *tracer) onDelete(key *BitArray) {
	t.deletes[*key] = struct{}{}
}

func (t *tracer) reset() {
	t.inserts = make(map[BitArray]struct{})
	t.deletes = make(map[BitArray]struct{})
}

func (t *tracer) copy() *tracer {
	return &tracer{
		inserts: maps.Clone(t.inserts),
		deletes: maps.Clone(t.deletes),
	}
}

func (t *tracer) deletedNodes() []BitArray {
	keys := make([]BitArray, 0, len(t.deletes))
	for k := range t.deletes {
		keys = append(keys, k)
	}
	return keys
}
