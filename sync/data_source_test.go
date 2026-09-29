package sync_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mainnetGenesisClassHash = "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"

func TestMissingClassHashes(t *testing.T) {
	knownClass := felt.NewUnsafeFromString[felt.Felt](mainnetGenesisClassHash)
	classA := felt.NewUnsafeFromString[felt.Felt]("0xa")
	classB := felt.NewUnsafeFromString[felt.Felt]("0xb")
	classC := felt.NewUnsafeFromString[felt.Felt]("0xc")

	tests := []struct {
		name         string
		storedBlocks uint64
		stateDiff    *core.StateDiff
		want         []*felt.Felt
	}{
		{
			name:      "nothing introduced",
			stateDiff: &core.StateDiff{},
			want:      []*felt.Felt{},
		},
		{
			name: "empty database lacks every introduced class",
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{*classA: classA},
				DeclaredV0Classes: []*felt.Felt{classB},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{*classC: classC},
			},
			want: []*felt.Felt{classA, classB, classC},
		},
		{
			name: "class introduced several ways is reported once",
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x1"): classA,
					*felt.NewUnsafeFromString[felt.Felt]("0x2"): classA,
				},
				DeclaredV0Classes: []*felt.Felt{classA},
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{*classA: classA},
			},
			want: []*felt.Felt{classA},
		},
		{
			name:         "classes the database holds are skipped",
			storedBlocks: 1,
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*felt.NewUnsafeFromString[felt.Felt]("0x1"): knownClass,
					*felt.NewUnsafeFromString[felt.Felt]("0x2"): classA,
				},
			},
			want: []*felt.Felt{classA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := blockchain.New(
				memory.New(),
				&networks.Mainnet,
				blockchain.WithNewState(statetestutils.UseNewState()),
			)
			gateway := adaptfeeder.New(feeder.NewTestClient(t, &networks.Mainnet))
			feederSource := sync.NewFeederGatewayDataSource(chain, gateway)
			for number := range tt.storedBlocks {
				committed, err := feederSource.BlockByNumber(t.Context(), number)
				require.NoError(t, err)
				require.NoError(t, chain.Store(
					committed.Block, &core.BlockCommitments{}, committed.StateUpdate, committed.NewClasses,
				))
			}

			got, err := sync.MissingClassHashes(chain, tt.stateDiff)

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
