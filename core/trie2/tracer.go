package trie2

import (
	"maps"
)

type tracer struct {
	inserts map[Path]struct{}
	deletes map[Path]struct{}
}

func newTracer() *tracer {
	return &tracer{
		inserts: make(map[Path]struct{}),
		deletes: make(map[Path]struct{}),
	}
}

func (t *tracer) onInsert(key *Path) {
	k := *key
	if _, present := t.deletes[k]; present {
		return
	}
	t.inserts[k] = struct{}{}
}

func (t *tracer) onDelete(key *Path) {
	k := *key
	if _, present := t.inserts[k]; present {
		return
	}
	t.deletes[k] = struct{}{}
}

func (t *tracer) reset() {
	t.inserts = make(map[Path]struct{})
	t.deletes = make(map[Path]struct{})
}

func (t *tracer) copy() *tracer {
	return &tracer{
		inserts: maps.Clone(t.inserts),
		deletes: maps.Clone(t.deletes),
	}
}

func (t *tracer) deletedNodes() []Path {
	keys := make([]Path, 0, len(t.deletes))
	for k := range t.deletes {
		keys = append(keys, k)
	}
	return keys
}
