package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	testFelt1 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000001")
	testFelt2 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000002")
	testFelt3 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000003")
	testFelt4 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000004")
)

func buildRequest(method string, params ...interface{}) string {
	request := struct {
		Jsonrpc string        `json:"jsonrpc"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
		ID      int           `json:"id"`
	}{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(request)
	return buf.String()
}

func buildResponse(result interface{}) string {
	response := struct {
		Jsonrpc string      `json:"jsonrpc"`
		Result  interface{} `json:"result"`
		ID      int         `json:"id"`
	}{
		Jsonrpc: "2.0",
		Result:  result,
		ID:      1,
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(response)
	return buf.String()
}

func TestStarknetGetStorageAt(t *testing.T) {
	// setup
	services.BlockService.Setup(db.NewKeyValueDb(t.TempDir(), 0))
	if err := services.BlockService.Run(); err != nil {
		t.Fatalf("unexpected error starting block service: %s", err)
	}
	defer services.BlockService.Close(context.Background())

	blockHash := types.BlockHash(testFelt1)
	blockNumber := uint64(2175)
	services.BlockService.StoreBlock(blockHash, &types.Block{
		BlockHash:       blockHash,
		BlockNumber:     blockNumber,
		ParentHash:      types.HexToBlockHash("0xf8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
		Status:          types.BlockStatusAcceptedOnL2,
		Sequencer:       types.HexToAddress("0x0"),
		NewRoot:         types.HexToFelt("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
		OldRoot:         types.HexToFelt("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
		AcceptedTime:    1652492749,
		TimeStamp:       1652488132,
		TxCount:         2,
		TxCommitment:    types.HexToFelt("0x0"),
		EventCount:      19,
		EventCommitment: types.HexToFelt("0x0"),
		TxHashes: []types.TransactionHash{
			types.HexToTransactionHash("0x5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
			types.HexToTransactionHash("0x4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
		},
	})

	services.StateService.Setup(
		db.NewKeyValueDb(t.TempDir(), 0),
		db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0)),
	)
	if err := services.StateService.Run(); err != nil {
		t.Fatalf("unexpected error starting state service: %s", err)
	}
	defer services.StateService.Close(context.Background())

	address := testFelt2
	key := testFelt3
	value := testFelt4
	storage := &state.Storage{
		Storage: map[string]string{
			key.Hex(): value.Hex(),
		},
	}
	services.StateService.StoreStorage(address.Hex(), blockNumber, storage)

	// test
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_getStorageAt", address.Hex(), key.Hex(), blockHash.Felt().String()),
			Response: buildResponse(Felt(value.Hex())),
		},
	})
}

func TestStarknetGetCode(t *testing.T) {
	// setup
	services.AbiService.Setup(db.NewKeyValueDb(t.TempDir(), 0))
	if err := services.AbiService.Run(); err != nil {
		t.Fatalf("unexpected error starting abi service: %s", err)
	}
	defer services.AbiService.Close(context.Background())

	address := types.Address(testFelt2)
	abi := &abi.Abi{
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
			},
			Outputs: nil,
		},
	}

	services.StateService.Setup(
		db.NewKeyValueDb(t.TempDir(), 0),
		db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0)),
	)
	if err := services.StateService.Run(); err != nil {
		t.Fatalf("unexpected error starting state service: %s", err)
	}

	services.AbiService.StoreAbi(address.Hex(), abi)

	defer services.StateService.Close(context.Background())

	code := &state.Code{
		Code: [][]byte{
			types.HexToFelt("0x1111").Bytes(),
			types.HexToFelt("0x1112").Bytes(),
			types.HexToFelt("0x1113").Bytes(),
			types.HexToFelt("0x1114").Bytes(),
		},
	}
	services.StateService.StoreCode(address.Bytes(), code)

	abiResponse, _ := json.Marshal(abi)
	codeResponse := make([]Felt, len(code.Code))
	for i, bcode := range code.Code {
		codeResponse[i] = Felt(types.BytesToFelt(bcode).Hex())
	}

	// test
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_getCode", address.Hex()),
			Response: buildResponse(CodeResult{Bytecode: codeResponse, Abi: string(abiResponse)}),
		},
	})
}
