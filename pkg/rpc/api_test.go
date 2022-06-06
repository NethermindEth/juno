package rpc

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/common"
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
	blockHash := testFelt1
	blockNumber := uint64(2175)
	services.BlockService.StoreBlock(blockHash.Bytes(), &block.Block{
		Hash:             blockHash.Bytes(),
		BlockNumber:      blockNumber,
		ParentBlockHash:  common.Hex2Bytes("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
		Status:           "ACCEPTED_ON_L2",
		SequencerAddress: common.Hex2Bytes("0"),
		GlobalStateRoot:  common.Hex2Bytes("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
		OldRoot:          common.Hex2Bytes("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
		AcceptedTime:     1652492749,
		TimeStamp:        1652488132,
		TxCount:          2,
		TxCommitment:     common.Hex2Bytes("0"),
		EventCount:       19,
		EventCommitment:  common.Hex2Bytes("0"),
		TxHashes: [][]byte{
			common.Hex2Bytes("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
			common.Hex2Bytes("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
		},
	})

	services.StateService.Setup(db.NewKeyValueDb(t.TempDir(), 0), db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0)))
	if err := services.StateService.Run(); err != nil {
		t.Fatalf("unexpected error starting state service: %s", err)
	}
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
			Request:  buildRequest("starknet_getStorageAt", address.Hex(), key.Hex(), blockHash.Hex()),
			Response: buildResponse(value),
		},
	})
}

// func TestStarknetGetCode(t *testing.T) {
// 	// setup

// 	services.BlockService.Setup(db.NewKeyValueDb(t.TempDir(), 0))
// 	if err := services.BlockService.Run(); err != nil {
// 		t.Fatalf("unexpected error starting block service: %s", err)
// 	}
// 	hash := common.Hex2Bytes("43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb")
// 	services.BlockService.StoreBlock(hash, &block.Block{
// 		Hash:             hash,
// 		BlockNumber:      2175,
// 		ParentBlockHash:  common.Hex2Bytes("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
// 		Status:           "ACCEPTED_ON_L2",
// 		SequencerAddress: common.Hex2Bytes("0"),
// 		GlobalStateRoot:  common.Hex2Bytes("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
// 		OldRoot:          common.Hex2Bytes("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
// 		AcceptedTime:     1652492749,
// 		TimeStamp:        1652488132,
// 		TxCount:          2,
// 		TxCommitment:     common.Hex2Bytes("0"),
// 		EventCount:       19,
// 		EventCommitment:  common.Hex2Bytes("0"),
// 		TxHashes: [][]byte{
// 			common.Hex2Bytes("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
// 			common.Hex2Bytes("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
// 		},
// 	})

// 	services.StateService.Setup(db.NewKeyValueDb(t.TempDir(), 0), db.NewBlockSpecificDatabase(db.NewKeyValueDb(t.TempDir(), 0)))
// 	if err := services.StateService.Run(); err != nil {
// 		t.Fatalf("unexpected error starting state service: %s", err)
// 	}
// 	address := common.Hex2Bytes("1bd7ca87f139693e6681be2042194cf631c4e8d77027bf0ea9e6d55fc6018ac")
// 	code := &state.Code{Code: [][]byte{
// 		common.Hex2Bytes("40780017fff7fff"),
// 		common.Hex2Bytes("1"),
// 		common.Hex2Bytes("208b7fff7fff7ffe"),
// 		common.Hex2Bytes("400380007ffb7ffc"),
// 		common.Hex2Bytes("400380017ffb7ffd"),
// 		common.Hex2Bytes("800000000000010fffffffffffffffffffffffffffffffffffffffffffffffb"),
// 		common.Hex2Bytes("107a2e2e5a8b6552e977246c45bfac446305174e86be2e5c74e8c0a20fd1de7"),
// 	}}
// 	services.StateService.StoreCode(address, code)

// 	// create tests
// 	tests := []rpcTest{
// 		{
// 			Request:  buildRequest("starknet_getStorageAt", "0x0000000000000000000000000000000000000000", "0x0"),
// 			Response: buildResponse("0x0000000000000000000000000000000000000000000000000000000000000000"),
// 		},
// 	}

// 	// test
// 	testServer(t, tests)
// }
