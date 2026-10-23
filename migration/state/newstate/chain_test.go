package newstate_test

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core"
	//nolint:staticcheck,nolintlint // old state layout
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie2/triedb"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/require"
)

// chainAddrs are the contracts randomStateDiffs touches, system contracts first.
var chainAddrs = []felt.Felt{
	felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2),
	felt.FromUint64[felt.Felt](0x100), felt.FromUint64[felt.Felt](0x101),
	felt.FromUint64[felt.Felt](0x102), felt.FromUint64[felt.Felt](0x103),
	felt.FromUint64[felt.Felt](0x104), felt.FromUint64[felt.Felt](0x105),
}

var chainSlots = []felt.Felt{
	felt.FromUint64[felt.Felt](1), felt.FromUint64[felt.Felt](2), felt.FromUint64[felt.Felt](3),
}

// randomStateDiffs deploys, replaces classes, bumps nonces and writes storage at random.
func randomStateDiffs(seed int64, blocks int) []*core.StateDiff {
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // deterministic fixture
	deployed := map[felt.Felt]bool{}
	nonces := map[felt.Felt]uint64{}
	randomFelt := func(base, n int) *felt.Felt {
		v := felt.FromUint64[felt.Felt](uint64(base + r.Intn(n)))
		return &v
	}

	diffs := make([]*core.StateDiff, blocks)
	for height := range diffs {
		d := &core.StateDiff{
			StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{},
			Nonces:            map[felt.Felt]*felt.Felt{},
			DeployedContracts: map[felt.Felt]*felt.Felt{},
			ReplacedClasses:   map[felt.Felt]*felt.Felt{},
		}
		for _, addr := range chainAddrs {
			system := state.IsSystemContract(&addr)
			if !system && !deployed[addr] {
				if r.Intn(4) == 0 {
					d.DeployedContracts[addr] = randomFelt(0xC0, 4)
					deployed[addr] = true
				}
				continue
			}
			if !system && r.Intn(6) == 0 {
				d.ReplacedClasses[addr] = randomFelt(0xC0, 4)
			}
			if !system && r.Intn(3) == 0 {
				nonces[addr]++
				n := felt.FromUint64[felt.Felt](nonces[addr])
				d.Nonces[addr] = &n
			}
			writes := map[felt.Felt]*felt.Felt{}
			for range r.Intn(3) {
				writes[chainSlots[r.Intn(len(chainSlots))]] = randomFelt(0, 6)
			}
			if len(writes) > 0 {
				d.StorageDiffs[addr] = writes
			}
		}
		diffs[height] = d
	}
	return diffs
}

// storeDeprecatedChain applies diffs through the deprecated state and writes block data.
func storeDeprecatedChain(t *testing.T, database db.KeyValueStore, diffs []*core.StateDiff) {
	t.Helper()
	var header *core.Header
	for height, d := range diffs {
		blockNum := uint64(height)
		txn := database.NewIndexedBatch() //nolint:staticcheck // the deprecated state needs it
		st := deprecatedstate.New(txn)
		oldRoot, err := st.Commitment("")
		require.NoError(t, err)
		header = &core.Header{Number: blockNum, Hash: felt.NewFromUint64[felt.Felt](blockNum + 1)}
		su := &core.StateUpdate{
			BlockHash: header.Hash, OldRoot: &oldRoot, NewRoot: &felt.Zero, StateDiff: d,
		}
		require.NoError(t, st.Update(header, su, nil, true), "block %d", height)

		require.NoError(t, core.WriteBlockHeaderByNumber(txn, header))
		require.NoError(t, core.WriteBlockHeaderNumberByHash(txn, header.Hash, blockNum))
		require.NoError(t, core.WriteBlockCommitment(txn, blockNum, &core.BlockCommitments{}))
		require.NoError(t, core.WriteStateUpdateByBlockNum(txn, blockNum, su))
		require.NoError(t, core.WriteTransactionsAndReceipts(txn, blockNum, nil, nil))
		require.NoError(t, txn.Write())
	}
	require.NoError(t, core.WriteChainHeight(database, header.Number))
	l1Head := &core.L1Head{BlockNumber: header.Number, BlockHash: header.Hash}
	require.NoError(t, core.WriteL1Head(database, l1Head))
}

// stateAnswers reads every contract and slot at every height from the given one up to the tip.
func stateAnswers(
	t *testing.T, database db.KeyValueStore, from uint64, deprecated bool,
) map[string]string {
	t.Helper()
	tip, err := core.GetChainHeight(database)
	require.NoError(t, err)
	format := func(v felt.Felt, err error) string {
		if errors.Is(err, db.ErrKeyNotFound) {
			return "not found"
		}
		require.NoError(t, err)
		return v.String()
	}
	stateDB := state.NewStateDB(database, triedb.New(database, nil))
	answers := map[string]string{}
	for height := from; height <= tip; height++ {
		var reader core.StateReader
		if deprecated {
			txn := database.NewIndexedBatch() //nolint:staticcheck // the deprecated state needs it
			reader = deprecatedstate.NewHistory(deprecatedstate.New(txn), height)
		} else {
			history, err := state.NewStateHistory(height, &felt.Zero, stateDB)
			require.NoError(t, err)
			reader = &history
		}
		for _, addr := range chainAddrs {
			key := fmt.Sprintf("%d/%s/", height, &addr)
			answers[key+"class"] = format(reader.ContractClassHash(&addr))
			answers[key+"nonce"] = format(reader.ContractNonce(&addr))
			for _, slot := range chainSlots {
				answers[key+slot.String()] = format(reader.ContractStorage(&addr, &slot))
			}
		}
	}
	return answers
}
