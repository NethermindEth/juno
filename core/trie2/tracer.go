package trie2

import (
	"maps"
)

// Tracks the changes to the trie, so that we know which node needs to be updated or deleted in the database
type nodeTracer struct {
	inserts map[Path]struct{}
	deletes map[Path]struct{} // Track both path and hash of deleted nodes
}

func newTracer() nodeTracer {
	return nodeTracer{
		inserts: make(map[Path]struct{}),
		deletes: make(map[Path]struct{}),
	}
}

// Tracks the newly inserted trie node. If the trie node was previously deleted, remove it from the deletion set
// as it means that the node will not be deleted in the database
func (t *nodeTracer) onInsert(key *Path) {
	k := *key
	if _, present := t.deletes[k]; present {
		delete(t.deletes, k)
		return
	}
	t.inserts[k] = struct{}{}
}

// Tracks the newly deleted trie node. If the trie node was previously inserted, remove it from the insertion set
// as it means that the node will not be inserted in the database
func (t *nodeTracer) onDelete(key *Path) {
	k := *key
	if _, present := t.inserts[k]; present {
		delete(t.inserts, k)
		return
	}
	t.deletes[k] = struct{}{}
}

func (t *nodeTracer) copy() nodeTracer {
	return nodeTracer{
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
