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
	done, err := rootProcessed(i.database, desc.TrieBucket, &desc.Owner)
	if err != nil {
		return fmt.Errorf("rootProcessed(%v, %x): %w", desc.TrieBucket, desc.Owner, err)
	}

	t := &i.tasks[index]
	if done {
		// Already migrated — credit the counts so progress display reaches 100% on resume.
		t.tries++
		t.nodes += desc.NodeCount
		return i.flush(t, outputs)
	}

	if err := i.migrateTrie(t, desc, outputs); err != nil {
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

// migrateTrie reads one deprecated trie and writes its equivalent into the
// new layout. Three things differ between the formats: how nodes are keyed
// on disk, how nodes are encoded, and how path compression is expressed.
//
// On-disk keying
// --------------
// Both layouts share a common prefix; only the suffix differs:
//
//	common (both)              suffix
//	─────────────              ─────────────────────────────────────────
//	bucket [|| owner]    →     path-length-byte || path-bytes     (deprecated)
//	                     →     nodeType-byte || path-length-byte || path-bytes
//	                                                              (new)
//
// The owner is present only for storage tries. The new layout's extra
// nodeType byte splits leaves from internal nodes into two index slices
// within the same bucket — the new-state lookups use this to short-circuit
// between leaf reads and internal-node traversals.
//
// Node encoding
// -------------
// Both layouts are byte streams. The deprecated format keeps each node
// self-contained — internal binary nodes embed the compressed paths to
// their children inline:
//
//	leaf       value
//	binary     value || left-child-path || right-child-path
//	           [|| left-hash || right-hash, optional cache, ignored here]
//
// "value" is the node's own Starknet trie hash, or the stored value when
// the node is a leaf.
//
// The new format gives each node an explicit type tag and moves path
// compression into separate edge nodes:
//
//	value      value
//	binary     0x01 || left-edge-hash || right-edge-hash
//	edge       0x02 || child-hash || encoded-path-segment
//
// Path compression
// ----------------
// This is the key structural change. The deprecated format compresses
// paths inside the parent binary node (via its embedded child-path
// fields). The new format moves compression into dedicated edge nodes
// sitting between binary nodes and their children:
//
//	deprecated:   binary ──────── child-path ────────► child
//	new:          binary ──► edge ──► child
//
// The deprecated root marker — a single entry at the bare bucket prefix
// recording the root's path — disappears in the new layout. Whatever the
// deprecated root embedded becomes either a direct binary/leaf at the
// empty path or, when the deprecated root path was itself non-empty, an
// edge node at the empty path that points "down" to the real root.
//
// Traversal
// ---------
// The migrator walks the deprecated trie depth-first, decoding one node at
// a time. A leaf becomes a value node at the same path. An internal binary
// node, after both subtrees have been visited, becomes a binary node plus
// up to two edge nodes (one per non-empty child segment). If the trie's
// stored root path is itself non-empty — meaning the deprecated root
// embeds a compression — a single edge node at the empty path is written
// after the traversal completes, replacing the root marker.
//
// Hashes
// ------
// Starknet trie hashes:
//
//	leaf       value
//	binary     hashFn(left-edge-hash, right-edge-hash)
//	edge       hashFn(child-hash, path-segment-as-felt) + segment-length
//
// Zero-length edges short-circuit to the bare child-hash — the convention
// for absent edges. Class tries hash with Poseidon; contract and storage
// tries with Pedersen.
//
// For small tries every edge hash is computed inline. Above the threshold,
// edge-hash jobs are batched and dispatched to a worker pool for parallel
// computation; the scheduler preserves the original job order so the
// persisted bytes are byte-identical to a natively-built trie2.
//
// In-flight batches flush at target size; cancellation is observed at
// every flush and every channel send.
func (i *ingestor) migrateTrie(t *task, desc TrieDesc, outputs chan<- task) error {
	if desc.NodeCount == 0 {
		return nil
	}
	parallelDispatch := desc.NodeCount >= SmallTrieThreshold
	prefix := deprecatedTriePrefix(desc)
	sched := newHashScheduler(desc.HashFn, parallelDispatch, desc.TrieBucket, desc.Owner, i.pool)

	rootHash, err := i.traverse(t, outputs, prefix, *desc.RootPath, sched)
	if err != nil {
		return err
	}
	if err := sched.sync(t.batch); err != nil {
		return err
	}
	if desc.RootPath.Len() > 0 {
		if err := writeRootEdgeNode(desc.RootPath, rootHash, sched, t.batch); err != nil {
			return err
		}
	}
	return nil
}

func (i *ingestor) traverse(
	t *task,
	outputs chan<- task,
	prefix []byte,
	oldPath trie.BitArray,
	sched *hashScheduler,
) (felt.Felt, error) {
	parsed, err := readNode(i.database, prefix, &oldPath)
	if err != nil {
		return felt.Felt{}, err
	}
	t.nodes++

	if parsed.isLeaf {
		newPath := toNewPath(&oldPath)
		if err := writeLeafNode(newPath, &parsed.value, sched, t.batch); err != nil {
			return felt.Felt{}, err
		}
		if err := i.flush(t, outputs); err != nil {
			return felt.Felt{}, err
		}
		return parsed.value, nil
	}

	leftHash, err := i.traverse(t, outputs, prefix, parsed.left, sched)
	if err != nil {
		return felt.Felt{}, err
	}
	rightHash, err := i.traverse(t, outputs, prefix, parsed.right, sched)
	if err != nil {
		return felt.Felt{}, err
	}

	newPath := toNewPath(&oldPath)
	if err := processBinary(
		newPath, &parsed.left, &parsed.right, leftHash, rightHash, sched, t.batch,
	); err != nil {
		return felt.Felt{}, err
	}
	if err := i.flush(t, outputs); err != nil {
		return felt.Felt{}, err
	}
	return parsed.value, nil
}

func rootProcessed(r db.KeyValueReader, newBucket db.Bucket, owner *felt.Address) (bool, error) {
	var emptyPath trieutils.Path
	var buf [maxNodeKeySize]byte

	n := encodeNodeKey(buf[:], newBucket, owner, &emptyPath, false)
	if exists, err := r.Has(buf[:n]); err != nil || exists {
		return exists, err
	}
	n = encodeNodeKey(buf[:], newBucket, owner, &emptyPath, true)
	return r.Has(buf[:n])
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

func readNode(r db.KeyValueReader, prefix []byte, oldPath *trie.BitArray) (parsedNode, error) {
	var arr [maxNodeKeySize]byte
	n := copy(arr[:], prefix)
	n += encodeOldPath(oldPath, arr[n:])
	var node parsedNode
	err := r.Get(arr[:n], node.UnmarshalBinary)
	return node, err
}

func (n *parsedNode) UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return fmt.Errorf("trie: node data too short (%d bytes)", len(data))
	}
	n.value = felt.FromBytes[felt.Felt](data[:felt.Bytes])
	data = data[felt.Bytes:]
	if len(data) == 0 {
		n.isLeaf = true
		return nil
	}
	if err := n.left.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("trie: unmarshalling left path: %w", err)
	}
	data = data[n.left.EncodedLen():]
	if err := n.right.UnmarshalBinary(data); err != nil {
		return fmt.Errorf("trie: unmarshalling right path: %w", err)
	}
	return nil
}

func writeLeafNode(
	path trieutils.Path,
	value *felt.Felt,
	sched *hashScheduler,
	batch db.Batch,
) error {
	var buf [maxNodeKeySize + valueNodeBlobSize]byte
	keyLen := encodeNodeKey(buf[:], sched.bucket, &sched.owner, &path, true)
	blob := value.Bytes()
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

func writeRootEdgeNode(
	rootPath *trie.BitArray,
	childHash felt.Felt,
	sched *hashScheduler,
	batch db.Batch,
) error {
	seg := toNewPath(rootPath)
	var buf [edgeNodeMaxSize]byte
	n := encodeEdgeNode(buf[:], &childHash, &seg)
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
