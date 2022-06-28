package services

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
)

func TestAbiService_StoreGet(t *testing.T) {
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	database, err := db.NewMDBXDatabase(env, "ABI")
	if err != nil {
		t.Error(err)
	}
	AbiService.Setup(database)
	if err := AbiService.Run(); err != nil {
		t.Errorf("unexpeted error in Run: %s", err)
	}
	defer AbiService.Close(context.Background())

	for address, a := range abis {
		AbiService.StoreAbi(address, a)
	}
	for address, a := range abis {
		result := AbiService.GetAbi(address)
		if result == nil {
			t.Errorf("abi not foud for key: %s", address)
		}
		if !a.Equal(result) {
			t.Errorf("ABI are not equal after Store-Get operations, address: %s", address)
		}
	}
}

var abis = map[string]*abi.Abi{
	"1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac": {
		Functions: []*abi.Function{
			{
				Name: "initialize",
				Inputs: []*abi.Function_Input{
					{
						Name: "signer",
						Type: "felt",
					},
					{
						Name: "guardian",
						Type: "felt",
					},
				},
				Outputs: nil,
			},
			{
				Name: "__execute__",
				Inputs: []*abi.Function_Input{
					{
						Name: "call_array_len",
						Type: "felt",
					},
					{
						Name: "call_array",
						Type: "felt",
					},
					{
						Name: "calldata_len",
						Type: "felt",
					},
					{
						Name: "calldata",
						Type: "felt*",
					},
					{
						Name: "nonce",
						Type: "felt",
					},
				},
				Outputs: []*abi.Function_Output{
					{
						Name: "retdata_size",
						Type: "felt",
					},
					{
						Name: "retdata",
						Type: "felt*",
					},
				},
			},
			{
				Name: "upgrade",
				Inputs: []*abi.Function_Input{
					{
						Name: "implementation",
						Type: "felt",
					},
				},
				Outputs: nil,
			},
		},
		Events: []*abi.AbiEvent{
			{
				Data: []*abi.AbiEvent_Data{
					{
						Name: "new_signer",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "signer_changed",
			},
			{
				Data: []*abi.AbiEvent_Data{
					{
						Name: "new_guardian",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "guardian_changed",
			},
			{
				Data: []*abi.AbiEvent_Data{
					{
						Name: "new_guardian",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "guardian_backup_changed",
			},
			{
				Data: []*abi.AbiEvent_Data{
					{
						Name: "active_at",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "escape_guardian_triggered",
			},
			{
				Data: []*abi.AbiEvent_Data{
					{
						Name: "active_at",
						Type: "felt",
					},
				},
				Keys: nil,
				Name: "escape_signer_triggered",
			},
			{
				Data: nil,
				Keys: nil,
				Name: "escape_canceled",
			},
		},
		Structs: []*abi.Struct{
			{
				Fields: []*abi.Struct_Field{
					{
						Name:   "to",
						Type:   "felt",
						Offset: 0,
					},
					{
						Name:   "selector",
						Type:   "felt",
						Offset: 1,
					},
					{
						Name:   "data_offset",
						Type:   "felt",
						Offset: 2,
					},
					{
						Name:   "data_len",
						Type:   "felt",
						Offset: 3,
					},
				},
				Name: "CallArray",
				Size: 4,
			},
		},
		L1Handlers: []*abi.Function{
			{
				Name: "__l1_default__",
				Inputs: []*abi.Function_Input{
					{
						Name: "selector",
						Type: "felt",
					},
					{
						Name: "calldata_size",
						Type: "felt",
					},
					{
						Name: "calldata",
						Type: "felt*",
					},
				},
				Outputs: nil,
			},
		},
		Constructor: &abi.Function{
			Name: "constructor",
			Inputs: []*abi.Function_Input{
				{
					Name: "signer",
					Type: "felt",
				},
				{
					Name: "guardian",
					Type: "felt",
				},
			},
			Outputs: nil,
		},
	},
}
