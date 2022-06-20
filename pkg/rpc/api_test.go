package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/abi"
	"github.com/NethermindEth/juno/internal/db/state"
	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/feeder/feederfakes"
	"github.com/NethermindEth/juno/pkg/types"

	"gotest.tools/assert"
)

var (
	testFelt1 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000001")
	testFelt2 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000002")
	testFelt3 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000003")
	testFelt4 types.Felt = types.HexToFelt("0x0000000000000000000000000000000000000000000000000000000000000004")

	fakeClient feederfakes.FakeHttpClient = feederfakes.FakeHttpClient{}
	client     feeder.HttpClient          = &fakeClient
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
	err := db.InitializeDatabaseEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	blockDb, err := db.GetDatabase("BLOCK")
	if err != nil {
		t.Error(err)
	}
	// setup
	services.BlockService.Setup(blockDb)
	if err := services.BlockService.Run(); err != nil {
		t.Fatalf("unexpected error starting block service: %s", err)
	}
	defer services.BlockService.Close(context.Background())

	blockHash := types.BlockHash(testFelt1)
	blockNumber := uint64(2175)
	block := &types.Block{
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
	}
	services.BlockService.StoreBlock(blockHash, block)

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

	// Set up feeder client for PENDING block
	body, err := json.Marshal(block)
	if err != nil {
		t.Fatal("unexpected marshal error", err)
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error when calling `feeder.DoReturns`", err)
	}
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)

	// test
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_getStorageAt", address.Hex(), key.Hex(), blockHash.Felt().String()),
			Response: buildResponse(Felt(value.Hex())),
		},
		{
			Request:  buildRequest("starknet_getStorageAt", address.Hex(), key.Hex(), BlocktagPending),
			Response: buildResponse(Felt(value.Hex())),
		},
	})
}

func TestStarknetGetCode(t *testing.T) {
	err := db.InitializeDatabaseEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	abiDb, err := db.GetDatabase("ABI")
	if err != nil {
		t.Error(err)
	}
	// setup
	services.AbiService.Setup(abiDb)
	if err := services.AbiService.Run(); err != nil {
		t.Fatalf("unexpected error starting abi service: %s", err)
	}
	defer services.AbiService.Close(context.Background())

	address := types.Address(testFelt2)
	wantAbi := &abi.Abi{
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

	services.AbiService.StoreAbi(address.Hex(), wantAbi)

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

	abiResponse, _ := json.Marshal(wantAbi)
	codeResponse := make([]types.Felt, len(code.Code))
	for i, bcode := range code.Code {
		codeResponse[i] = types.BytesToFelt(bcode)
	}

	// Set up feeder client for pending block
	input := `{"abi": [{"inputs": [{"name": "a", "type": "a"}], "name": "a", "outputs": [{"name": "a", "type": "a"}], "type": "function"}, {"inputs": [{"name": "a", "type": "a"}], "name": "a", "outputs": [{"name": "a", "type": "a"}], "type": "l1_handler"}, {"members": [{"offset": 1, "name": "a", "type": "a"}], "name": "a", "size": 1, "type": "struct"}, {"inputs": [{"name": "a", "type": "a"}], "name": "a", "outputs": [{"name": "a", "type": "a"}], "type": "constructor"}, {"data": [{"name": "a", "type": "a"}], "keys": ["a"], "name": "a", "type": "event"}], "bytecode": ["0xa"]}`
	fakeClient.DoReturns(generateResponse(input), nil)
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)

	want := &abi.Abi{
		Functions: []*abi.Function{
			{
				Name: "a",
				Inputs: []*abi.Function_Input{
					{
						Name: "a",
						Type: "a",
					},
				},
				Outputs: []*abi.Function_Output{
					{
						Name: "a",
						Type: "a",
					},
				},
			},
		},
		Events: []*abi.AbiEvent{
			{
				Name: "a",
				Data: []*abi.AbiEvent_Data{
					{
						Name: "a",
						Type: "a",
					},
				},
				Keys: []string{"a"},
			},
		},
		Structs: []*abi.Struct{
			{
				Fields: []*abi.Struct_Field{
					{
						Name:   "a",
						Type:   "a",
						Offset: 1,
					},
				},
				Name: "a",
				Size: 1,
			},
		},
		L1Handlers: []*abi.Function{
			{
				Name: "a",
				Inputs: []*abi.Function_Input{
					{
						Name: "a",
						Type: "a",
					},
				},
				Outputs: []*abi.Function_Output{
					{
						Name: "a",
						Type: "a",
					},
				},
			},
		},
		Constructor: &abi.Function{
			Name: "a",
			Inputs: []*abi.Function_Input{
				{
					Name: "a",
					Type: "a",
				},
			},
			Outputs: []*abi.Function_Output{
				{
					Name: "a",
					Type: "a",
				},
			},
		},
	}
	abiResponse2, err := json.Marshal(want)
	if err != nil {
		t.Fatal("unexpected marshal error", err)
	}
	// test
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_getCode", address.Hex()),
			Response: buildResponse(CodeResult{Bytecode: codeResponse, Abi: string(abiResponse)}),
		},
		{
			Request:  buildRequest("starknet_getCode", "0xa"), // address not held locally --> must query feeder gateway
			Response: buildResponse(CodeResult{Bytecode: []types.Felt{types.HexToFelt("a")}, Abi: string(abiResponse2)}),
		},
	})
}

