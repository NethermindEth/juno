package feeder_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/NethermindEth/juno/utils"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

const mockUrl = "https://mock_feeder.io/"

func testClient(url string) *feeder.Client {
	return feeder.NewClient(url).WithBackoff(feeder.NopBackoff).WithMaxRetries(0)
}

func TestStateUpdateUnmarshal(t *testing.T) {
	jsonData := []byte(`{
  "block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
  "new_root": "021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6",
  "old_root": "0000000000000000000000000000000000000000000000000000000000000000",
  "state_diff": {
    "storage_diffs": {
      "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": [
        {
          "key": "0x5",
          "value": "0x22b"
        }
      ]
    },
    "nonces": { 
		"0x37" : "0x44"
	},
    "deployed_contracts": [
      {
        "address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
        "class_hash": "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"
      }
	],
    "declared_contracts": [
		"0x3744"
	]
  }
}`)

	var update feeder.StateUpdate
	err := json.Unmarshal(jsonData, &update)
	assert.Equal(t, nil, err, "Unexpected error")
	expected, _ := new(felt.Felt).SetString("0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943")
	assert.Equal(t, true, update.BlockHash.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
	assert.Equal(t, true, update.NewRoot.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, true, update.OldRoot.Equal(expected))
	assert.Equal(t, 1, len(update.StateDiff.StorageDiffs))

	diffs, found := update.StateDiff.StorageDiffs["0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6"]
	assert.Equal(t, true, found)
	assert.Equal(t, 1, len(diffs))

	expected, _ = new(felt.Felt).SetString("0x5")
	assert.Equal(t, true, diffs[0].Key.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x22b")
	assert.Equal(t, true, diffs[0].Value.Equal(expected))

	assert.Equal(t, 1, len(update.StateDiff.DeployedContracts))
	expected, _ = new(felt.Felt).SetString("0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].Address.Equal(expected))
	expected, _ = new(felt.Felt).SetString("0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].ClassHash.Equal(expected))

	assert.Equal(t, 1, len(update.StateDiff.DeclaredContracts))
	expected, _ = new(felt.Felt).SetString("0x3744")
	assert.Equal(t, true, update.StateDiff.DeclaredContracts[0].Equal(expected))

	assert.Equal(t, 1, len(update.StateDiff.Nonces))
	expected, _ = new(felt.Felt).SetString("0x44")
	value, ok := update.StateDiff.Nonces["0x37"]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, value.Equal(expected))
}

func TestDeclareTransactionUnmarshal(t *testing.T) {
	declareJson := []byte(`{
      "transaction_hash":"0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d",
      "version":"0x1",
      "max_fee":"0x5af3107a4000",
      "signature":[
         "0x516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff",
         "0x6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda"
      ],
      "nonce":"0x1d",
      "class_hash":"0x2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453",
      "sender_address":"0xb8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23",
      "type":"DECLARE"
   }`)
	var declareTx feeder.Transaction
	err := json.Unmarshal(declareJson, &declareTx)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d", declareTx.Hash.String())
	assert.Equal(t, "0x1", declareTx.Version.String())
	assert.Equal(t, "0x5af3107a4000", declareTx.MaxFee.String())
	assert.Equal(t, "0x1d", declareTx.Nonce.String())
	assert.Equal(t, "0x2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453", declareTx.ClassHash.String())
	assert.Equal(t, "0xb8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23", declareTx.SenderAddress.String())
	assert.Equal(t, "DECLARE", declareTx.Type)
	assert.Equal(t, 2, len(declareTx.Signature))
	assert.Equal(t, "0x516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff", declareTx.Signature[0].String())
	assert.Equal(t, "0x6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda", declareTx.Signature[1].String())
}

func TestInvokeTransactionUnmarshal(t *testing.T) {
	invokeJson := []byte(`{
      "transaction_hash":"0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df",
      "version":"0x44",
      "max_fee":"0x37",
      "signature":[

      ],
      "contract_address":"0x17daeb497b6fe0f7adaa32b44677c3a9712b6856b792ad993fcef20aed21ac8",
      "entry_point_selector":"0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64",
      "calldata":[
         "0x346f2b6376b4b57f714ba187716fce9edff1361628cc54783ed0351538faa5e",
         "0x2"
      ],
      "type":"INVOKE_FUNCTION"
   }`)
	var invokeTx feeder.Transaction
	err := json.Unmarshal(invokeJson, &invokeTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df", invokeTx.Hash.String())
	assert.Equal(t, "0x44", invokeTx.Version.String())
	assert.Equal(t, "0x37", invokeTx.MaxFee.String())
	assert.Equal(t, 0, len(invokeTx.Signature))
	assert.Equal(t, "0x17daeb497b6fe0f7adaa32b44677c3a9712b6856b792ad993fcef20aed21ac8", invokeTx.ContractAddress.String())
	assert.Equal(t, "0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64", invokeTx.EntryPointSelector.String())
	assert.Equal(t, 2, len(invokeTx.CallData))
	assert.Equal(t, "0x346f2b6376b4b57f714ba187716fce9edff1361628cc54783ed0351538faa5e", invokeTx.CallData[0].String())
	assert.Equal(t, "0x2", invokeTx.CallData[1].String())
	assert.Equal(t, "INVOKE_FUNCTION", invokeTx.Type)
}

func TestDeployTransactionUnmarshal(t *testing.T) {
	deployJson := []byte(`{
      "transaction_hash":"0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee",
      "version":"0x0",
      "contract_address":"0x7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc",
      "contract_address_salt":"0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8",
      "class_hash":"0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e",
      "constructor_calldata":[
         "0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a",
         "0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a",
         "0x1",
         "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8"
      ],
      "type":"DEPLOY"
   }`)
	var deployTx feeder.Transaction
	err := json.Unmarshal(deployJson, &deployTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee", deployTx.Hash.String())
	assert.Equal(t, "0x0", deployTx.Version.String())
	assert.Equal(t, "0x7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc", deployTx.ContractAddress.String())
	assert.Equal(t, "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", deployTx.ContractAddressSalt.String())
	assert.Equal(t, "0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e", deployTx.ClassHash.String())
	assert.Equal(t, 4, len(deployTx.ConstructorCallData))
	assert.Equal(t, "0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a", deployTx.ConstructorCallData[0].String())
	assert.Equal(t, "0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a", deployTx.ConstructorCallData[1].String())
	assert.Equal(t, "0x1", deployTx.ConstructorCallData[2].String())
	assert.Equal(t, "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", deployTx.ConstructorCallData[3].String())
	assert.Equal(t, "DEPLOY", deployTx.Type)
}

func TestDeployAccountTransactionUnmarshal(t *testing.T) {
	deployJson := []byte(`{
      "transaction_hash":"0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e",
      "version":"0x1",
      "max_fee":"0x59f5f9f474b0",
      "signature":[
         "0x467ae89bbbbaa0139e8f8a02ddc614bd80252998f3c033239f59f9f2ab973c5",
         "0x92938929b5afcd596d651a6d28ed38baf90b000192897617d98de19d475331"
      ],
      "nonce":"0x0",
      "contract_address":"0x104714313388bd0ab569ac247fed6cf0b7a2c737105c00d64c23e24bd8dea40",
      "contract_address_salt":"0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3",
      "class_hash":"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
      "constructor_calldata":[
         "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2",
         "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
         "0x2",
         "0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3",
         "0x0"
      ],
      "type":"DEPLOY_ACCOUNT"
   }`)
	var deployTx feeder.Transaction
	err := json.Unmarshal(deployJson, &deployTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e", deployTx.Hash.String())
	assert.Equal(t, "0x1", deployTx.Version.String())
	assert.Equal(t, "0x59f5f9f474b0", deployTx.MaxFee.String())
	assert.Equal(t, 2, len(deployTx.Signature))
	assert.Equal(t, "0x467ae89bbbbaa0139e8f8a02ddc614bd80252998f3c033239f59f9f2ab973c5", deployTx.Signature[0].String())
	assert.Equal(t, "0x92938929b5afcd596d651a6d28ed38baf90b000192897617d98de19d475331", deployTx.Signature[1].String())
	assert.Equal(t, "0x0", deployTx.Nonce.String())
	assert.Equal(t, "0x104714313388bd0ab569ac247fed6cf0b7a2c737105c00d64c23e24bd8dea40", deployTx.ContractAddress.String())
	assert.Equal(t, "0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", deployTx.ContractAddressSalt.String())
	assert.Equal(t, "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", deployTx.ClassHash.String())

	assert.Equal(t, 5, len(deployTx.ConstructorCallData))
	assert.Equal(t, "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", deployTx.ConstructorCallData[0].String())
	assert.Equal(t, "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463", deployTx.ConstructorCallData[1].String())
	assert.Equal(t, "0x2", deployTx.ConstructorCallData[2].String())
	assert.Equal(t, "0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", deployTx.ConstructorCallData[3].String())
	assert.Equal(t, "0x0", deployTx.ConstructorCallData[4].String())
	assert.Equal(t, "DEPLOY_ACCOUNT", deployTx.Type)
}

func TestL1HandlerTransactionUnmarshal(t *testing.T) {
	handlerJson := []byte(`{
      "transaction_hash":"0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190",
      "version":"0x0",
      "contract_address":"0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82",
      "entry_point_selector":"0x2d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5",
      "nonce":"0x1654d",
      "calldata":[
         "0xae0ee0a63a2ce6baeeffe56e7714fb4efe48d419",
         "0x218559e75713ca564d6eaf043b73388e9ac7c2f459ef8905988052051d3ef5e",
         "0x2386f26fc10000",
         "0x0"
      ],
      "type":"L1_HANDLER"
   }`)
	var handlerTx feeder.Transaction
	err := json.Unmarshal(handlerJson, &handlerTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190", handlerTx.Hash.String())
	assert.Equal(t, "0x0", handlerTx.Version.String())
	assert.Equal(t, "0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82", handlerTx.ContractAddress.String())
	assert.Equal(t, "0x2d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5", handlerTx.EntryPointSelector.String())
	assert.Equal(t, "0x1654d", handlerTx.Nonce.String())
	assert.Equal(t, 4, len(handlerTx.CallData))
	assert.Equal(t, "0xae0ee0a63a2ce6baeeffe56e7714fb4efe48d419", handlerTx.CallData[0].String())
	assert.Equal(t, "0x218559e75713ca564d6eaf043b73388e9ac7c2f459ef8905988052051d3ef5e", handlerTx.CallData[1].String())
	assert.Equal(t, "0x2386f26fc10000", handlerTx.CallData[2].String())
	assert.Equal(t, "0x0", handlerTx.CallData[3].String())
	assert.Equal(t, "L1_HANDLER", handlerTx.Type)
}

func TestBlockWithoutSequencerAddressUnmarshal(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()

	block, err := client.Block(context.Background(), 11817)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "0x24c692acaed3b486990bd9d2b2fbbee802b37b3bd79c59f295bad3277200a83", block.Hash.String())
	assert.Equal(t, "0x3873ccb937f14429b169c654dda28886d2cc2d6ea17b3cff9748fe5cfdb67e0", block.ParentHash.String())
	assert.Equal(t, uint64(11817), block.Number)
	assert.Equal(t, "0x3df24be7b5fed6b41de08d38686b6142944119ca2a345c38793590d6804bba4", block.StateRoot.String())
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x27ad16775", block.GasPrice.String())
	assert.Equal(t, 52, len(block.Transactions))
	assert.Equal(t, 52, len(block.Receipts))
	assert.Equal(t, uint64(1669465009), block.Timestamp)
	assert.Equal(t, "0.10.1", block.Version)
}

func TestBlockWithSequencerAddressUnmarshal(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()

	block, err := client.Block(context.Background(), 19199)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "0x41811b69473f26503e0375806ee97d05951ccc7840e3d2bbe14ffb2522e5be1", block.Hash.String())
	assert.Equal(t, "0x68427fb6f1f5e687fbd779b3cc0d4ee31b49575ed0f8c749f827e4a45611efc", block.ParentHash.String())
	assert.Equal(t, uint64(19199), block.Number)
	assert.Equal(t, "0x541b796ea02703d02ff31459815f65f410ceefe80a4e3499f7ef9ccc36d26ee", block.StateRoot.String())
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x31c4e2d75", block.GasPrice.String())
	assert.Equal(t, 324, len(block.Transactions))
	assert.Equal(t, 324, len(block.Receipts))
	assert.Equal(t, uint64(1674728186), block.Timestamp)
	assert.Equal(t, "0.10.3", block.Version)
	assert.Equal(t, "0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9", block.SequencerAddress.String())
}

func TestClassUnmarshal(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()

	hash, _ := new(felt.Felt).SetString("0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f")
	class, err := client.ClassDefinition(context.Background(), hash)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 1, len(class.EntryPoints.Constructor))
	assert.Equal(t, "0xa1", class.EntryPoints.Constructor[0].Offset.String())
	assert.Equal(t, "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", class.EntryPoints.Constructor[0].Selector.String())
	assert.Equal(t, 1, len(class.EntryPoints.L1Handler))
	assert.Equal(t, 1, len(class.EntryPoints.External))
	assert.Equal(t, 250, len(class.Program.Data))
	assert.Equal(t, []string{"pedersen", "range_check"}, class.Program.Builtins)
	assert.Equal(t, "0.10.1", class.Program.CompilerVersion)
}

func TestBuildQueryString_WithErrorUrl(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	baseUrl := "https\t://mock_feeder.io"
	client := feeder.NewClient(baseUrl)
	client.Block(context.Background(), 0)
}

func TestStateUpdate(t *testing.T) {
	jsonData := []byte(`{
		"block_hash": "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
		"new_root": "021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6",
		"old_root": "0000000000000000000000000000000000000000000000000000000000000000",
		"state_diff": {
		  "storage_diffs": {
			"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": [
			  {
				"key": "0x5",
				"value": "0x22b"
			  }
			]
		  },
		  "nonces": {
			"0x37" : "0x44"
		  },
		  "deployed_contracts": [
			{
			  "address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			  "class_hash": "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"
			}
		  ],
		  "declared_contracts": [
				"0x3744"
		  ]
		}
	  }`)

	var update feeder.StateUpdate
	err := json.Unmarshal(jsonData, &update)
	assert.Equal(t, nil, err, "Unexpected error")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get_state_update":
			{
				queryMap, err := url.ParseQuery(r.URL.RawQuery)
				assert.Equal(t, nil, err, "No Query value")
				queryBlockNumebr := queryMap["blockNumber"]
				t.Log(queryBlockNumebr[0])
				if queryBlockNumebr[0] == "10" {
					w.WriteHeader(200)
					marshedUpdate, _ := json.Marshal(update)
					w.Write(marshedUpdate)
				} else {
					w.WriteHeader(404)
				}
			}
		}
	}))
	defer srv.Close()
	client := testClient(srv.URL)

	t.Run("Test normal case", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(context.Background(), 10)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.Equal(t, update, *stateUpdate)
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(context.Background(), 1000000)
		assert.Nil(t, stateUpdate, "Unexpected error")
		assert.NotNil(t, err)
	})
}

