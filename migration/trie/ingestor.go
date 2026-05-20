package trie

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
)

const (
	dfsStackCap   = 251
	maxOldKeySize = 1 + 32 + 1 + 32
)

type traverseStackState uint8

const (
	readNodeState traverseStackState = iota
	leftSubtreeDoneState
	rightSubtreeDoneState
)

type task struct {
	batch db.Batch
	tries int
}

type ingestor struct {
	ctx            context.Context
	database       db.KeyValueReader
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	pool           *hashWorkerPool
	tasks          [IngestorCount]task
	dfsStacks      [IngestorCount][]dfsFrame
}

func newIngestor(
	ctx context.Context,
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	pool *hashWorkerPool,
) *ingestor {
	in := &ingestor{
		ctx:            ctx,
		database:       database,
		batchSemaphore: batchSemaphore,
		pool:           pool,
	}
	for i := range IngestorCount {
		in.tasks[i].batch = batchSemaphore.GetBlocking()
		in.dfsStacks[i] = make([]dfsFrame, 0, dfsStackCap)
	}
	return in
}

func (i *ingestor) Run(index int, desc TrieDesc, outputs chan<- task) error {
	done, err := hasDestRoot(i.database, desc.NewBucket, &desc.Owner)
	if err != nil {
		return fmt.Errorf("hasDestRoot(%v, %x): %w", desc.NewBucket, desc.Owner, err)
	}
	if done {
		return nil
	}

	t := &i.tasks[index]

	i.dfsStacks[index], err = migrateTrie(
		i.database, desc, i.pool, t, i.flush, outputs, i.dfsStacks[index],
	)
	if err != nil {
		return err
	}

	t.tries++
	return i.flush(t, outputs)
}

func (i *ingestor) Done(index int, outputs chan<- task) error {
	select {
	case <-i.ctx.Done():
		return i.ctx.Err()
	case outputs <- i.tasks[index]:
	}
	return nil
}

// flush rotates t.batch if it's hit target size: sends the current batch
// downstream and acquires a fresh one. Mutates t in place.
func (i *ingestor) flush(t *task, outputs chan<- task) error {
	if t.batch.Size() < targetBatchByteSize {
		return nil
	}
	select {
	case <-i.ctx.Done():
		return i.ctx.Err()
	case outputs <- task{batch: t.batch, tries: t.tries}:
	}
	t.tries = 0
	t.batch = i.batchSemaphore.GetBlocking()
	return nil
}

// migrateTrie writes the new-format representation of a single deprecated
// trie into t.batch. It walks the deprecated trie in DFS order, emitting
// value/binary/edge nodes via the hashScheduler, and calls flush once per
// step to rotate the batch when it hits target size.
func migrateTrie(
	r db.KeyValueReader,
	desc TrieDesc,
	pool *hashWorkerPool,
	t *task,
	flush func(*task, chan<- task) error,
	outputs chan<- task,
	stack []dfsFrame,
) ([]dfsFrame, error) {
	stack = stack[:0]
	if desc.RootPath == nil {
		return stack, nil
	}
	parallelDispatch := desc.NodeCount >= SmallTrieThreshold
	prefix := oldTriePrefix(desc)
	sched := newHashScheduler(desc.HashFn, parallelDispatch, desc.NewBucket, desc.Owner, pool)

	rootHash, stack, err := traverse(r, prefix, desc.RootPath, sched, t, flush, outputs, stack)
	if err != nil {
		return stack, err
	}
	if err := sched.sync(t.batch); err != nil {
		return stack, err
	}
	if desc.RootPath.Len() > 0 {
		if err := writeRootEdge(desc.RootPath, rootHash, sched, t.batch); err != nil {
			return stack, err
		}
	}
	return stack, nil
}

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
	t *task,
	flush func(*task, chan<- task) error,
	outputs chan<- task,
	stack []dfsFrame,
) (felt.Felt, []dfsFrame, error) {
	stack = stack[:1]
	stack[0] = dfsFrame{oldPath: *start}
	var lastHash felt.Felt
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		switch top.state {
		case readNodeState:
			parsed, err := readNode(r, prefix, &top.oldPath)
			if err != nil {
				return felt.Felt{}, stack, err
			}
			top.value = parsed.value
			top.left = parsed.left
			top.right = parsed.right
			top.isLeaf = parsed.isLeaf
			if top.isLeaf {
				newPath := toNewPath(&top.oldPath)
				if err := processLeaf(newPath, &top.value, sched, t.batch); err != nil {
					return felt.Felt{}, stack, err
				}
				lastHash = top.value
				stack = stack[:len(stack)-1]
			} else {
				top.state = leftSubtreeDoneState
				stack = pushFrame(stack, top.left)
			}
		case leftSubtreeDoneState:
			top.leftHash = lastHash
			top.state = rightSubtreeDoneState
			stack = pushFrame(stack, top.right)
		case rightSubtreeDoneState:
			newPath := toNewPath(&top.oldPath)
			if err := processBinary(
				newPath,
				&top.left,
				&top.right,
				top.leftHash,
				lastHash,
				sched,
				t.batch,
			); err != nil {
				return felt.Felt{}, stack, err
			}
			lastHash = top.value
			stack = stack[:len(stack)-1]
		}
		if err := flush(t, outputs); err != nil {
			return felt.Felt{}, stack, err
		}
	}
	return lastHash, stack, nil
}

func pushFrame(stack []dfsFrame, oldPath trie.BitArray) []dfsFrame {
	n := len(stack)
	stack = stack[:n+1]
	stack[n] = dfsFrame{oldPath: oldPath}
	return stack
}

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
	return trieutils.WriteNodeByPath(
		batch,
		sched.bucket,
		&sched.owner,
		&trieutils.Path{},
		false,
		buf[:n],
	)
}

func oldTriePrefix(desc TrieDesc) []byte {
	if desc.OldBucket == db.ContractStorage {
		ownerFelt := felt.Felt(desc.Owner)
		ownerBytes := ownerFelt.Bytes()
		return desc.OldBucket.Key(ownerBytes[:])
	}
	return desc.OldBucket.Key()
}