var (
	txns = []types.IsTransaction{
		&types.TransactionInvoke{
			Hash:               types.HexToTransactionHash("0x4e0e3a35c2d99fce89e0583d042f98cf71d1384554b5771694956a62b9f05fd"),
			ContractAddress:    types.HexToAddress("0x3585c0f52144b1db7bfb8f644182324704c2ac1cc3470957bd097ead7ef2aa4"),
			EntryPointSelector: types.HexToFelt("0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"),
			CallData: []types.Felt{
				types.HexToFelt("0x1"),
				types.HexToFelt("0x6a09ccb1caaecf3d9683efe335a667b2169a409d19c589ba1eb771cd210af75"),
				types.HexToFelt("0x2f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354"),
				types.HexToFelt("0x0"),
				types.HexToFelt("0x3"),
				types.HexToFelt("0x3"),
				types.HexToFelt("0x3585c0f52144b1db7bfb8f644182324704c2ac1cc3470957bd097ead7ef2aa4"),
				types.HexToFelt("0x2c72fe176204c4f35400000"),
				types.HexToFelt("0x0"),
				types.HexToFelt("0x0"),
			},
			Signature: []types.Felt{
				types.HexToFelt("0x6cc4394a1db343b0f1a391e697469a96e9da126d41c8d360b9356d4396b0734"),
				types.HexToFelt("0x5cd4c66b0e8a60d28a2b0af658432555df36fd7a7eeb6b146bd3b603e088945"),
			},
			MaxFee: types.HexToFelt("0x0"),
		},
		&types.TransactionInvoke{
			Hash:               types.HexToTransactionHash("0x6687ec12afde961e106b3b8f060f268cb29e6d70e2dbec38d7a6ef7b6c1f846"),
			ContractAddress:    types.HexToAddress("0x433d2313d0995a41089ecbc91f6b3fd262119dbb6d10194f995612e2da20993"),
			EntryPointSelector: types.HexToFelt("0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"),
			CallData: []types.Felt{
				types.HexToFelt("0x1"),
				types.HexToFelt("0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82"),
				types.HexToFelt("0xe48e45e0642d5f170bb832c637926f4c85b77d555848b693304600c4275f26"),
				types.HexToFelt("0x0"),
				types.HexToFelt("0x3"),
				types.HexToFelt("0x3"),
				types.HexToFelt("0x49d2db5f6c17a5a2894f52125048aaa988850009"),
				types.HexToFelt("0x2c72fe176204c4f35400000"),
				types.HexToFelt("0x0"),
				types.HexToFelt("0x0"),
			},
			Signature: []types.Felt{
				types.HexToFelt("0x26807fb7cf476856c5c0592c66481e10ab8bb256b1eb7553c18d2734629feff"),
				types.HexToFelt("0x6b8b8d99a8c988fa58582c76b4867c856aa0dd2c6f8ae8b5b76965f12305f98"),
			},
			MaxFee: types.HexToFelt("0x0"),
		},
		&types.TransactionDeploy{
			Hash:            types.HexToTransactionHash("0x12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1"),
			ContractAddress: types.HexToAddress("0x31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280"),
			ConstructorCallData: []types.Felt{
				types.HexToFelt("0xcfc2e2866fd08bfb4ac73b70e0c136e326ae18fc797a2c090c8811c695577e"),
				types.HexToFelt("0x5f1dd5a5aef88e0498eeca4e7b2ea0fa7110608c11531278742f0b5499af4b3"),
			},
		},
	}
	receipts = []*types.TransactionReceipt{
		{
			TxHash:     types.HexToTransactionHash("0x4e0e3a35c2d99fce89e0583d042f98cf71d1384554b5771694956a62b9f05fd"),
			ActualFee:  types.HexToFelt("0x0"),
			Status:     types.TxStatusAcceptedOnL1,
			StatusData: "",
			Events: []types.Event{
				{
					FromAddress: types.HexToAddress("0x03585c0f52144b1db7bfb8f644182324704c2ac1cc3470957bd097ead7ef2aa4"),
					Keys: []types.Felt{
						types.HexToFelt("0x5ad857f66a5b55f1301ff1ed7e098ac6d4433148f0b72ebc4a2945ab85ad53"),
					},
					Data: []types.Felt{
						types.HexToFelt("0x4e0e3a35c2d99fce89e0583d042f98cf71d1384554b5771694956a62b9f05fd"),
						types.HexToFelt("0x0"),
					},
				},
			},
		},
		{
			TxHash:     types.HexToTransactionHash("0x6687ec12afde961e106b3b8f060f268cb29e6d70e2dbec38d7a6ef7b6c1f846"),
			ActualFee:  types.HexToFelt("0x0"),
			Status:     types.TxStatusAcceptedOnL2,
			StatusData: "",
			MessagesSent: []types.MessageL2ToL1{
				{
					ToAddress: types.HexToEthAddress("0xae0Ee0A63A2cE6BaeEFFE56e7714FB4EFE48D419"),
					Payload: []types.Felt{
						types.HexToFelt("0x0"),
						types.HexToFelt("0x49d2db5f6c17a5a2894f52125048aaa988850009"),
						types.HexToFelt("0x2386f26fc10000"),
						types.HexToFelt("0x0"),
					},
				},
			},
			L1OriginMessage: nil,
			Events: []types.Event{
				{
					FromAddress: types.HexToAddress("0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82"),
					Keys: []types.Felt{
						types.HexToFelt("0x194fc63c49b0f07c8e7a78476844837255213824bd6cb81e0ccfb949921aad1"),
					},
					Data: []types.Felt{
						types.HexToFelt("0x49d2db5f6c17a5a2894f52125048aaa988850009"),
						types.HexToFelt("0x2386f26fc10000"),
						types.HexToFelt("0x0"),
					},
				},
				{
					FromAddress: types.HexToAddress("0x433d2313d0995a41089ecbc91f6b3fd262119dbb6d10194f995612e2da20993"),
					Keys: []types.Felt{
						types.HexToFelt("0x433d2313d0995a41089ecbc91f6b3fd262119dbb6d10194f995612e2da20993"),
					},
					Data: []types.Felt{
						types.HexToFelt("0x433d2313d0995a41089ecbc91f6b3fd262119dbb6d10194f995612e2da20993"),
						types.HexToFelt("0x0"),
					},
				},
			},
		},
		{
			TxHash:     types.HexToTransactionHash("0x12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1"),
			ActualFee:  types.HexToFelt("0x0"),
			Status:     types.TxStatusAcceptedOnL1,
			StatusData: "",
			L1OriginMessage: &types.MessageL1ToL2{
				FromAddress: types.HexToEthAddress("0xae0ee0a63a2ce6baeeffe56e7714fb4efe48d419"),
				Payload: []types.Felt{
					types.HexToFelt("0xc"),
					types.HexToFelt("0x22"),
				},
			},
		},
	}
	blocks = []types.Block{
		{
			BlockHash:    types.HexToBlockHash("0x1cf10396b725510b794f03d90bd670d463747ece48c94e36ac9e04b9ec122b6"),
			ParentHash:   types.HexToBlockHash("0x2439288f35c3da4a8aa3f689ddcf6f83fd9bdc9357c04d12265501e68e14d64"),
			BlockNumber:  2591,
			Status:       types.BlockStatusAcceptedOnL1,
			Sequencer:    types.HexToAddress("0x21f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5"),
			NewRoot:      types.HexToFelt("0166c5ae3f0f631a4d5543d447aa4cee6a975b2e83070d8986c94952aea10aae"),
			OldRoot:      types.HexToFelt("06d8106c901208726feea70d1155d085aee27e63ae0368b9ecc1c3bf1b5d4fe8"),
			AcceptedTime: 1654588282,
			TimeStamp:    1654587282,
			TxCount:      3,
			TxHashes: []types.TransactionHash{
				types.HexToTransactionHash("0x4e0e3a35c2d99fce89e0583d042f98cf71d1384554b5771694956a62b9f05fd"),
				types.HexToTransactionHash("0x6687ec12afde961e106b3b8f060f268cb29e6d70e2dbec38d7a6ef7b6c1f846"),
				types.HexToTransactionHash("0x12c96ae3c050771689eb261c9bf78fac2580708c7f1f3d69a9647d8be59f1e1"),
			},
			EventCount: 3,
		},
	}
)

