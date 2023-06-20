package core_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionEncoding(t *testing.T) {
	tests := []struct {
		name string
		tx   core.Transaction
	}{
		{
			name: "Declare Transaction",
			tx: &core.DeclareTransaction{
				TransactionHash: new(felt.Felt).SetUint64(1),
				ClassHash:       new(felt.Felt).SetUint64(2),
				SenderAddress:   new(felt.Felt).SetUint64(3),
				MaxFee:          new(felt.Felt).SetUint64(4),
				TransactionSignature: []*felt.Felt{
					new(felt.Felt).SetUint64(5),
					new(felt.Felt).SetUint64(6),
				},
				Nonce:   new(felt.Felt).SetUint64(7),
				Version: new(felt.Felt).SetUint64(8),
			},
		},
		{
			name: "Deploy Transaction",
			tx: &core.DeployTransaction{
				TransactionHash:     new(felt.Felt).SetUint64(1),
				ContractAddressSalt: new(felt.Felt).SetUint64(2),
				ContractAddress:     new(felt.Felt).SetUint64(3),
				ClassHash:           new(felt.Felt).SetUint64(4),
				ConstructorCallData: []*felt.Felt{
					new(felt.Felt).SetUint64(5),
					new(felt.Felt).SetUint64(6),
				},
				Version: new(felt.Felt).SetUint64(7),
			},
		},
		{
			name: "Invoke Transaction",
			tx: &core.InvokeTransaction{
				TransactionHash: new(felt.Felt).SetUint64(1),
				CallData: []*felt.Felt{
					new(felt.Felt).SetUint64(2),
					new(felt.Felt).SetUint64(3),
				},
				TransactionSignature: []*felt.Felt{
					new(felt.Felt).SetUint64(4),
					new(felt.Felt).SetUint64(5),
				},
				MaxFee:             new(felt.Felt).SetUint64(6),
				ContractAddress:    new(felt.Felt).SetUint64(7),
				Version:            new(felt.Felt).SetUint64(8),
				EntryPointSelector: new(felt.Felt).SetUint64(9),
				Nonce:              new(felt.Felt).SetUint64(10),
			},
		},
		{
			name: "Deploy Account Transaction",
			tx: &core.DeployAccountTransaction{
				DeployTransaction: core.DeployTransaction{
					TransactionHash:     new(felt.Felt).SetUint64(1),
					ContractAddressSalt: new(felt.Felt).SetUint64(2),
					ContractAddress:     new(felt.Felt).SetUint64(3),
					ClassHash:           new(felt.Felt).SetUint64(4),
					ConstructorCallData: []*felt.Felt{
						new(felt.Felt).SetUint64(5),
						new(felt.Felt).SetUint64(6),
					},
					Version: new(felt.Felt).SetUint64(7),
				},
				MaxFee: new(felt.Felt).SetUint64(8),
				TransactionSignature: []*felt.Felt{
					new(felt.Felt).SetUint64(9),
					new(felt.Felt).SetUint64(10),
				},
				Nonce: new(felt.Felt).SetUint64(11),
			},
		},
		{
			name: "L1 Handler Transaction",
			tx: &core.L1HandlerTransaction{
				TransactionHash:    new(felt.Felt).SetUint64(1),
				ContractAddress:    new(felt.Felt).SetUint64(2),
				EntryPointSelector: new(felt.Felt).SetUint64(3),
				Nonce:              new(felt.Felt).SetUint64(4),
				CallData: []*felt.Felt{
					new(felt.Felt).SetUint64(5),
					new(felt.Felt).SetUint64(6),
				},
				Version: new(felt.Felt).SetUint64(7),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkTransactionSymmetry(t, test.tx)
		})
	}
}

func checkTransactionSymmetry(t *testing.T, input core.Transaction) {
	t.Helper()
	require.NoError(t, encoder.RegisterType(reflect.TypeOf(input)))

	data, err := encoder.Marshal(input)
	require.NoError(t, err)

	var txn core.Transaction
	require.NoError(t, encoder.Unmarshal(data, &txn))

	switch v := txn.(type) {
	case *core.DeclareTransaction:
		assert.Equal(t, input, v)
	case *core.DeployTransaction:
		assert.Equal(t, input, v)
	case *core.InvokeTransaction:
		assert.Equal(t, input, v)
	case *core.DeployAccountTransaction:
		assert.Equal(t, input, v)
	case *core.L1HandlerTransaction:
		assert.Equal(t, input, v)
	default:
		assert.Fail(t, "not a transaction")
	}
}

