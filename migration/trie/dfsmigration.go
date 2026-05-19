package trie

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type dfsMigrator struct {
	parallelDispatch bool
	pool             *hashWorkerPool
}

func newDFSMigrator(parallelDispatch bool, pool *hashWorkerPool) *dfsMigrator {
	return &dfsMigrator{parallelDispatch: parallelDispatch, pool: pool}
}

func (m *dfsMigrator) Migrate(
	r db.KeyValueReader,
	batch db.Batch,
	desc TrieDesc,
	flush FlushBatchFn,
	stack []dfsFrame,
) (db.Batch, []dfsFrame, error) {
	stack = stack[:0]
	if desc.RootPath == nil {
		return batch, stack, nil
	}
	prefix := oldTriePrefix(desc)
	rootPath := desc.RootPath
	sched := newHashScheduler(desc.HashFn,
		m.parallelDispatch,
		desc.NewBucket, desc.Owner, m.pool)

	rootHash, batch, stack, err := traverse(r, prefix, rootPath, sched, batch, flush, stack)
	if err != nil {
		return batch, stack, err
	}
	if err = sched.sync(batch); err != nil {
		return batch, stack, err
	}
	if rootPath.Len() > 0 {
		if err = writeRootEdge(rootPath, rootHash, sched, batch); err != nil {
			return batch, stack, err
		}
	}
	return batch, stack, nil
}

type traverseStackState uint8

const (
	readNodeState traverseStackState = iota
	leftSubtreeDoneState
	rightSubtreeDoneState
)

type dfsFrame struct {
	oldPath  trie.BitArray
	left     trie.BitArray
	right    trie.BitArray
	value    felt.Felt
	leftHash felt.Felt
	isLeaf   bool
	state    traverseStackState
}

type parsedNode struct {
	value  felt.Felt
	left   trie.BitArray
	right  trie.BitArray
	isLeaf bool
}

func traverse(
	r db.KeyValueReader,
	prefix []byte,
	start *trie.BitArray,
	sched *hashScheduler,
	batch db.Batch,
	flush FlushBatchFn,
	stack []dfsFrame,
) (felt.Felt, db.Batch, []dfsFrame, error) {
	stack = stack[:1]
	stack[0] = dfsFrame{oldPath: *start}
	var lastHash felt.Felt
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		switch top.state {
		case readNodeState:
			parsed, err := readNode(r, prefix, &top.oldPath)
			if err != nil {
				return felt.Felt{}, batch, stack, err
			}
			top.value = parsed.value
			top.left = parsed.left
			top.right = parsed.right
			top.isLeaf = parsed.isLeaf
			if top.isLeaf {
				newPath := toNewPath(&top.oldPath)
				if err := processLeaf(newPath, &top.value, sched, batch); err != nil {
					return felt.Felt{}, batch, stack, err
				}
				lastHash = top.value
				stack = stack[:len(stack)-1]
			} else {
				left := top.left
				top.state = leftSubtreeDoneState
				stack = pushFrame(stack, left)
			}
		case leftSubtreeDoneState:
			top.leftHash = lastHash
			right := top.right
			top.state = rightSubtreeDoneState
			stack = pushFrame(stack, right)
		case rightSubtreeDoneState:
			newPath := toNewPath(&top.oldPath)
			if err := processBinary(
				newPath,
				&top.left,
				&top.right,
				top.leftHash,
				lastHash,
				sched,
				batch,
			); err != nil {
				return felt.Felt{}, batch, stack, err
			}
			lastHash = top.value
			stack = stack[:len(stack)-1]
		}
		var err error
		batch, err = flush(batch)
		if err != nil {
			return felt.Felt{}, batch, stack, err
		}
	}
	return lastHash, batch, stack, nil
}

func pushFrame(stack []dfsFrame, oldPath trie.BitArray) []dfsFrame {
	n := len(stack)
	stack = stack[:n+1]
	stack[n] = dfsFrame{oldPath: oldPath}
	return stack
}