func TestGetBlock(t *testing.T) {
	err := db.InitializeDatabaseEnv(t.TempDir(), 3, 0)
	if err != nil {
		t.Error(err)
	}
	txDb, err := db.GetDatabase("TRANSACTION")
	if err != nil {
		t.Error(err)
	}
	receiptDb, err := db.GetDatabase("RECEIPT")
	if err != nil {
		t.Error(err)
	}
	blockDb, err := db.GetDatabase("BLOCK")
	if err != nil {
		t.Error(err)
	}
	// Initialize transaction service
	services.TransactionService.Setup(txDb, receiptDb)
	if err := services.TransactionService.Run(); err != nil {
		t.Fatalf("unexpected error starting the transaction service: %s", err)
	}
	defer services.TransactionService.Close(context.Background())
	// Initialize block service
	services.BlockService.Setup(blockDb)
	if err := services.BlockService.Run(); err != nil {
		t.Fatalf("unexpeceted error starting the block service: %s", err)
	}
	defer services.BlockService.Close(context.Background())
	// Store transactions
	for _, txn := range txns {
		services.TransactionService.StoreTransaction(txn.GetHash(), txn)
	}
	// Store receipts
	for _, receipt := range receipts {
		services.TransactionService.StoreReceipt(receipt.TxHash, receipt)
	}
	// Store blocks
	for _, block := range blocks {
		services.BlockService.StoreBlock(block.BlockHash, &block)
	}

	type testItem struct {
		Scope    RequestedScope
		Response BlockResponse
	}
	// Test requests
	response := make(map[string][]testItem)

	response["0x1cf10396b725510b794f03d90bd670d463747ece48c94e36ac9e04b9ec122b6"] = []testItem{
		{
			Scope: ScopeTxnHash,
			Response: BlockResponse{
				BlockHash:    types.HexToBlockHash("0x1cf10396b725510b794f03d90bd670d463747ece48c94e36ac9e04b9ec122b6"),
				ParentHash:   types.HexToBlockHash("0x2439288f35c3da4a8aa3f689ddcf6f83fd9bdc9357c04d12265501e68e14d64"),
				BlockNumber:  2591,
				Status:       types.BlockStatusAcceptedOnL1,
				Sequencer:    types.HexToAddress("0x21f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5"),
				NewRoot:      types.HexToFelt("0166c5ae3f0f631a4d5543d447aa4cee6a975b2e83070d8986c94952aea10aae"),
				OldRoot:      types.HexToFelt("06d8106c901208726feea70d1155d085aee27e63ae0368b9ecc1c3bf1b5d4fe8"),
				AcceptedTime: 1654588282,
				Transactions: []types.TransactionHash{
					txns[0].GetHash(),
					txns[1].GetHash(),
					txns[2].GetHash(),
				},
			},
		},
		{
			Scope: ScopeFullTxns,
			Response: BlockResponse{
				BlockHash:    types.HexToBlockHash("0x1cf10396b725510b794f03d90bd670d463747ece48c94e36ac9e04b9ec122b6"),
				ParentHash:   types.HexToBlockHash("0x2439288f35c3da4a8aa3f689ddcf6f83fd9bdc9357c04d12265501e68e14d64"),
				BlockNumber:  2591,
				Status:       types.BlockStatusAcceptedOnL1,
				Sequencer:    types.HexToAddress("0x21f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5"),
				NewRoot:      types.HexToFelt("0166c5ae3f0f631a4d5543d447aa4cee6a975b2e83070d8986c94952aea10aae"),
				OldRoot:      types.HexToFelt("06d8106c901208726feea70d1155d085aee27e63ae0368b9ecc1c3bf1b5d4fe8"),
				AcceptedTime: 1654588282,
				Transactions: []*Txn{
					NewTxn(txns[0]),
					NewTxn(txns[1]),
					NewTxn(txns[2]),
				},
			},
		},
		{
			Scope: ScopeFullTxnAndReceipts,
			Response: BlockResponse{
				BlockHash:    types.HexToBlockHash("0x1cf10396b725510b794f03d90bd670d463747ece48c94e36ac9e04b9ec122b6"),
				ParentHash:   types.HexToBlockHash("0x2439288f35c3da4a8aa3f689ddcf6f83fd9bdc9357c04d12265501e68e14d64"),
				BlockNumber:  2591,
				Status:       types.BlockStatusAcceptedOnL1,
				Sequencer:    types.HexToAddress("0x21f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5"),
				NewRoot:      types.HexToFelt("0166c5ae3f0f631a4d5543d447aa4cee6a975b2e83070d8986c94952aea10aae"),
				OldRoot:      types.HexToFelt("06d8106c901208726feea70d1155d085aee27e63ae0368b9ecc1c3bf1b5d4fe8"),
				AcceptedTime: 1654588282,
				Transactions: []*TxnAndReceipt{
					{
						Txn:        *NewTxn(txns[0]),
						TxnReceipt: *NewTxnReceipt(receipts[0]),
					},
					{
						Txn:        *NewTxn(txns[1]),
						TxnReceipt: *NewTxnReceipt(receipts[1]),
					},
					{
						Txn:        *NewTxn(txns[2]),
						TxnReceipt: *NewTxnReceipt(receipts[2]),
					},
				},
			},
		},
	}

	for _, block := range blocks {
		blockTest := response[block.BlockHash.Hex()]
		for _, item := range blockTest {
			testServer(t, []rpcTest{
				{
					Request:  buildRequest("starknet_getBlockByHash", block.BlockHash, item.Scope),
					Response: buildResponse(item.Response),
				},
				{
					Request:  buildRequest("starknet_getBlockByNumber", block.BlockNumber, item.Scope),
					Response: buildResponse(item.Response),
				},
			})
		}
	}

	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)
	body, err := json.Marshal(feeder.StarknetBlock{
		BlockHash:        "a",
		ParentBlockHash:  "a",
		BlockNumber:      0,
		GasPrice:         "a",
		SequencerAddress: "a",
		StateRoot:        "a",
		Status:           "a",
		Timestamp:        0,
		Transactions:     []feeder.TxnSpecificInfo{},
	})
	if err != nil {
		t.Fatal("unexpected marshal error")
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error while initializing fake data in feeder client")
	}
	want := &BlockResponse{
		BlockHash:    types.HexToBlockHash("a"),
		ParentHash:   types.HexToBlockHash("a"),
		BlockNumber:  0,
		Status:       types.StringToBlockStatus("UNKNOWN"),
		Sequencer:    types.HexToAddress("a"),
		NewRoot:      types.HexToFelt("a"),
		Transactions: []*Txn{},
	}
	// Test request for pending block
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_getBlockByNumber", BlocktagPending, ""), // scope is irrelevant
			Response: buildResponse(want),
		},
	})
}