func TestVerifyTransactionHash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.MAINNET)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)

	txnHash0 := utils.HexToFelt(t, "0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae")
	txn0, err := gw.Transaction(context.Background(), txnHash0)
	require.NoError(t, err)

	txnHash1 := utils.HexToFelt(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee")
	txn1, err := gw.Transaction(context.Background(), txnHash1)
	require.NoError(t, err)

	txnHash2 := utils.HexToFelt(t, "0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e")
	txn2, err := gw.Transaction(context.Background(), txnHash2)
	require.NoError(t, err)

	txnHash3 := utils.HexToFelt(t, "0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190")
	txn3, err := gw.Transaction(context.Background(), txnHash3)
	require.NoError(t, err)

	txnHash4 := utils.HexToFelt(t, "0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497")
	txn4, err := gw.Transaction(context.Background(), txnHash4)
	require.NoError(t, err)

	t.Run("contains bad transaction", func(t *testing.T) {
		badTxn0 := new(core.DeclareTransaction)
		*badTxn0 = *txn0.(*core.DeclareTransaction)
		badTxn0.Version = new(felt.Felt).SetUint64(3)

		badTxn1 := new(core.L1HandlerTransaction)
		*badTxn1 = *txn3.(*core.L1HandlerTransaction)
		badTxn1.TransactionHash = utils.HexToFelt(t, "0xab")

		tests := map[felt.Felt]struct {
			name    string
			wantErr error
			txn     core.Transaction
		}{
			*badTxn0.Hash(): {
				name:    "Declare - error if transaction hash calculation failed",
				wantErr: fmt.Errorf("cannot calculate transaction hash of Transaction %v, reason: invalid Transaction (type: *core.DeclareTransaction) version: 3", badTxn0.Hash().String()),
				txn:     badTxn0,
			},
			*badTxn1.Hash(): {
				name:    "Deploy - error if transaction hashes don't match",
				wantErr: fmt.Errorf("cannot verify transaction hash of Transaction %v", badTxn1.Hash().String()),
				txn:     badTxn1,
			},
			*txn2.Hash(): {
				name:    "DeployAccount - no error if transaction hashes match",
				wantErr: nil,
				txn:     txn2,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				tErr := core.VerifyTransactions([]core.Transaction{test.txn}, utils.MAINNET, "99.99.99")
				require.Equal(t, test.wantErr, tErr)
			})
		}
	})

	t.Run("does not contain bad transaction(s)", func(t *testing.T) {
		txns := []core.Transaction{txn0, txn1, txn2, txn3, txn4}
		assert.NoError(t, core.VerifyTransactions(txns, utils.MAINNET, "99.99.99"))
	})
}
func TestGenerateTransactionHash(t *testing.T) {
	pub := utils.HexToFelt(t, "0xc24ee1f993471a03a72d4d5e9f21e91296db50811e2298cc36224552afd5c1")

	tx := core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			ClassHash:           utils.HexToFelt(t, "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918"),
			ContractAddress:     utils.HexToFelt(t, "0x63242861a734490bf31412bcb84a6ad37e370c99a5697de6dd3e8c2ebd40539"),
			ContractAddressSalt: pub,
			ConstructorCallData: []*felt.Felt{
				utils.HexToFelt(t, "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2"),
				utils.HexToFelt(t, "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463"),
				utils.HexToFelt(t, "0x2"),
				pub,
				utils.HexToFelt(t, "0x0"),
			},
			Version: new(felt.Felt).SetUint64(1),
		},
		MaxFee: new(felt.Felt).SetUint64(10000000000),
		Nonce:  new(felt.Felt).SetUint64(0),
	}

	txhash, err := core.TransactionHash(&tx, utils.Network(utils.GOERLI))
	require.NoError(t, err)
	require.Equal(t, txhash.String(), "0x794ddc51a8298b57064667cd8fb9ef79d7410c71d8f8ad8098b4462520f582e")
}