const maxOldKeySize = 1 + 32 + 1 + 32

func encodeOldPath(path *trie.BitArray, dst []byte) int {
	pathLen := path.Len()
	b := path.Bytes()
	activeBytes := (uint(pathLen) + 7) / 8
	dst[0] = pathLen
	copy(dst[1:], b[32-activeBytes:])
	return int(activeBytes) + 1
}

// readNode loads the deprecated-format node at (prefix, oldPath) and returns
// its parsed fields. The caller assigns the parsed values into its own state
// (e.g. a DFS stack frame); this function does not mutate any input.
func readNode(r db.KeyValueReader, prefix []byte, oldPath *trie.BitArray) (parsedNode, error) {
	var arr [maxOldKeySize]byte
	n := copy(arr[:], prefix)
	n += encodeOldPath(oldPath, arr[n:])
	var node parsedNode
	err := r.Get(arr[:n], func(val []byte) error {
		var perr error
		node, perr = parseNodeData(val)
		return perr
	})
	return node, err
}

// parseNodeData decodes a deprecated-format node's raw bytes:
// felt(value) [ BitArray(left) BitArray(right) [ felt felt ] ]
// The trailing left/right hashes are ignored — the migrator re-derives hashes
// itself — so only the fields it actually needs are returned.
func parseNodeData(data []byte) (parsedNode, error) {
	var n parsedNode
	if len(data) < felt.Bytes {
		return n, fmt.Errorf("trie: node data too short (%d bytes)", len(data))
	}
	n.value = felt.FromBytes[felt.Felt](data[:felt.Bytes])
	data = data[felt.Bytes:]
	if len(data) == 0 {
		n.isLeaf = true
		return n, nil
	}
	if err := n.left.UnmarshalBinary(data); err != nil {
		return n, fmt.Errorf("trie: unmarshalling left path: %w", err)
	}
	data = data[n.left.EncodedLen():]
	if err := n.right.UnmarshalBinary(data); err != nil {
		return n, fmt.Errorf("trie: unmarshalling right path: %w", err)
	}
	return n, nil
}

func processLeaf(
	path trieutils.Path,
	value *felt.Felt,
	sched *hashScheduler,
	batch db.Batch,
) error {
	var buf [trieutils.MaxNodeKeySize + valueNodeBlobSize]byte
	keyLen := trieutils.EncodeNodeKey(buf[:], sched.bucket, &sched.owner, &path, true)
	blob := encodeValueNode(value)
	copy(buf[keyLen:], blob[:])
	return batch.Put(buf[:keyLen], buf[keyLen:keyLen+valueNodeBlobSize])
}

func processBinary(
	parentPath trieutils.Path,
	left, right *trie.BitArray,
	leftChildHash, rightChildHash felt.Felt,
	sched *hashScheduler,
	batch db.Batch,
) error {
	leftSeg := compressedSegment(left, parentPath.Len())
	rightSeg := compressedSegment(right, parentPath.Len())
	return sched.schedule(&edgeHashJob{
		leftChildHash:  leftChildHash,
		leftSeg:        leftSeg,
		rightChildHash: rightChildHash,
		rightSeg:       rightSeg,
		parentPath:     parentPath,
	}, batch)
}

func writeRootEdge(
	rootPath *trie.BitArray,
	childHash felt.Felt,
	sched *hashScheduler,
	batch db.Batch,
) error {
	seg := toNewPath(rootPath)
	var buf [edgeNodeMaxSize]byte
	n := encodeEdgeNodeInto(buf[:], &childHash, &seg)
	return trieutils.WriteNodeByPath(batch, sched.bucket, &sched.owner, &trieutils.Path{}, false, buf[:n])
}

func oldTriePrefix(desc TrieDesc) []byte {
	if desc.OldBucket == db.ContractStorage {
		ownerFelt := felt.Felt(desc.Owner)
		ownerBytes := ownerFelt.Bytes()
		return desc.OldBucket.Key(ownerBytes[:])
	}
	return desc.OldBucket.Key()
}