// generateResponse returns a HTTP 200 response
func generateResponse(body string) *http.Response {
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
		Header:        make(http.Header, 0),
	}
}

func TestGetBlockByTag(t *testing.T) {
	// Reassign global feederClient with fake http client
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)

	tx := Txn{
		FunctionCall: FunctionCall{
			ContractAddress:    types.HexToAddress("a"),
			EntryPointSelector: types.HexToFelt("a"),
			CallData:           []types.Felt{types.HexToFelt("a")},
		},
		TxnHash: types.HexToTransactionHash("a"),
	}

	tests := [...]struct {
		scope RequestedScope
		want  interface{}
	}{
		{
			scope: ScopeTxnHash,
			want:  []*types.TransactionHash{&tx.TxnHash},
		},
		{
			scope: ScopeFullTxns,
			want:  []*Txn{&tx},
		},
		{
			scope: ScopeFullTxnAndReceipts,
			want: []*TxnAndReceipt{
				{
					Txn: tx,
					TxnReceipt: TxnReceipt{
						TxnHash:    types.HexToTransactionHash("a"),
						Status:     types.TxStatusUnknown,
						StatusData: "",
						MessagesSent: []*MsgToL1{
							{
								ToAddress: types.HexToEthAddress("a"),
								Payload:   []types.Felt{types.HexToFelt("a")},
							},
						},
						L1OriginMessage: &MsgToL2{
							FromAddress: types.HexToEthAddress("a"),
							Payload:     []types.Felt{types.HexToFelt("a")},
						},
						Events: []*Event{
							{
								FromAddress: types.HexToAddress("a"),
								EventContent: EventContent{
									Keys: []types.Felt{types.HexToFelt("a")},
									Data: []types.Felt{types.HexToFelt("a")},
								},
							},
						},
					},
				},
			},
		},
	}

	// TODO get an actual block for this. Make data more realistic
	fakeBlock := feeder.StarknetBlock{
		BlockHash:        "a",
		ParentBlockHash:  "a",
		BlockNumber:      0,
		GasPrice:         "a",
		SequencerAddress: "a",
		StateRoot:        "a",
		Status:           "a",
		Timestamp:        0,
		Transactions: []feeder.TxnSpecificInfo{
			{
				Calldata:           []string{"a"},
				ContractAddress:    "a",
				EntryPointSelector: "a",
				EntryPointType:     "a",
				Signature:          []string{"a"},
				TransactionHash:    "a",
				Type:               "INVOKE",
			},
		},
		TransactionReceipts: []feeder.TransactionExecution{
			{
				TransactionIndex: 0,
				TransactionHash:  "a",
				L1ToL2Message: feeder.L1ToL2Message{
					FromAddress: "a",
					ToAddress:   "a",
					Selector:    "a",
					Payload:     []string{"a"},
					Nonce:       "0",
				},
				L2ToL1Messages: []feeder.L2ToL1Message{
					{
						FromAddress: "a",
						ToAddress:   "a",
						Payload:     []string{"a"},
					},
				},
				Events: []feeder.Event{
					{
						FromAddress: "a",
						Keys:        []string{"a"},
						Data:        []string{"a"},
					},
				},
				ExecutionResources: feeder.ExecutionResources{
					NSteps:                 0,
					BuiltinInstanceCounter: map[string]int64{"a": 0},
					NMemoryHoles:           0,
				},
				ActualFee: "0",
			},
		},
	}

	want := &BlockResponse{
		BlockHash:   types.HexToBlockHash("a"),
		ParentHash:  types.HexToBlockHash("a"),
		BlockNumber: 0,
		Status:      types.StringToBlockStatus("UNKNOWN"),
		Sequencer:   types.HexToAddress("a"),
		NewRoot:     types.HexToFelt("a"),
	}

	for _, test := range tests {
		t.Run(string(test.scope), func(t *testing.T) {
			body, err := json.Marshal(fakeBlock)
			if err != nil {
				t.Fatal("unexpected marshal error", err)
			}
			fakeClient.DoReturns(generateResponse(string(body)), nil)
			if err != nil {
				t.Fatal("unexpected error when calling `feeder.DoReturns`", err)
			}

			want.Transactions = test.want
			response, err := getBlockByTag(context.Background(), "", test.scope)
			if err != nil {
				t.Fatal("unexpected error when calling `getBlockByTag`", err)
			}
			assert.DeepEqual(t, response, want)
		})
	}
}

