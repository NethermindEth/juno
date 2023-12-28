package core_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/feeder"
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
				Version: new(core.TransactionVersion).SetUint64(8),
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
				Version: new(core.TransactionVersion).SetUint64(7),
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
				Version:            new(core.TransactionVersion).SetUint64(8),
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
					Version: new(core.TransactionVersion).SetUint64(7),
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
				Version: new(core.TransactionVersion).SetUint64(7),
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
	client := feeder.NewTestClient(t, utils.Mainnet)
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
		badTxn0.Version = new(core.TransactionVersion).SetUint64(18)

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
				wantErr: fmt.Errorf("cannot calculate transaction hash of Transaction %v, reason: %w", badTxn0.Hash().String(), errors.New("invalid Transaction (type: *core.DeclareTransaction) version: 0x12")),
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
				tErr := core.VerifyTransactions([]core.Transaction{test.txn}, utils.Mainnet, "99.99.99")
				require.Equal(t, test.wantErr, tErr)
			})
		}
	})

	t.Run("does not contain bad transaction(s)", func(t *testing.T) {
		txns := []core.Transaction{txn0, txn1, txn2, txn3, txn4}
		assert.NoError(t, core.VerifyTransactions(txns, utils.Mainnet, "99.99.99"))
	})
}

func TestTransactionV3Hash(t *testing.T) {
	network := utils.Integration
	gw := adaptfeeder.New(feeder.NewTestClient(t, network))
	ctx := context.Background()

	// Test data was obtained by playing with Papyrus's implementation before v0.13 was released on integration.
	tests := map[string]struct {
		tx   func(hash *felt.Felt) core.Transaction
		want *felt.Felt
	}{
		"invoke": {
			// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				invoke, ok := tx.(*core.InvokeTransaction)
				require.True(t, ok)
				invoke.TransactionHash = nil
				return invoke
			},
			want: utils.HexToFelt(t, "0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd"),
		},
		// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3
		"declare": {
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				declare, ok := tx.(*core.DeclareTransaction)
				require.True(t, ok)
				declare.TransactionHash = nil
				return declare
			},
			want: utils.HexToFelt(t, "0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3"),
		},
		// https://external.integration.starknet.io/feeder_gateway/get_transaction?transactionHash=0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0
		"deployAccount": {
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				deployAccount, ok := tx.(*core.DeployAccountTransaction)
				require.True(t, ok)
				deployAccount.TransactionHash = nil
				return deployAccount
			},
			want: utils.HexToFelt(t, "0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0"),
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			got, err := core.TransactionHash(test.tx(test.want), network)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestTransactionVersi(t *testing.T) {
	f := utils.HexToFelt(t, "0x100000000000000000000000000000002")
	v := (*core.TransactionVersion)(f)

	assert.True(t, v.HasQueryBit())
	assert.True(t, v.Is(2))
	assert.False(t, v.Is(1))
	assert.False(t, v.Is(0))

	withoutQBit := v.WithoutQueryBit()
	assert.False(t, withoutQBit.HasQueryBit())
	assert.True(t, withoutQBit.Is(2))
	assert.False(t, withoutQBit.Is(1))
	assert.False(t, withoutQBit.Is(0))
}

func TestMessageHash(t *testing.T) {
	tests := []struct {
		expected string
		tx       *core.L1HandlerTransaction
	}{
		{
			expected: "f3507cad1b674c2b2f26a0a51cc8abebe96ad7a8a9cd1aa54b00fddee776e4cf",
			tx: &core.L1HandlerTransaction{
				ContractAddress:    utils.HexToFelt(t, "0x073314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82"),
				EntryPointSelector: utils.HexToFelt(t, "0x02d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5"),
				Nonce:              utils.HexToFelt(t, "0xbf0dd"),
				CallData: []*felt.Felt{
					utils.HexToFelt(t, "0xc3511006c04ef1d78af4c8e0e74ec18a6e64ff9e"),
					utils.HexToFelt(t, "0x3efc988748484820f1c157fb48e218d39cadc07a662482d3875d37445b3c082"),
					utils.HexToFelt(t, "0x11c37937e08000"),
					utils.HexToFelt(t, "0x0"),
				},
			},
		},
		{
			expected: "546479edde51ea965cfa77ecde8d749d198e54e4ec71e4b543866dbe837d8a26",
			tx: &core.L1HandlerTransaction{
				ContractAddress:    utils.HexToFelt(t, "0x078466c2444176f0be70650b3d1f520e19a095ca5fa6ff124ddc49f27a30bdac"),
				EntryPointSelector: utils.HexToFelt(t, "0xe3f5e9e1456ffa52a3fbc7e8c296631d4cc2120c0be1e2829301c0d8fa026b"),
				CallData: []*felt.Felt{
					utils.HexToFelt(t, "0xeaea1710a78bd93bf022fda3e95100dc12973b1b"),
					utils.HexToFelt(t, "0x8c1e1e5b47980d214965f3bd8ea34c413e120ae4"),
					utils.HexToFelt(t, "0x1"),
				},
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, hex.EncodeToString(test.tx.MessageHash()))
	}
}