func TestTransaction(t *testing.T) {
	jsonTransactionStatus := []byte(`{
		"block_hash": "0x0",
		"block_number": 0,
		"status": "ACCEPTED_ON_L2",
		"transaction": {
			"calldata": [
				"0x1",
				"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
				"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320",
				"0x0",
				"0x1",
				"0x1",
				"0x4d2"
			],
			"contract_address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			"max_fee": "0x0",
			"nonce": "0x3",
			"signature": [
				"0x26ee1f973def2e6d5c7f32aaad96c84dab32df6a62ee0e8b530a72bc5478fe6",
				"0x502b15e440cc2a8a1966f0c973015096715e366fde36a4659c56f84249688e"
			],
			"transaction_hash": "0x69d743891f69d758928e163eff1e3d7256752f549f134974d4aa8d26d5d7da8",
			"type": "INVOKE_FUNCTION",
			"version": "0x1"
		},
		"transaction_index": 1
	}
	`)
	var transactionStatus feeder.TransactionStatus
	err := json.Unmarshal(jsonTransactionStatus, &transactionStatus)
	assert.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get_transaction":
			{
				queryMap, err := url.ParseQuery(r.URL.RawQuery)
				assert.Equal(t, nil, err, "No Query value")
				transactionHash := queryMap["transactionHash"]
				t.Log(transactionHash[0])
				if transactionHash[0] == "0x0" {
					w.WriteHeader(200)
					marshaledStr, _ := json.Marshal(transactionStatus)
					w.Write(marshaledStr)

				} else {
					w.WriteHeader(404)
				}
			}
		}
	}))
	defer srv.Close()
	t.Run("Test normal case", func(t *testing.T) {
		transaction_hash, _ := new(felt.Felt).SetString("0x00")
		client := testClient(srv.URL)
		actualStatus, err := client.Transaction(context.Background(), transaction_hash)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.Equal(t, transactionStatus, *actualStatus)
	})
	t.Run("Test case when transaction_hash does not exist", func(t *testing.T) {
		transaction_hash, _ := new(felt.Felt).SetString("0xffff")
		client := testClient(srv.URL)
		actualStatus, err := client.Transaction(context.Background(), transaction_hash)
		assert.Nil(t, actualStatus, "Unexpected error")
		assert.NotNil(t, err)
	})
}

