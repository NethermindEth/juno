package core_test

import (
	_ "embed"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestTransactionHash(t *testing.T) {
	tests := map[string]struct {
		txnHash string
		network utils.Network
	}{
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0
		"Deploy transaction": {
			txnHash: "0x6486c6303dba2f364c684a2e9609211c5b8e417e767f37b527cda51e776e6f0",
			network: utils.MAINNET,
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e
		"Invoke transaction version 0": {
			txnHash: "0xf1d99fb97509e0dfc425ddc2a8c5398b74231658ca58b6f8da92f39cb739e",
			network: utils.MAINNET,
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497
		"Invoke transaction version 1": {
			txnHash: "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497",
			network: utils.MAINNET,
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4
		"Declare transaction version 0": {
			txnHash: "0x222f8902d1eeea76fa2642a90e2411bfd71cffb299b3a299029e1937fab3fe4",
			network: utils.MAINNET,
		},
		// https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae
		"Declare transaction version 1": {
			txnHash: "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae",
			network: utils.MAINNET,
		},
	}

	gw, closer := testsource.NewTestGateway(utils.MAINNET)
	defer closer.Close()

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			hash := hexToFelt(test.txnHash)
			txn, err := gw.Transaction(hash)
			assert.NoError(t, err)
			assert.NotNil(t, txn)

			transactionHash, err := txn.Hash(test.network)
			if err != nil {
				t.Errorf("no error expected but got %v", err)
			}
			if !transactionHash.Equal(hash) {
				t.Errorf("wrong hash: got %s, want %s", transactionHash.Text(16), hash.Text(16))
			}

			checkTransactionSymmetry(t, txn)
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
