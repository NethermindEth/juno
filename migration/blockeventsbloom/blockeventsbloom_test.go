package blockeventsbloom

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

// deprecatedBlockHeader is the pre-migration core.Header layout, which embedded
// the event bloom filter. The tests use it to write and read legacy
// (bloom-in-header) records; field names and cbor tags must match the old
// core.Header exactly.
type deprecatedBlockHeader struct {
	Hash             *felt.Felt
	ParentHash       *felt.Felt
	Number           uint64
	GlobalStateRoot  *felt.Felt
	SequencerAddress *felt.Felt
	TransactionCount uint64
	EventCount       uint64
	Timestamp        uint64
	ProtocolVersion  string
	EventsBloom      *bloom.BloomFilter
	L1GasPriceETH    *felt.Felt `cbor:"gasprice"`
	Signatures       [][]*felt.Felt
	L1GasPriceSTRK   *felt.Felt `cbor:"gaspricestrk"`
	L1DAMode         core.L1DAMode
	L1DataGasPrice   *core.GasPrice
	L2GasPrice       *core.GasPrice
}

// writeLegacyHeader stores a header in the pre-migration (bloom-in-header)
// layout, plus a commitment so the block counts as retained (the migration
// keys its start block off the retention floor).
func writeLegacyHeader(t *testing.T, database db.KeyValueStore, n uint64) {
	t.Helper()
	bloomFilter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	bloomFilter.Add([]byte{byte(n), 0xAB})
	legacy := &deprecatedBlockHeader{
		Number:      n,
		Hash:        new(felt.Felt).SetUint64(n),
		EventsBloom: bloomFilter,
	}
	data, err := encoder.Marshal(legacy)
	require.NoError(t, err)
	require.NoError(t, database.Put(db.BlockHeaderByNumberKey(n), data))
	require.NoError(t, core.WriteBlockCommitment(database, n, &core.BlockCommitments{}))
}

func decodeLegacyHeader(t *testing.T, database db.KeyValueReader, n uint64) *deprecatedBlockHeader {
	t.Helper()
	var h deprecatedBlockHeader
	require.NoError(t, database.Get(db.BlockHeaderByNumberKey(n), func(data []byte) error {
		return encoder.Unmarshal(data, &h)
	}))
	return &h
}

func TestBlockEventsBloomMigration(t *testing.T) {
	t.Run("empty database is a no-op", func(t *testing.T) {
		database := memory.New()
		state, err := (&Migrator{}).Migrate(
			t.Context(), database, &networks.Sepolia, log.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, state)
	})

	t.Run("strips bloom out of the header", func(t *testing.T) {
		database := memory.New()
		const height = uint64(20)
		for n := uint64(0); n <= height; n++ {
			writeLegacyHeader(t, database, n)
		}
		require.NoError(t, core.WriteChainHeight(database, height))

		state, err := (&Migrator{}).Migrate(
			t.Context(), database, &networks.Sepolia, log.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, state)

		for n := range height + 1 {
			// Header must no longer carry the bloom; the bloom is discarded.
			require.Nil(t, decodeLegacyHeader(t, database, n).EventsBloom,
				"header %d still carries a bloom", n)
		}
	})

	t.Run("re-running is idempotent", func(t *testing.T) {
		database := memory.New()
		writeLegacyHeader(t, database, 0)
		require.NoError(t, core.WriteChainHeight(database, 0))

		for range 2 {
			_, err := (&Migrator{}).Migrate(t.Context(), database, &networks.Sepolia, log.NewNopZapLogger())
			require.NoError(t, err)
		}

		require.Nil(t, decodeLegacyHeader(t, database, 0).EventsBloom)
	})

	t.Run("resumes from the saved intermediate state", func(t *testing.T) {
		database := memory.New()
		const (
			height   = uint64(20)
			resumeAt = uint64(10)
		)
		for n := uint64(0); n <= height; n++ {
			writeLegacyHeader(t, database, n)
		}
		require.NoError(t, core.WriteChainHeight(database, height))

		migrator := &Migrator{}
		require.NoError(t, migrator.Before(encodeIntermediateState(resumeAt)))
		state, err := migrator.Migrate(t.Context(), database, &networks.Sepolia, log.NewNopZapLogger())
		require.NoError(t, err)
		require.Nil(t, state)

		// Blocks below the resume point are left untouched: bloom still in the header.
		for n := range resumeAt {
			require.NotNil(t, decodeLegacyHeader(t, database, n).EventsBloom,
				"header %d should still carry its bloom", n)
		}
		// Blocks from the resume point onward are stripped.
		for n := resumeAt; n <= height; n++ {
			require.Nil(t, decodeLegacyHeader(t, database, n).EventsBloom,
				"header %d should have been stripped", n)
		}
	})

	t.Run("bounds the scan to the retained window", func(t *testing.T) {
		database := memory.New()
		const height = uint64(10)
		// Only blocks [5, height] have headers/commitments; [0,5) are "pruned".
		// The scan starts at the retention floor, so the pruned prefix is never
		// visited — visiting it would error, since a missing header in the
		// scanned range is treated as corruption, not skipped.
		for n := uint64(5); n <= height; n++ {
			writeLegacyHeader(t, database, n)
		}
		require.NoError(t, core.WriteChainHeight(database, height))

		state, err := (&Migrator{}).Migrate(
			t.Context(), database, &networks.Sepolia, log.NewNopZapLogger(),
		)
		require.NoError(t, err)
		require.Nil(t, state)

		// Retained blocks were scanned and stripped.
		for n := uint64(5); n <= height; n++ {
			require.Nil(t, decodeLegacyHeader(t, database, n).EventsBloom,
				"header %d should have been stripped", n)
		}
	})
}
