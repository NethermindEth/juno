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

const maxOldKeySize = 1 + 32 + 1 + 32

type task struct {
	batch db.Batch
	tries int
	nodes int
}

type ingestor struct {
	ctx            context.Context
	database       db.KeyValueReader
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	pool           *hashWorkerPool
	tasks          [IngestorCount]task
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
	}
	return in
}

func (i *ingestor) Run(index int, desc TrieDesc, outputs chan<- task) error {
	done, err := hasDestRoot(i.database, desc.TrieBucket, &desc.Owner)
	if err != nil {
		return fmt.Errorf("hasDestRoot(%v, %x): %w", desc.TrieBucket, desc.Owner, err)
	}

	t := &i.tasks[index]
	if done {
		// Already migrated — credit the counts so progress display reaches 100% on resume.
		t.tries++
		t.nodes += desc.NodeCount
		return i.flush(t, outputs)
	}

	if err := migrateTrie(i.database, desc, i.pool, t, i.flush, outputs); err != nil {
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
	case outputs <- task{batch: t.batch, tries: t.tries, nodes: t.nodes}:
	}
	t.tries = 0
	t.nodes = 0
	t.batch = i.batchSemaphore.GetBlocking()
	return nil
}

// migrateTrie writes the new-format representation of a single deprecated
// trie into t.batch. It walks the deprecated trie via DFS, emits value /
// binary / edge nodes through the hashScheduler, and calls flush at each
// step to rotate the batch when it hits target size.
func migrateTrie(
	r db.KeyValueReader,
	desc TrieDesc,
	pool *hashWorkerPool,
	t *task,
	flush func(*task, chan<- task) error,
	outputs chan<- task,
) error {
	if desc.RootPath == nil {
		return nil
	}
	parallelDispatch := desc.NodeCount >= SmallTrieThreshold
	prefix := deprecatedTriePrefix(desc)
	sched := newHashScheduler(desc.HashFn, parallelDispatch, desc.TrieBucket, desc.Owner, pool)

	rootHash, err := traverse(r, prefix, *desc.RootPath, sched, t, flush, outputs)
	if err != nil {
		return err
	}
	if err := sched.sync(t.batch); err != nil {
		return err
	}
	if desc.RootPath.Len() > 0 {
		if err := writeRootEdge(desc.RootPath, rootHash, sched, t.batch); err != nil {
			return err
		}
	}
	return nil
}

// traverse walks the deprecated trie rooted at oldPath in DFS order, writing
// the new-format equivalents into t.batch via sched. Returns the hash of
// the visited subtree — used by the caller to wire up parent binary nodes.
func traverse(
	r db.KeyValueReader,
	prefix []byte,
	oldPath trie.BitArray,
	sched *hashScheduler,
	t *task,
	flush func(*task, chan<- task) error,
	outputs chan<- task,
) (felt.Felt, error) {
	parsed, err := readNode(r, prefix, &oldPath)
	if err != nil {
		return felt.Felt{}, err
	}
	t.nodes++

	if parsed.isLeaf {
		newPath := toNewPath(&oldPath)
		if err := processLeaf(newPath, &parsed.value, sched, t.batch); err != nil {
			return felt.Felt{}, err
		}
		if err := flush(t, outputs); err != nil {
			return felt.Felt{}, err
		}
		return parsed.value, nil
	}

	leftHash, err := traverse(r, prefix, parsed.left, sched, t, flush, outputs)
	if err != nil {
		return felt.Felt{}, err
	}
	rightHash, err := traverse(r, prefix, parsed.right, sched, t, flush, outputs)
	if err != nil {
		return felt.Felt{}, err
	}

	newPath := toNewPath(&oldPath)
	if err := processBinary(
		newPath, &parsed.left, &parsed.right, leftHash, rightHash, sched, t.batch,
	); err != nil {
		return felt.Felt{}, err
	}
	if err := flush(t, outputs); err != nil {
		return felt.Felt{}, err
	}
	return parsed.value, nil
}

func encodeOldPath(path *trie.BitArray, dst []byte) int {
	pathLen := path.Len()
	b := path.Bytes()
	activeBytes := (uint(pathLen) + 7) / 8
	dst[0] = pathLen
	copy(dst[1:], b[32-activeBytes:])
	return int(activeBytes) + 1
}

type parsedNode struct {
	value  felt.Felt
	left   trie.BitArray
	right  trie.BitArray
	isLeaf bool
}

// readNode loads the deprecated-format node at (prefix, oldPath) and returns
// its parsed fields. The caller owns the result; this function does not
// mutate any input.
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

func deprecatedTriePrefix(desc TrieDesc) []byte {
	switch desc.DeprecatedTrieBucket {
	case db.ClassesTrie, db.StateTrie:
		return desc.DeprecatedTrieBucket.Key()
	case db.ContractStorage:
		ownerBytes := desc.Owner.Bytes()
		return desc.DeprecatedTrieBucket.Key(ownerBytes[:])
	default:
		panic(fmt.Sprintf(
			"unexpected deprecated trie bucket %v",
			desc.DeprecatedTrieBucket,
		))
	}
}
