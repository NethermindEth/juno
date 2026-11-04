package sync_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/adaptfeeder"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mainnetGenesisClassHash = "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"

func TestMissingClassHashes(t *testing.T) {
	knownClass := felt.NewUnsafeFromString[felt.ClassHash](mainnetGenesisClassHash)
	classA := felt.NewUnsafeFromString[felt.ClassHash]("0xa")
	classB := felt.NewUnsafeFromString[felt.ClassHash]("0xb")
	classC := felt.NewUnsafeFromString[felt.ClassHash]("0xc")
	sierraClassA := felt.NewUnsafeFromString[felt.SierraClassHash]("0xa")
	sierraClassC := felt.NewUnsafeFromString[felt.SierraClassHash]("0xc")
	compiledClass := felt.NewUnsafeFromString[felt.CasmClassHash]("0xd")

	tests := []struct {
		name         string
		storedBlocks uint64
		stateDiff    *core.StateDiff
		want         []*felt.ClassHash
	}{
		{
			name:      "nothing introduced",
			stateDiff: &core.StateDiff{},
			want:      []*felt.ClassHash{},
		},
		{
			name: "empty database lacks every introduced class",
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Address]*felt.ClassHash{
					*felt.NewUnsafeFromString[felt.Address]("0x1"): classA,
				},
				DeclaredV0Classes: []*felt.ClassHash{classB},
				DeclaredV1Classes: map[felt.SierraClassHash]*felt.CasmClassHash{*sierraClassC: compiledClass},
			},
			want: []*felt.ClassHash{classA, classB, classC},
		},
		{
			name: "class introduced several ways is reported once",
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Address]*felt.ClassHash{
					*felt.NewUnsafeFromString[felt.Address]("0x1"): classA,
					*felt.NewUnsafeFromString[felt.Address]("0x2"): classA,
				},
				DeclaredV0Classes: []*felt.ClassHash{classA},
				DeclaredV1Classes: map[felt.SierraClassHash]*felt.CasmClassHash{*sierraClassA: compiledClass},
			},
			want: []*felt.ClassHash{classA},
		},
		{
			name:         "classes the database holds are skipped",
			storedBlocks: 1,
			stateDiff: &core.StateDiff{
				DeployedContracts: map[felt.Address]*felt.ClassHash{
					*felt.NewUnsafeFromString[felt.Address]("0x1"): knownClass,
					*felt.NewUnsafeFromString[felt.Address]("0x2"): classA,
				},
			},
			want: []*felt.ClassHash{classA},
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
					committed.Block,
					&core.BlockCommitments{},
					committed.StateUpdate,
					committed.NewClasses,
					core.EventsBloom(committed.Block.Receipts),
				))
			}

			got, err := sync.MissingClassHashes(chain, tt.stateDiff)

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
