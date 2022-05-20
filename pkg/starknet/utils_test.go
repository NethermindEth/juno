package starknet

import (
	"github.com/NethermindEth/juno/pkg/feeder"
	"testing"
)

//func ethereumFaker() *ethclient.Client {
//	return geth.NewEthereumClient()
//}
func TestRemove0x(t *testing.T) {
	tests := [...]struct {
		entry    string
		expected string
	}{
		{
			"0x001111",
			"1111",
		},
		{
			"04a1111",
			"4a1111",
		},
		{
			"000000001",
			"1",
		},
		{
			"000ssldkfmsd1111",
			"ssldkfmsd1111",
		},
		{
			"000111sp",
			"111sp",
		},
		{
			"0",
			"0",
		},
	}

	for _, test := range tests {
		answer := remove0x(test.entry)
		if answer != test.expected {
			t.Fail()
		}
	}
}

func TestStateUpdateResponseToStateDiff(t *testing.T) {
	kvs := []feeder.KV{
		{
			Key:   "Key1",
			Value: "Value1",
		},
		{
			Key:   "Key2",
			Value: "Value2",
		},
	}
	diff := feeder.StateDiff{
		DeployedContracts: []struct {
			Address      string `json:"address"`
			ContractHash string `json:"contract_hash"`
		}{
			{
				"address1",
				"contract_hash1",
			},
			{
				"address2",
				"contract_hash2",
			},
		},
		StorageDiffs: map[string][]feeder.KV{
			"key_address": kvs,
		},
	}
	feederVal := feeder.StateUpdateResponse{
		BlockHash: "BlockHash",
		NewRoot:   "NewRoot",
		OldRoot:   "OldRoot",
		StateDiff: diff,
	}

	value := stateUpdateResponseToStateDiff(feederVal)

	if len(value.DeployedContracts) != len(feederVal.StateDiff.DeployedContracts) {
		t.Fail()
	}
	if value.DeployedContracts[0].ContractHash != feederVal.StateDiff.DeployedContracts[0].ContractHash ||
		value.DeployedContracts[1].ContractHash != feederVal.StateDiff.DeployedContracts[1].ContractHash ||
		value.DeployedContracts[0].Address != feederVal.StateDiff.DeployedContracts[0].Address ||
		value.DeployedContracts[1].Address != feederVal.StateDiff.DeployedContracts[1].Address {
		t.Fail()
	}

	val, ok := diff.StorageDiffs["key_address"]
	if !ok {
		t.Fail()
	}
	val2, ok := feederVal.StateDiff.StorageDiffs["key_address"]
	if !ok {
		t.Fail()
	}

	if len(val) != len(val2) {
		for k, v := range val {
			if v.Key != val2[k].Key || v.Value != val2[k].Value {
				t.Fail()
			}
		}
	}
}
