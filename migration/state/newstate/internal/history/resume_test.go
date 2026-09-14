package history

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// TestAddressSeqEscapesResumePoint pins the mechanism resume rests on: the
// source writes an address into progress before yielding it, so a refused
// yield leaves progress on the first address never handed out.
func TestAddressSeqEscapesResumePoint(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	addrs := []felt.Felt{
		felt.FromUint64[felt.Felt](1),
		felt.FromUint64[felt.Felt](2),
		felt.FromUint64[felt.Felt](3),
	}
	for _, addr := range addrs {
		require.NoError(t, state.WriteContract(memDB, &addr, felt.Zero, felt.Zero, 1))
	}
	bytesOf := func(addr felt.Felt) [felt.Bytes]byte { return addr.Bytes() }

	// Accept the first address and refuse the second, which is what the
	// pipeline's source does when the context ends mid-walk: a refused address
	// was never handed out, so it is the resume point.
	var progress [felt.Bytes]byte
	seq, sourceErr := addressSeq(memDB, &progress)
	seen := 0
	for range seq {
		seen++
		if seen == 2 {
			break
		}
	}
	require.NoError(t, sourceErr())
	require.Equal(t, bytesOf(addrs[1]), progress, "the refused address is the resume point")

	// Resume from there and let it run out: everything from the resume point
	// on is yielded. Completion is the pipeline's IsDone, not a value here;
	// progress simply holds the last address handed out.
	seq, sourceErr = addressSeq(memDB, &progress)
	var got []felt.Address
	for addr := range seq {
		got = append(got, addr)
	}
	require.NoError(t, sourceErr())
	require.Equal(t, []felt.Address{felt.Address(addrs[1]), felt.Address(addrs[2])}, got)
	require.Equal(t, bytesOf(addrs[2]), progress)
}

// cancelAfter wraps an ingestor and cancels the context once n contracts have
// been started, stopping a phase in the middle of its walk.
type cancelAfter struct {
	pipeline.State[felt.Address, common.Task]
	cancel  context.CancelFunc
	n       int64
	started atomic.Int64
}

func (c *cancelAfter) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	if c.started.Add(1) == c.n {
		c.cancel()
	}
	return c.State.Run(index, addr, outputs)
}

// TestRunPhaseDrainsHandedOutContractsOnCancel is the guarantee the resume
// point depends on: cancellation stops the source, but every contract it had
// already handed out is finished and committed. So every address before the
// recorded progress must be fully migrated, and resuming from it must complete
// the phase.
func TestRunPhaseDrainsHandedOutContractsOnCancel(t *testing.T) {
	memDB := memory.New()
	t.Cleanup(func() { memDB.Close() })

	const contracts = 200
	addrs := make([]felt.Felt, contracts)
	for i := range addrs {
		addrs[i] = felt.FromUint64[felt.Felt](uint64(i + 1))
		require.NoError(t, state.WriteContract(
			memDB, &addrs[i], felt.FromUint64[felt.Felt](7), felt.Zero, 1,
		))
		require.NoError(t, core.WriteDeprecatedContractNonceHistory(memDB, &addrs[i], &felt.Zero, 200))
	}
	logger := log.NewNopZapLogger()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var progress [felt.Bytes]byte
	interrupted := phase{
		name:       "nonce",
		deprecated: db.DeprecatedContractNonceHistory,
		newIngestor: func(
			sem semaphore.ResourceSemaphore[db.Batch], r db.KeyValueReader,
		) pipeline.State[felt.Address, common.Task] {
			return &cancelAfter{State: newNonceIngestor(sem, r), cancel: cancel, n: 5}
		},
	}
	done, err := runPhase(ctx, memDB, logger, &interrupted, &progress)
	require.NoError(t, err)
	require.False(t, done, "the walk must have been interrupted")

	handedOut := 0
	for i := range addrs {
		addrBytes := addrs[i].Bytes()
		if bytes.Compare(addrBytes[:], progress[:]) >= 0 {
			break
		}
		handedOut++
		require.True(t, hasKey(t, memDB, db.ContractNonceHistoryKey(&addrs[i])),
			"contract %d was handed out before the resume point but is not committed", i+1)
	}
	require.GreaterOrEqual(t, handedOut, 5, "the five that triggered the cancel were all handed out")
	require.Equal(t, contracts, countKeys(t, memDB, db.DeprecatedContractNonceHistory),
		"deprecated rows survive until the phase completes")

	// Resume from the recorded address with the plain ingestor and finish.
	resumed := interrupted
	resumed.newIngestor = func(
		sem semaphore.ResourceSemaphore[db.Batch], r db.KeyValueReader,
	) pipeline.State[felt.Address, common.Task] {
		return newNonceIngestor(sem, r)
	}
	done, err = runPhase(context.Background(), memDB, logger, &resumed, &progress)
	require.NoError(t, err)
	require.True(t, done)
	for i := range addrs {
		require.True(t, hasKey(t, memDB, db.ContractNonceHistoryKey(&addrs[i])),
			"contract %d after resume", i+1)
	}
	require.Zero(t, countKeys(t, memDB, db.DeprecatedContractNonceHistory))
}

func hasKey(t *testing.T, r db.KeyValueReader, prefix []byte) bool {
	t.Helper()
	it, err := r.NewIterator(prefix, true)
	require.NoError(t, err)
	defer it.Close()
	return it.First()
}

func countKeys(t *testing.T, r db.KeyValueReader, bucket db.Bucket) int {
	t.Helper()
	it, err := r.NewIterator(bucket.Key(), true)
	require.NoError(t, err)
	defer it.Close()
	n := 0
	for valid := it.First(); valid; valid = it.Next() {
		n++
	}
	return n
}