func TestBlock(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()

	t.Run("Test normal case", func(t *testing.T) {
		blcokNumber := uint64(11817)
		actualBlock, err := client.Block(context.Background(), blcokNumber)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualBlock)
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		blcokNumber := uint64(1000000)

		actualBlock, err := client.Block(context.Background(), blcokNumber)
		assert.Nil(t, actualBlock, "Unexpected error")
		assert.NotNil(t, err)
	})
}

func TestClassDefinition(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	defer closeFn()

	t.Run("Test normal case", func(t *testing.T) {
		classHash, _ := new(felt.Felt).SetString("0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f")

		actualClass, err := client.ClassDefinition(context.Background(), classHash)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualClass)
	})
	t.Run("Test classHash not find", func(t *testing.T) {
		classHash, _ := new(felt.Felt).SetString("0x000")
		actualClass, err := client.ClassDefinition(context.Background(), classHash)
		assert.Nil(t, actualClass, "Unexpected error")
		assert.NotNil(t, err)
	})
}

func TestHttpError(t *testing.T) {
	maxRetries := 2
	callCount := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount[r.URL.String()]++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	client := feeder.NewClient(srv.URL).WithBackoff(feeder.NopBackoff).WithMaxRetries(maxRetries)

	t.Run("HTTP err in GetBlock", func(t *testing.T) {
		_, err := client.Block(context.Background(), 0)
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetTransaction", func(t *testing.T) {
		_, err := client.Transaction(context.Background(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetClassDefinition", func(t *testing.T) {
		_, err := client.ClassDefinition(context.Background(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetStateUpdate", func(t *testing.T) {
		_, err := client.StateUpdate(context.Background(), 0)
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	for _, called := range callCount {
		assert.Equal(t, maxRetries+1, called)
	}
}

func TestBackoffFailure(t *testing.T) {
	maxRetries := 5
	try := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		try += 1
	}))
	defer srv.Close()

	c := feeder.NewClient(srv.URL).WithBackoff(feeder.NopBackoff).WithMaxRetries(maxRetries)

	_, err := c.Block(context.Background(), 0)
	assert.EqualError(t, err, "500 Internal Server Error")
	assert.Equal(t, maxRetries, try-1) // we have retried `maxRetries` times
}
