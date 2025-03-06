package pathdb

import (
	"bytes"
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func getBytesBuffer() *bytes.Buffer {
	buf := bytesBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBytesBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bytesBufferPool.Put(buf)
}

var errSnapshotStale = errors.New("layer is stale")

type diskLayer struct {
	root   felt.Felt // State root hash of this disk layer
	id     uint64    // Corresponding state id
	block  uint64    // Associated block number
	buffer *buffer   // Dirty buffer to aggregate node writes
	db     *Database // Path-based trie database

	stale bool // If true, the layer is stale and should not be used
	lock  sync.RWMutex
}

func newDiskLayer(root felt.Felt, id, block uint64, db *Database, buffer *buffer) *diskLayer {
	return &diskLayer{
		root:   root,
		id:     id,
		block:  block,
		buffer: buffer,
		db:     db,
	}
}

func (dl *diskLayer) node(owner felt.Felt, path trieutils.Path, isClass bool) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if dl.stale {
		return nil, errSnapshotStale
	}

	n, found := dl.buffer.node(owner, path, isClass)
	if found {
		return n.Blob(), nil
	}

	buf := getBytesBuffer()
	defer putBytesBuffer(buf)

	_, err := dl.db.Get(buf, owner, path, isClass)
	if err != nil {
		return nil, err
	}

	var blob []byte
	copy(blob, buf.Bytes())
	return blob, nil
}

// Merges the given diff layer into the node buffer and returns a newly constructed
// disk layer.
func (dl *diskLayer) commit(diff *diffLayer, force bool) (*diskLayer, error) {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	if dl.stale {
		return nil, errSnapshotStale
	}
	dl.stale = true

	// TODO(weiihann): write state id?
	// TODO(weiihann): handle potential edge case?

	combined := dl.buffer.commit(diff.nodes)
	if combined.isFull() || force {
		panic("TODO(weiihann): flush the buffer")
	}

	newDisk := newDiskLayer(diff.rootHash(), diff.stateID(), diff.block, dl.db, combined)
	return newDisk, nil
}

func (dl *diskLayer) update(root felt.Felt, id, block uint64, nodes *nodeSet) *diffLayer {
	return newDiffLayer(dl, root, id, block, nodes)
}

func (dl *diskLayer) journal() error {
	panic("TODO(weiihann): implement me")
}

func (dl *diskLayer) rootHash() felt.Felt {
	return dl.root
}

func (dl *diskLayer) stateID() uint64 {
	return dl.id
}

func (dl *diskLayer) parentLayer() layer {
	return nil
}

func (dl *diskLayer) isStale() bool {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.stale
}

func (dl *diskLayer) markStale() {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	if dl.stale {
		panic("layer is already marked as stale")
	}

	dl.stale = true
}

func (dl *diskLayer) size() uint64 {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if dl.stale {
		return 0
	}

	return dl.buffer.size()
}

func (dl *diskLayer) resetCache() {
	dl.lock.Lock()
	defer dl.lock.Unlock()

	if dl.stale {
		return
	}

	dl.buffer.reset()
}
