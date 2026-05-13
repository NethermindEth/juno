package node

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

func TestFetchL1HeadIfMissing_SkipsL1FetchWhenHeadPresent(t *testing.T) {
	database := memory.New()
	require.NoError(t, core.WriteL1Head(database, &core.L1Head{
		BlockNumber: 1,
		BlockHash:   felt.NewRandom[felt.Felt](),
		StateRoot:   felt.NewRandom[felt.Felt](),
	}))

	cfg := &Config{EthNode: ""}
	require.NoError(t, fetchL1HeadIfMissing(t.Context(), database, cfg, nil, log.NewNopZapLogger()))
}