func TestGetTransactionByHash(t *testing.T) {
	err := db.InitializeDatabaseEnv(t.TempDir(), 2, 0)
	if err != nil {
		t.Error(err)
	}
	txDb, err := db.GetDatabase("TRANSACTION")
	if err != nil {
		t.Error(err)
	}
	receiptDb, err := db.GetDatabase("RECEIPT")
	if err != nil {
		t.Error(err)
	}
	services.TransactionService.Setup(txDb, receiptDb)
	if err := services.TransactionService.Run(); err != nil {
		t.Fatalf("unexpected error starting the transaction service: %s", err)
	}
	defer services.TransactionService.Close(context.Background())
	responses := make(map[string]*Txn)
	for _, txn := range txns {
		services.TransactionService.StoreTransaction(txn.GetHash(), txn)
		responses[txn.GetHash().String()] = NewTxn(txn)
	}
	for _, txn := range txns {
		testServer(t, []rpcTest{
			{
				Request:  buildRequest("starknet_getTransactionByHash", txn.GetHash()),
				Response: buildResponse(responses[txn.GetHash().String()]),
			},
		})
	}
}

func TestStarknetPendingTransactions(t *testing.T) {
	// Reassign global feederClient with fake http client
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)

	// Generate fake response
	x := feeder.StarknetBlock{
		Transactions: []feeder.TxnSpecificInfo{
			{
				Calldata:           []string{"a"},
				ContractAddress:    "a",
				EntryPointSelector: "a",
				EntryPointType:     "a",
				Signature:          []string{"a"},
				TransactionHash:    "a",
				Type:               "INVOKE",
			},
		},
	}
	body, err := json.Marshal(x)
	if err != nil {
		t.Fatal("unexpected marshal error")
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error while initializing fake data in feeder client")
	}

	// Test
	want := []*Txn{
		{
			FunctionCall: FunctionCall{
				ContractAddress:    types.HexToAddress("a"),
				EntryPointSelector: types.HexToFelt("a"),
				CallData:           []types.Felt{types.HexToFelt("a")},
			},
			TxnHash: types.HexToTransactionHash("a"),
		},
	}
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_pendingTransactions"),
			Response: buildResponse(want),
		},
	})
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
	err := db.InitializeDatabaseEnv(t.TempDir(), 3, 0)
	if err != nil {
		t.Error(err)
	}
	txDb, err := db.GetDatabase("TRANSACTION")
	if err != nil {
		t.Error(err)
	}
	receiptDb, err := db.GetDatabase("RECEIPT")
	if err != nil {
		t.Error(err)
	}
	blockDb, err := db.GetDatabase("BLOCK")
	if err != nil {
		t.Error(err)
	}
	// Initialize transaction service
	services.TransactionService.Setup(txDb, receiptDb)
	if err := services.TransactionService.Run(); err != nil {
		t.Fatalf("unexpected error starting the transaction service: %s", err)
	}
	defer services.TransactionService.Close(context.Background())
	// Initialize block service
	services.BlockService.Setup(blockDb)
	if err := services.BlockService.Run(); err != nil {
		t.Fatalf("unexpeceted error starting the block service: %s", err)
	}
	defer services.BlockService.Close(context.Background())
	// Store transactions
	for _, txn := range txns {
		services.TransactionService.StoreTransaction(txn.GetHash(), txn)
	}
	// Store blocks
	for _, block := range blocks {
		services.BlockService.StoreBlock(block.BlockHash, &block)
	}
	// Build test cases
	tests := make([]rpcTest, 0)
	for _, block := range blocks {
		for i := range block.TxHashes {
			tests = append(tests, rpcTest{
				Request:  buildRequest("starknet_getTransactionByBlockHashAndIndex", block.BlockHash.Hex(), i),
				Response: buildResponse(NewTxn(txns[i])),
			})
		}
	}

	// Make test case for pending block
	fakeBlock := feeder.StarknetBlock{
		BlockHash:        "a",
		ParentBlockHash:  "a",
		BlockNumber:      0,
		GasPrice:         "a",
		SequencerAddress: "a",
		StateRoot:        "a",
		Status:           "a",
		Timestamp:        0,
		Transactions: []feeder.TxnSpecificInfo{
			{
				Calldata:           []string{"a"},
				ContractAddress:    "a",
				EntryPointSelector: "a",
				EntryPointType:     "a",
				Signature:          []string{"a"},
				TransactionHash:    "a",
				Type:               "INVOKE",
			},
		},
	}
	// Set up feeder client for PENDING block
	body, err := json.Marshal(fakeBlock)
	if err != nil {
		t.Fatal("unexpected marshal error", err)
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error when calling `feeder.DoReturns`", err)
	}
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)
	tx := Txn{
		FunctionCall: FunctionCall{
			ContractAddress:    types.HexToAddress("a"),
			EntryPointSelector: types.HexToFelt("a"),
			CallData:           []types.Felt{types.HexToFelt("a")},
		},
		TxnHash: types.HexToTransactionHash("a"),
	}
	tests = append(tests, rpcTest{
		Request:  buildRequest("starknet_getTransactionByBlockHashAndIndex", BlocktagPending, 0),
		Response: buildResponse(tx),
	})

	// Run tests
	testServer(t, tests)
}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {
	err := db.InitializeDatabaseEnv(t.TempDir(), 3, 0)
	if err != nil {
		t.Error(err)
	}
	txDb, err := db.GetDatabase("TRANSACTION")
	if err != nil {
		t.Error(err)
	}
	receiptDb, err := db.GetDatabase("RECEIPT")
	if err != nil {
		t.Error(err)
	}
	blockDb, err := db.GetDatabase("BLOCK")
	if err != nil {
		t.Error(err)
	}
	// Initialize transaction service
	services.TransactionService.Setup(txDb, receiptDb)
	if err := services.TransactionService.Run(); err != nil {
		t.Fatalf("unexpected error starting the transaction service: %s", err)
	}
	defer services.TransactionService.Close(context.Background())
	// Initialize block service
	services.BlockService.Setup(blockDb)
	if err := services.BlockService.Run(); err != nil {
		t.Fatalf("unexpeceted error starting the block service: %s", err)
	}
	defer services.BlockService.Close(context.Background())
	// Store transactions
	for _, txn := range txns {
		services.TransactionService.StoreTransaction(txn.GetHash(), txn)
	}
	// Store blocks
	for _, block := range blocks {
		services.BlockService.StoreBlock(block.BlockHash, &block)
	}
	// Build test cases
	tests := make([]rpcTest, 0)
	for _, block := range blocks {
		for i := range block.TxHashes {
			tests = append(tests, rpcTest{
				Request:  buildRequest("starknet_getTransactionByBlockNumberAndIndex", block.BlockNumber, i),
				Response: buildResponse(NewTxn(txns[i])),
			})
		}
	}

	// Make test case for pending block
	fakeBlock := feeder.StarknetBlock{
		BlockHash:        "a",
		ParentBlockHash:  "a",
		BlockNumber:      0,
		GasPrice:         "a",
		SequencerAddress: "a",
		StateRoot:        "a",
		Status:           "a",
		Timestamp:        0,
		Transactions: []feeder.TxnSpecificInfo{
			{
				Calldata:           []string{"a"},
				ContractAddress:    "a",
				EntryPointSelector: "a",
				EntryPointType:     "a",
				Signature:          []string{"a"},
				TransactionHash:    "a",
				Type:               "INVOKE",
			},
		},
	}

	// Set up feeder client for PENDING block
	body, err := json.Marshal(fakeBlock)
	if err != nil {
		t.Fatal("unexpected marshal error", err)
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error when calling `feeder.DoReturns`", err)
	}
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)
	tx := Txn{
		FunctionCall: FunctionCall{
			ContractAddress:    types.HexToAddress("a"),
			EntryPointSelector: types.HexToFelt("a"),
			CallData:           []types.Felt{types.HexToFelt("a")},
		},
		TxnHash: types.HexToTransactionHash("a"),
	}
	tests = append(tests, rpcTest{
		Request:  buildRequest("starknet_getTransactionByBlockNumberAndIndex", BlocktagPending, 0),
		Response: buildResponse(tx),
	})

	// Run tests
	testServer(t, tests)
}

func TestStarknetCall(t *testing.T) {
	funcCall := FunctionCall{
		ContractAddress:    types.HexToAddress("a"),
		EntryPointSelector: types.HexToFelt("a"),
		CallData:           []types.Felt{types.HexToFelt("a")},
	}

	// Reassign global feederClient with fake http client
	feederClient = feeder.NewClient("https://localhost:8100", "/feeder_gateway", &client)

	// Generate fake response
	x := &map[string][]string{"result": {"a"}}
	body, err := json.Marshal(x)
	if err != nil {
		t.Fatal("unexpected marshal error")
	}
	fakeClient.DoReturns(generateResponse(string(body)), nil)
	if err != nil {
		t.Fatal("unexpected error while initializing fake data in feeder client")
	}

	// Test
	testServer(t, []rpcTest{
		{
			Request:  buildRequest("starknet_call", funcCall, BlocktagPending),
			Response: buildResponse(ResultCall{"a"}),
		},
	})
}
