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
		desc.NewBucket, &desc.Owner, m.pool)

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
	readNode traverseStackState = iota
	leftSubtreeDone
	rightSubtreeDone
)

type dfsFrame struct {
	oldPath     trie.BitArray
	left        trie.BitArray
	right       trie.BitArray
	value       felt.Felt
	leftHash    felt.Felt
	isLeaf      bool
	state       traverseStackState
	binaryDepth uint8
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
		case readNode:
			if err := parseNodeInto(r, prefix, top); err != nil {
				return felt.Felt{}, batch, stack, err
			}
			newPath := toNewPath(&top.oldPath)
			if top.isLeaf {
				if err := processLeaf(newPath, &top.value, sched, batch); err != nil {
					return felt.Felt{}, batch, stack, err
				}
				lastHash = top.value
				stack = stack[:len(stack)-1]
			} else {
				left := top.left
				top.state = leftSubtreeDone
				stack = pushFrame(stack, left)
			}
		case leftSubtreeDone:
			top.leftHash = lastHash
			right := top.right
			top.state = rightSubtreeDone
			stack = pushFrame(stack, right)
		case rightSubtreeDone:
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
		batch = flush(batch)
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

func parseNodeInto(r db.KeyValueReader, prefix []byte, frame *dfsFrame) error {
	var arr [maxOldKeySize]byte
	n := copy(arr[:], prefix)
	n += encodeOldPath(&frame.oldPath, arr[n:])
	return r.Get(arr[:n], func(val []byte) error {
		return parseNodeData(val, &frame.value, &frame.left, &frame.right, &frame.isLeaf)
	})
}

func parseNodeData(data []byte, value *felt.Felt, left, right *trie.BitArray, isLeaf *bool) error {
	if len(data) < felt.Bytes {
		return fmt.Errorf("trie: node data too short (%d bytes)", len(data))
	}
	*value = felt.FromBytes[felt.Felt](data[:felt.Bytes])
	data = data[felt.Bytes:]
	if len(data) == 0 {
		*isLeaf = true
		return nil
	}
	*isLeaf = false
	if err := left.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("trie: unmarshalling left path: %w", err)
	}
	data = data[left.EncodedLen():]
	if err := right.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("trie: unmarshalling right path: %w", err)
	}
	return nil
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
	leftFull := toNewPath(left)
	rightFull := toNewPath(right)
	var leftEdgePath, rightEdgePath trieutils.Path
	leftEdgePath.MSBs(&leftFull, parentPath.Len()+1)
	rightEdgePath.MSBs(&rightFull, parentPath.Len()+1)
	return sched.schedule(edgeHashJob{
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
	blob := encodeEdgeNode(&childHash, &seg)
	return trieutils.WriteNodeByPath(batch, sched.bucket, &sched.owner, &trieutils.Path{}, false, blob)
}

func oldTriePrefix(desc TrieDesc) []byte {
	if desc.OldBucket == db.ContractStorage {
		ownerFelt := felt.Felt(desc.Owner)
		ownerBytes := ownerFelt.Bytes()
		return desc.OldBucket.Key(ownerBytes[:])
	}
	return desc.OldBucket.Key()
}
