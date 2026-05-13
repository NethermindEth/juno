package node

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestFetchL1HeadIfMissing_SkipsL1FetchWhenHeadPresent(t *testing.T) {
	database := memory.New()
	want := &core.L1Head{
		BlockNumber: 1,
		BlockHash:   felt.NewRandom[felt.Felt](),
		StateRoot:   felt.NewRandom[felt.Felt](),
	}
	require.NoError(t, core.WriteL1Head(database, want))

	cfg := &Config{EthNode: ""}
	require.NoError(t, fetchL1HeadIfMissing(t.Context(), database, cfg, nil, log.NewNopZapLogger()))

	got, err := core.GetL1Head(database)
	require.NoError(t, err)
	require.Equal(t, *want, got)
}

func TestFetchL1HeadIfMissing_WrapsL1ClientError(t *testing.T) {
	database := memory.New()
	cfg := &Config{EthNode: ""}
	err := fetchL1HeadIfMissing(t.Context(), database, cfg, nil, log.NewNopZapLogger())
	require.ErrorContains(t, err, "creating a new L1 client")
}

func TestMigrateIfNeeded_WrapsPruneFetchError(t *testing.T) {
	cfg := &Config{
		Prune:   true,
		EthNode: "",
		Network: networks.Sepolia,
	}
	err := migrateIfNeeded(t.Context(), memory.New(), cfg, nil, log.NewNopZapLogger())
	require.ErrorContains(t, err, "fetch L1 head for pruning")
}
