package rpcv10_test

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// numBenchContracts controls how many contracts (each with a non-zero nonce) get
// written into the synthetic state before the benchmark.
const numBenchContracts = 50_000

// buildSyntheticStateUpdate returns a state update that deploys
// n contracts, each with a distinct non-zero nonce, plus the
// slice of their addresses to read back. Addresses are
// pseudorandom full-width felts (deterministic seed).
func buildSyntheticStateUpdate(t testing.TB, n int) (*core.StateUpdate, []felt.Felt) {
	t.Helper()
	classHash := felt.FromUint64[felt.Felt](0xc0ffee)
	deployed := make(map[felt.Felt]*felt.Felt, n)
	nonces := make(map[felt.Felt]*felt.Felt, n)
	addrs := make([]felt.Felt, n)

	rng := rand.NewChaCha8([32]byte{})
	var buf [32]byte
	for i := range n {
		_, _ = rng.Read(buf[:])
		addr := felt.FromBytes[felt.Felt](buf[:])
		nonce := felt.FromUint64[felt.Felt](uint64(i) + 1)

		deployed[addr] = &classHash
		nonces[addr] = &nonce
		addrs[i] = addr
	}

	su := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: &felt.Zero,
		StateDiff: &core.StateDiff{
			DeployedContracts: deployed,
			Nonces:            nonces,
		},
	}
	return su, addrs
}

// buildHandler builds a Handler backed by a real on-disk Pebble DB populated with
// the synthetic state.
func buildHandler(b *testing.B, su *core.StateUpdate) *rpc.Handler {
	b.Helper()
	testDB, err := pebble.New(b.TempDir())
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, testDB.Close()) })

	populate(b, testDB, su)

	chain := blockchain.New(testDB, &networks.Sepolia)
	return rpc.New(chain, nil, nil, log.NewNopZapLogger())
}

// populate writes the synthetic state into testDB via the deprecated state,
// which is the state currently running in production.
func populate(b *testing.B, testDB db.KeyValueStore, su *core.StateUpdate) {
	b.Helper()
	//nolint:staticcheck,nolintlint // deprecatedstate.New requires an IndexedBatch
	batch := testDB.NewIndexedBatch()
	require.NoError(b, deprecatedstate.New(batch).Update(&core.Header{Number: 0}, su, nil, true))

	// head markers: the minimum block metadata that HeadState needs
	hdr := &core.Header{
		Number:          0,
		Hash:            &felt.Zero,
		ParentHash:      &felt.Zero,
		GlobalStateRoot: &felt.Zero,
	}
	require.NoError(b, core.WriteBlockHeaderByNumber(batch, hdr))
	require.NoError(b, core.WriteChainHeight(batch, 0))

	require.NoError(b, batch.Write())
}

// BenchmarkNonce profiles Handler.Nonce against a real DB-backed state. The
// addresses are pseudorandom felts.
//
// Profile the read path only (setup is excluded via -focus):
//
//	go test ./rpc/v10/ -run='^$' -bench='BenchmarkNonce' \
//	  -benchtime=10s -cpuprofile=cpu.out
//	go tool pprof -http=:8080 -focus='Handler\)\.Nonce' cpu.out
func BenchmarkNonce(b *testing.B) {
	b.ReportAllocs()

	su, addrs := buildSyntheticStateUpdate(b, numBenchContracts)
	latest := rpc.BlockIDLatest()
	handler := buildHandler(b, su)

	i := 0
	for b.Loop() {
		addr := addrs[i%len(addrs)]
		i++

		_, rpcErr := handler.Nonce(&latest, &addr)
		if rpcErr != nil {
			b.Fatalf("unexpected rpc error: %v", rpcErr)
		}
	}
}
