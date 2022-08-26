package sync

import (
	"testing"

	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

func TestStateUpdateResponseToStateDiff(t *testing.T) {
	resp := feeder.StateUpdateResponse{
		BlockHash: "0",
		NewRoot:   "0",
		OldRoot:   "0",
		StateDiff: feeder.StateDiff{
			DeployedContracts: []feeder.DeployedContract{
				{
					Address:      "0",
					ContractHash: "0",
				},
			},
			StorageDiffs: map[string][]feeder.KV{
				"key_address": {
					{
						Key:   "0",
						Value: "0",
					},
				},
			},
		},
	}

	want := &types.StateDiff{
		BlockHash: new(felt.Felt).SetHex("0"),
		NewRoot:   new(felt.Felt).SetHex("0"),
		OldRoot:   new(felt.Felt).SetHex("0"),
		StorageDiff: types.StorageDiff{
			new(felt.Felt).SetHex("0").Value(): []types.MemoryCell{
				{
					Address: new(felt.Felt).SetHex("0"),
					Value:   new(felt.Felt).SetHex("0"),
				},
			},
		},
		DeployedContracts: []types.DeployedContract{
			{
				Address: new(felt.Felt).SetHex("0"),
				Hash:    new(felt.Felt).SetHex("0"),
			},
		},
	}

	got := stateUpdateResponseToStateDiff(resp, 0)

	assert.DeepEqual(t, got, want, gocmp.Comparer(func(x *felt.Felt, y *felt.Felt) bool { return x.CmpCompat(y) == 0 }))
}
