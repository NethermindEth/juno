package core_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

var (
	//go:embed testdata/bytecode_v0_declare.json
	bytecodeV0DeclareTransBytes []byte
	//go:embed testdata/bytecode_v1_declare.json
	bytecodeV1DeclareTransBytes []byte
)

func TestDeployTransactions(t *testing.T) {
	tests := map[string]struct {
		input   core.DeployTransaction
		network utils.Network
		want    *felt.Felt
	}{
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0
		"Deploy transaction": {
			input: core.DeployTransaction{
				ContractAddress:     hexToFelt("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
				ContractAddressSalt: hexToFelt("0x74dc2fe193daf1abd8241b63329c1123214842b96ad7fd003d25512598a956b"),
				ClassHash:           hexToFelt("0x46f844ea1a3b3668f81d38b5c1bd55e816e0373802aefe732138628f0133486"),
				ConstructorCallData: [](*felt.Felt){
					hexToFelt("0x6d706cfbac9b8262d601c38251c5fbe0497c3a96cc91a92b08d91b61d9e70c4"),
					hexToFelt("0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
					hexToFelt("0x2"),
					hexToFelt("0x6658165b4984816ab189568637bedec5aa0a18305909c7f5726e4a16e3afef6"),
					hexToFelt("0x6b648b36b074a91eee55730f5f5e075ec19c0a8f9ffb0903cefeee93b6ff328"),
				},
				CallerAddress: new(felt.Felt).SetUint64(0),
				Version:       new(felt.Felt).SetUint64(0),
			},
			network: utils.MAINNET,
			want:    hexToFelt("0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash(test.network)
			if err != nil {
				t.Errorf("no error expected but got %v", err)
			}
			if !transactionHash.Equal(test.want) {
				t.Errorf("wrong hash: got %s, want %s", transactionHash.Text(16), test.want.Text(16))
			}

			checkTransactionSymmetry(t, &test.input)
		})
	}
}

func TestInvokeTransactions(t *testing.T) {
	tests := map[string]struct {
		input   core.InvokeTransaction
		network utils.Network
		want    *felt.Felt
	}{
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e
		"Invoke transaction version 0": {
			input: core.InvokeTransaction{
				ContractAddress:    hexToFelt("0x43324c97e376d7d164abded1af1e73e9ce8214249f711edb7059c1ca34560e8"),
				EntryPointSelector: hexToFelt("0x317eb442b72a9fae758d4fb26830ed0d9f31c8e7da4dbff4e8c59ea6a158e7f"),
				CallData: [](*felt.Felt){
					hexToFelt("0x1b654cb59f978da2eee76635158e5ff1399bf607cb2d05e3e3b4e41d7660ca2"),
					hexToFelt("0x2"),
					hexToFelt("0x5f743efdb29609bfc2002041bdd5c72257c0c6b5c268fc929a3e516c171c731"),
					hexToFelt("0x635afb0ea6c4cdddf93f42287b45b67acee4f08c6f6c53589e004e118491546"),
				},
				MaxFee:  hexToFelt("0x0"),
				Version: new(felt.Felt).SetUint64(0),
			},
			network: utils.MAINNET,
			want:    hexToFelt("0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e"),
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497
		"Invoke transaction version 1": {
			input: core.InvokeTransaction{
				ContractAddress: hexToFelt("0x3ec215c6c9028ff671b46a2a9814970ea23ed3c4bcc3838c6d1dcbf395263c3"),
				CallData: [](*felt.Felt){
					hexToFelt("0x1"),
					hexToFelt("0x727a63f78ee3f1bd18f78009067411ab369c31dece1ae22e16f567906409905"),
					hexToFelt("0x22de356837ac200bca613c78bd1fcc962a97770c06625f0c8b3edeb6ae4aa59"),
					hexToFelt("0x0"),
					hexToFelt("0xb"),
					hexToFelt("0xb"),
					hexToFelt("0xa"),
					hexToFelt("0x6db793d93ce48bc75a5ab02e6a82aad67f01ce52b7b903090725dbc4000eaa2"),
					hexToFelt("0x6141eac4031dfb422080ed567fe008fb337b9be2561f479a377aa1de1d1b676"),
					hexToFelt("0x27eb1a21fa7593dd12e988c9dd32917a0dea7d77db7e89a809464c09cf951c0"),
					hexToFelt("0x400a29400a34d8f69425e1f4335e6a6c24ce1111db3954e4befe4f90ca18eb7"),
					hexToFelt("0x599e56821170a12cdcf88fb8714057ce364a8728f738853da61d5b3af08a390"),
					hexToFelt("0x46ad66f467df625f3b2dd9d3272e61713e8f74b68adac6718f7497d742cfb17"),
					hexToFelt("0x4f348b585e6c1919d524a4bfe6f97230ecb61736fe57534ec42b628f7020849"),
					hexToFelt("0x19ae40a095ffe79b0c9fc03df2de0d2ab20f59a2692ed98a8c1062dbf691572"),
					hexToFelt("0xe120336994adef6c6e47694f87278686511d4622997d4a6f216bd6e9fa9acc"),
					hexToFelt("0x56e6637a4958d062db8c8198e315772819f64d915e5c7a8d58a99fa90ff0742"),
				},
				Signature: [](*felt.Felt){
					hexToFelt("0x383ba105b6d0f59fab96a412ad267213ddcd899e046278bdba64cd583d680b"),
					hexToFelt("0x1896619a17fde468978b8d885ffd6f5c8f4ac1b188233b81b91bcf7dbc56fbd"),
				},
				Nonce:         hexToFelt("0x42"),
				SenderAddress: hexToFelt("0x1fc039de7d864580b57a575e8e6b7114f4d2a954d7d29f876b2eb3dd09394a0"),
				MaxFee:        hexToFelt("0x17f0de82f4be6"),
				Version:       new(felt.Felt).SetUint64(1),
			},
			network: utils.MAINNET,
			want:    hexToFelt("0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash(test.network)
			if err != nil {
				t.Errorf("no error expected but got %v", err)
			}
			if !transactionHash.Equal(test.want) {
				t.Errorf("wrong hash: got %s, want %s", transactionHash.Text(16), test.want.Text(16))
			}

			checkTransactionSymmetry(t, &test.input)
		})
	}
}

func TestDeclareTransaction(t *testing.T) {
	var bytecodeV0Declare []*felt.Felt
	if err := json.Unmarshal(bytecodeV0DeclareTransBytes, &bytecodeV0Declare); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	var bytecodeV1Declare []*felt.Felt
	if err := json.Unmarshal(bytecodeV1DeclareTransBytes, &bytecodeV1Declare); err != nil {
		t.Fatalf("unexpected error while unmarshalling bytecodeBytes: %s", err)
	}

	tests := map[string]struct {
		input   core.DeclareTransaction
		network utils.Network
		want    *felt.Felt
	}{
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4
		"Declare transaction version 0": {
			input: core.DeclareTransaction{
				// https://alpha-mainnet.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0
				ClassHash:     hexToFelt("0x2760f25d5a4fb2bdde5f561fd0b44a3dee78c28903577d37d669939d97036a0"),
				Nonce:         hexToFelt("0x0"),
				SenderAddress: hexToFelt("0x1"),
				MaxFee:        hexToFelt("0x0"),
				Version:       new(felt.Felt).SetUint64(0),
			},
			network: utils.MAINNET,
			want:    hexToFelt("0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4"),
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae
		"Declare transaction version 1": {
			input: core.DeclareTransaction{
				ClassHash: hexToFelt("0x7aed6898458c4ed1d720d43e342381b25668ec7c3e8837f761051bf4d655e54"),
				Signature: [](*felt.Felt){
					hexToFelt("0x221b9576c4f7b46d900a331d89146dbb95a7b03d2eb86b4cdcf11331e4df7f2"),
					hexToFelt("0x667d8062f3574ba9b4965871eec1444f80dacfa7114e1d9c74662f5672c0620"),
				},
				Nonce:         hexToFelt("0x5"),
				SenderAddress: hexToFelt("0x39291faa79897de1fd6fb1a531d144daa1590d058358171b83eadb3ceafed8"),
				MaxFee:        hexToFelt("0xf6dbd653833"),
				Version:       new(felt.Felt).SetUint64(1),
			},
			network: utils.MAINNET,
			want:    hexToFelt("0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			transactionHash, err := test.input.Hash(test.network)
			if err != nil {
				t.Errorf("no error expected but got %v", err)
			}
			if !transactionHash.Equal(test.want) {
				t.Errorf("wrong hash: got %s, want %s", transactionHash.Text(16), test.want.Text(16))
			}

			checkTransactionSymmetry(t, &test.input)
		})
	}
}

func checkTransactionSymmetry(t *testing.T, input core.Transaction) {
	data, err := encoder.Marshal(input)
	assert.NoError(t, err)

	var txn core.Transaction
	assert.NoError(t, encoder.Unmarshal(data, &txn))

	switch v := txn.(type) {
	case *core.DeclareTransaction:
		assert.Equal(t, input, v)
	case *core.DeployTransaction:
		assert.Equal(t, input, v)
	case *core.InvokeTransaction:
		assert.Equal(t, input, v)
	default:
		t.Error("not a transaction")
	}
}
