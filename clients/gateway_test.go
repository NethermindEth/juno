package clients

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/stretchr/testify/assert"
)

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
    "nonces": {},
    "deployed_contracts": [
      {
        "address": "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
        "class_hash": "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"
      }
	],
    "declared_contracts": []
  }
}`)

	var update StateUpdate
	err := json.Unmarshal(jsonData, &update)
	assert.Equal(t, nil, err, "Unexpected error")
	expected, _ := new(fp.Element).SetString("0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943")
	assert.Equal(t, true, update.BlockHash.Equal(expected))
	expected, _ = new(fp.Element).SetString("0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
	assert.Equal(t, true, update.NewRoot.Equal(expected))
	expected, _ = new(fp.Element).SetString("0x0000000000000000000000000000000000000000000000000000000000000000")
	assert.Equal(t, true, update.OldRoot.Equal(expected))
	assert.Equal(t, 1, len(update.StateDiff.StorageDiffs))

	diffs, found := update.StateDiff.StorageDiffs["0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6"]
	assert.Equal(t, true, found)
	assert.Equal(t, 1, len(diffs))

	expected, _ = new(fp.Element).SetString("0x5")
	assert.Equal(t, true, diffs[0].Key.Equal(expected))
	expected, _ = new(fp.Element).SetString("0x22b")
	assert.Equal(t, true, diffs[0].Value.Equal(expected))

	assert.Equal(t, 1, len(update.StateDiff.DeployedContracts))
	expected, _ = new(fp.Element).SetString("0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].Address.Equal(expected))
	expected, _ = new(fp.Element).SetString("0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	assert.Equal(t, true, update.StateDiff.DeployedContracts[0].ClassHash.Equal(expected))
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
	var declareTx Transaction
	err := json.Unmarshal(declareJson, &declareTx)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d", declareTx.Hash.Text(16))
	assert.Equal(t, "1", declareTx.Version.Text(16))
	assert.Equal(t, "5af3107a4000", declareTx.MaxFee.Text(16))
	assert.Equal(t, "1d", declareTx.Nonce.Text(16))
	assert.Equal(t, "2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453", declareTx.ClassHash.Text(16))
	assert.Equal(t, "b8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23", declareTx.SenderAddress.Text(16))
	assert.Equal(t, "DECLARE", declareTx.Type)
	assert.Equal(t, 2, len(declareTx.Signature))
	assert.Equal(t, "516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff", declareTx.Signature[0].Text(16))
	assert.Equal(t, "6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda", declareTx.Signature[1].Text(16))
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
	var invokeTx Transaction
	err := json.Unmarshal(invokeJson, &invokeTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df", invokeTx.Hash.Text(16))
	assert.Equal(t, "44", invokeTx.Version.Text(16))
	assert.Equal(t, "37", invokeTx.MaxFee.Text(16))
	assert.Equal(t, 0, len(invokeTx.Signature))
	assert.Equal(t, "17daeb497b6fe0f7adaa32b44677c3a9712b6856b792ad993fcef20aed21ac8", invokeTx.ContractAddress.Text(16))
	assert.Equal(t, "218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64", invokeTx.EntryPointSelector.Text(16))
	assert.Equal(t, 2, len(invokeTx.Calldata))
	assert.Equal(t, "346f2b6376b4b57f714ba187716fce9edff1361628cc54783ed0351538faa5e", invokeTx.Calldata[0].Text(16))
	assert.Equal(t, "2", invokeTx.Calldata[1].Text(16))
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
	var deployTx Transaction
	err := json.Unmarshal(deployJson, &deployTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee", deployTx.Hash.Text(16))
	assert.Equal(t, "0", deployTx.Version.Text(16))
	assert.Equal(t, "7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc", deployTx.ContractAddress.Text(16))
	assert.Equal(t, "614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", deployTx.ContractAddressSalt.Text(16))
	assert.Equal(t, "3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e", deployTx.ClassHash.Text(16))
	assert.Equal(t, 4, len(deployTx.ConstructorCalldata))
	assert.Equal(t, "69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a", deployTx.ConstructorCalldata[0].Text(16))
	assert.Equal(t, "2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a", deployTx.ConstructorCalldata[1].Text(16))
	assert.Equal(t, "1", deployTx.ConstructorCalldata[2].Text(16))
	assert.Equal(t, "614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", deployTx.ConstructorCalldata[3].Text(16))
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
	var deployTx Transaction
	err := json.Unmarshal(deployJson, &deployTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e", deployTx.Hash.Text(16))
	assert.Equal(t, "1", deployTx.Version.Text(16))
	assert.Equal(t, "59f5f9f474b0", deployTx.MaxFee.Text(16))
	assert.Equal(t, 2, len(deployTx.Signature))
	assert.Equal(t, "467ae89bbbbaa0139e8f8a02ddc614bd80252998f3c033239f59f9f2ab973c5", deployTx.Signature[0].Text(16))
	assert.Equal(t, "92938929b5afcd596d651a6d28ed38baf90b000192897617d98de19d475331", deployTx.Signature[1].Text(16))
	assert.Equal(t, "0", deployTx.Nonce.Text(16))
	assert.Equal(t, "104714313388bd0ab569ac247fed6cf0b7a2c737105c00d64c23e24bd8dea40", deployTx.ContractAddress.Text(16))
	assert.Equal(t, "25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", deployTx.ContractAddressSalt.Text(16))
	assert.Equal(t, "25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", deployTx.ClassHash.Text(16))

	assert.Equal(t, 5, len(deployTx.ConstructorCalldata))
	assert.Equal(t, "33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", deployTx.ConstructorCalldata[0].Text(16))
	assert.Equal(t, "79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463", deployTx.ConstructorCalldata[1].Text(16))
	assert.Equal(t, "2", deployTx.ConstructorCalldata[2].Text(16))
	assert.Equal(t, "25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", deployTx.ConstructorCalldata[3].Text(16))
	assert.Equal(t, "0", deployTx.ConstructorCalldata[4].Text(16))
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
	var handlerTx Transaction
	err := json.Unmarshal(handlerJson, &handlerTx)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190", handlerTx.Hash.Text(16))
	assert.Equal(t, "0", handlerTx.Version.Text(16))
	assert.Equal(t, "73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82", handlerTx.ContractAddress.Text(16))
	assert.Equal(t, "2d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5", handlerTx.EntryPointSelector.Text(16))
	assert.Equal(t, "1654d", handlerTx.Nonce.Text(16))
	assert.Equal(t, 4, len(handlerTx.Calldata))
	assert.Equal(t, "ae0ee0a63a2ce6baeeffe56e7714fb4efe48d419", handlerTx.Calldata[0].Text(16))
	assert.Equal(t, "218559e75713ca564d6eaf043b73388e9ac7c2f459ef8905988052051d3ef5e", handlerTx.Calldata[1].Text(16))
	assert.Equal(t, "2386f26fc10000", handlerTx.Calldata[2].Text(16))
	assert.Equal(t, "0", handlerTx.Calldata[3].Text(16))
	assert.Equal(t, "L1_HANDLER", handlerTx.Type)
}

func TestBlockUnmarshal(t *testing.T) {
	blockJson, err := os.ReadFile("block_11817.json")
	if err != nil {
		t.Error(err)
	}

	var block Block
	err = json.Unmarshal(blockJson, &block)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "24c692acaed3b486990bd9d2b2fbbee802b37b3bd79c59f295bad3277200a83", block.Hash.Text(16))
	assert.Equal(t, "3873ccb937f14429b169c654dda28886d2cc2d6ea17b3cff9748fe5cfdb67e0", block.ParentHash.Text(16))
	assert.Equal(t, uint64(11817), block.Number)
	assert.Equal(t, "3df24be7b5fed6b41de08d38686b6142944119ca2a345c38793590d6804bba4", block.StateRoot.Text(16))
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "27ad16775", block.GasPrice.Text(16))
	assert.Equal(t, 52, len(block.Transactions))
	assert.Equal(t, 52, len(block.Receipts))
	assert.Equal(t, uint64(1669465009), block.Timestamp)
	assert.Equal(t, "0.10.1", block.Version)
}

func TestClassUnmarshal(t *testing.T) {
	classJson, err := os.ReadFile("class_01efa8f8.json")
	if err != nil {
		t.Error(err)
	}

	var class ClassDefinition
	err = json.Unmarshal(classJson, &class)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 1, len(class.EntryPoints.Constructor))
	assert.Equal(t, "a1", class.EntryPoints.Constructor[0].Offset.Text(16))
	assert.Equal(t, "28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", class.EntryPoints.Constructor[0].Selector.Text(16))
	assert.Equal(t, 1, len(class.EntryPoints.L1Handler))
	assert.Equal(t, 1, len(class.EntryPoints.External))
	assert.Equal(t, 250, len(class.Program.Data))
	assert.Equal(t, []string{"pedersen", "range_check"}, class.Program.Builtins)
	assert.Equal(t, "0.10.1", class.Program.CompilerVersion)
}
