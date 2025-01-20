package trie2

import (
	"maps"
)

type nodeTracer struct {
	inserts map[Path]struct{}
	deletes map[Path]struct{}
}

func newTracer() *nodeTracer {
	return &nodeTracer{
		inserts: make(map[Path]struct{}),
		deletes: make(map[Path]struct{}),
	}
}

func (t *nodeTracer) onInsert(key *Path) {
	k := *key
	if _, present := t.deletes[k]; present {
		return
	}
	t.inserts[k] = struct{}{}
}

func (t *nodeTracer) onDelete(key *Path) {
	k := *key
	if _, present := t.inserts[k]; present {
		return
	}
	t.deletes[k] = struct{}{}
}

func (t *nodeTracer) reset() {
	t.inserts = make(map[Path]struct{})
	t.deletes = make(map[Path]struct{})
}

func (t *nodeTracer) copy() *nodeTracer {
	return &nodeTracer{
		inserts: maps.Clone(t.inserts),
		deletes: maps.Clone(t.deletes),
	}
}

func (t *nodeTracer) deletedNodes() []Path {
	keys := make([]Path, 0, len(t.deletes))
	for k := range t.deletes {
		keys = append(keys, k)
	}
	return keys
}
