package trie

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSimpleJob(parent *trieutils.Path, leftChild, rightChild *felt.Felt) *edgeHashJob {
	return &edgeHashJob{
		leftChildHash:  *leftChild,
		rightChildHash: *rightChild,
		parentPath:     *parent,
	}
}

func TestHashScheduler_InlineWritesNodeToBatch(t *testing.T) {
	memDB := memory.New()
	batch := memDB.NewBatch()

	sched := newHashScheduler(crypto.Pedersen, false, db.ClassTrie, felt.Address{}, nil)
	parent := makeNewPath(3, 0b101)
	left := felt.NewFromUint64[felt.Felt](1)
	right := felt.NewFromUint64[felt.Felt](2)

	require.NoError(t, sched.schedule(makeSimpleJob(&parent, left, right), batch))
	require.NoError(t, batch.Write())

	blob := encodeBinaryNode(left, right)
	val, err := trieutils.GetNodeByPath(memDB, db.ClassTrie, &felt.Address{}, &parent, false)
	require.NoError(t, err)
	assert.Equal(t, blob[:], val)
}

func TestHashScheduler_BatchedMatchesInlinePedersen(t *testing.T) {
	testBatchedMatchesInline(t, crypto.Pedersen, 200)
}

func TestHashScheduler_BatchedMatchesInlinePoseidon(t *testing.T) {
	testBatchedMatchesInline(t, crypto.Poseidon, 200)
}

func testBatchedMatchesInline(t *testing.T, hashFn crypto.HashFn, n int) {
	t.Helper()

	pool := newHashWorkerPool()
	t.Cleanup(pool.close)

	jobs := make([]edgeHashJob, n)
	for i := range n {
		l := felt.NewFromUint64[felt.Felt](uint64(i*2 + 1))
		r := felt.NewFromUint64[felt.Felt](uint64(i*2 + 2))
		path := makeNewPath(uint8(i%250), uint64(i))
		jobs[i] = *makeSimpleJob(&path, l, r)
	}

	inlineDB := memory.New()
	{
		batch := inlineDB.NewBatch()
		sched := newHashScheduler(hashFn, false, db.ClassTrie, felt.Address{}, nil)
		for i := range jobs {
			require.NoError(t, sched.schedule(&jobs[i], batch))
		}
		require.NoError(t, batch.Write())
	}

	batchDB := memory.New()
	{
		batch := batchDB.NewBatch()
		sched := newHashScheduler(hashFn, true, db.ClassTrie, felt.Address{}, pool)
		for i := range jobs {
			require.NoError(t, sched.schedule(&jobs[i], batch))
		}
		require.NoError(t, sched.sync(batch))
		require.NoError(t, batch.Write())
	}

	for i := range jobs {
		job := &jobs[i]
		inlineVal, err := trieutils.GetNodeByPath(
			inlineDB,
			db.ClassTrie,
			&felt.Address{},
			&job.parentPath,
			false,
		)
		require.NoError(t, err, "path %d missing in inline", i)
		batchVal, err := trieutils.GetNodeByPath(
			batchDB,
			db.ClassTrie,
			&felt.Address{},
			&job.parentPath,
			false,
		)
		require.NoError(t, err, "path %d missing in batched", i)
		assert.Equal(t, inlineVal, batchVal, "mismatch at path %d", i)
	}
}

func TestHashScheduler_AutoFlushAtBatchSize(t *testing.T) {
	pool := newHashWorkerPool()
	t.Cleanup(pool.close)

	memDB := memory.New()
	batch := memDB.NewBatch()

	sched := newHashScheduler(crypto.Pedersen, true, db.ClassTrie, felt.Address{}, pool)
	for i := range parallelHashBatchSize {
		path := makeNewPath(251, uint64(i+1))
		leftChildHash := felt.NewFromUint64[felt.Felt](uint64(i + 1))
		rightChildHash := felt.NewFromUint64[felt.Felt](uint64(i + 2))
		job := makeSimpleJob(&path, leftChildHash, rightChildHash)
		require.NoError(t, sched.schedule(job, batch))
	}
	// After filling a full batch, jobs are handed off to pool; local slice is reset
	assert.Empty(t, sched.jobs, "jobs should be empty after auto-flush")
	// Drain in-flight batch before pool.close() to avoid send-on-closed-channel
	require.NoError(t, sched.sync(batch))
}

func TestHashScheduler_SingleJobDispatchesCorrectly(t *testing.T) {
	pool := newHashWorkerPool()
	t.Cleanup(pool.close)

	memDB := memory.New()
	batch := memDB.NewBatch()

	sched := newHashScheduler(crypto.Pedersen, true, db.ClassTrie, felt.Address{}, pool)
	parent := makeNewPath(4, 0b1010)
	l, r := felt.NewFromUint64[felt.Felt](7), felt.NewFromUint64[felt.Felt](13)
	job := makeSimpleJob(&parent, l, r)

	require.NoError(t, sched.schedule(job, batch))
	require.NoError(t, sched.sync(batch))
	require.NoError(t, batch.Write())

	blob := encodeBinaryNode(l, r)
	val, err := trieutils.GetNodeByPath(memDB, db.ClassTrie, &felt.Address{}, &parent, false)
	require.NoError(t, err)
	assert.Equal(t, blob[:], val)
}

func TestHashScheduler_LargeBatchDispatchesCorrectly(t *testing.T) {
	testBatchedMatchesInline(t, crypto.Pedersen, parallelHashBatchSize*3)
}
