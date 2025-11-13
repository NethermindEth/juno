package core_test

import (
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBlockVersion_version13_1_1(t *testing.T) {
	ver, err := core.ParseBlockVersion("0.13.1.1")
	require.NoError(t, err)

	customSemverGreater := ver.GreaterThan(semver.MustParse("0.13.1"))
	assert.False(t, customSemverGreater)
}

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
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	txnHash0 := felt.NewUnsafeFromString[felt.Felt]("0x1b4d9f09276629d496af1af8ff00173c11ff146affacb1b5c858d7aa89001ae")
	txn0, err := gw.Transaction(t.Context(), txnHash0)
	require.NoError(t, err)

	txnHash1 := felt.NewUnsafeFromString[felt.Felt]("0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee")
	txn1, err := gw.Transaction(t.Context(), txnHash1)
	require.NoError(t, err)

	txnHash2 := felt.NewUnsafeFromString[felt.Felt]("0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e")
	txn2, err := gw.Transaction(t.Context(), txnHash2)
	require.NoError(t, err)

	txnHash3 := felt.NewUnsafeFromString[felt.Felt]("0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190")
	txn3, err := gw.Transaction(t.Context(), txnHash3)
	require.NoError(t, err)

	txnHash4 := felt.NewUnsafeFromString[felt.Felt]("0x2897e3cec3e24e4d341df26b8cf1ab84ea1c01a051021836b36c6639145b497")
	txn4, err := gw.Transaction(t.Context(), txnHash4)
	require.NoError(t, err)

	t.Run("contains bad transaction", func(t *testing.T) {
		badTxn0 := new(core.DeclareTransaction)
		*badTxn0 = *txn0.(*core.DeclareTransaction)
		badTxn0.Version = new(core.TransactionVersion).SetUint64(18)

		badTxn1 := new(core.L1HandlerTransaction)
		*badTxn1 = *txn3.(*core.L1HandlerTransaction)
		badTxn1.TransactionHash = felt.NewUnsafeFromString[felt.Felt]("0xab")

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
				tErr := core.VerifyTransactions([]core.Transaction{test.txn}, &utils.Mainnet, "99.99.99")
				require.Equal(t, test.wantErr, tErr)
			})
		}
	})

	t.Run("does not contain bad transaction(s)", func(t *testing.T) {
		txns := []core.Transaction{txn0, txn1, txn2, txn3, txn4}
		assert.NoError(t, core.VerifyTransactions(txns, &utils.Mainnet, "99.99.99"))
	})
}

func TestTransactionV3Hash(t *testing.T) {
	network := utils.Sepolia
	gw := adaptfeeder.New(feeder.NewTestClient(t, &network))
	ctx := t.Context()

	tests := map[string]struct {
		tx   func(hash *felt.Felt) core.Transaction
		want felt.Felt
	}{
		"invoke": {
			// https://alpha-sepolia.starknet.io/feeder_gateway/get_transaction?transactionHash=0x76b52e17bc09064bd986ead34263e6305ef3cecfb3ae9e19b86bf4f1a1a20ea
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				invoke, ok := tx.(*core.InvokeTransaction)
				require.True(t, ok)
				invoke.TransactionHash = nil
				return invoke
			},
			want: felt.UnsafeFromString[felt.Felt](
				"0x76b52e17bc09064bd986ead34263e6305ef3cecfb3ae9e19b86bf4f1a1a20ea",
			),
		},
		// https://alpha-sepolia.starknet.io/feeder_gateway/get_transaction?transactionHash=0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155
		"declare": {
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				declare, ok := tx.(*core.DeclareTransaction)
				require.True(t, ok)
				declare.TransactionHash = nil
				return declare
			},
			want: felt.UnsafeFromString[felt.Felt](
				"0x30c852c522274765e1d681bc8a84ce7c41118370ef2ba7d18a427ed29f5b155",
			),
		},
		// https://alpha-sepolia.starknet.io/feeder_gateway/get_transaction?transactionHash=0x32413f8cee053089d6d7026a72e4108262ca3cfe868dd9159bc1dd160aec975
		"deployAccount": {
			tx: func(hash *felt.Felt) core.Transaction {
				tx, err := gw.Transaction(ctx, hash)
				require.NoError(t, err)
				deployAccount, ok := tx.(*core.DeployAccountTransaction)
				require.True(t, ok)
				deployAccount.TransactionHash = nil
				return deployAccount
			},
			want: felt.UnsafeFromString[felt.Felt](
				"0x32413f8cee053089d6d7026a72e4108262ca3cfe868dd9159bc1dd160aec975",
			),
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			got, err := core.TransactionHash(test.tx(&test.want), &network)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestTransactionVersion(t *testing.T) {
	f := felt.NewUnsafeFromString[felt.Felt]("0x100000000000000000000000000000002")
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
				ContractAddress:    felt.NewUnsafeFromString[felt.Felt]("0x073314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82"),
				EntryPointSelector: felt.NewUnsafeFromString[felt.Felt]("0x02d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5"),
				Nonce:              felt.NewUnsafeFromString[felt.Felt]("0xbf0dd"),
				CallData: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0xc3511006c04ef1d78af4c8e0e74ec18a6e64ff9e"),
					felt.NewUnsafeFromString[felt.Felt]("0x3efc988748484820f1c157fb48e218d39cadc07a662482d3875d37445b3c082"),
					felt.NewUnsafeFromString[felt.Felt]("0x11c37937e08000"),
					felt.NewUnsafeFromString[felt.Felt]("0x0"),
				},
			},
		},
		{
			expected: "546479edde51ea965cfa77ecde8d749d198e54e4ec71e4b543866dbe837d8a26",
			tx: &core.L1HandlerTransaction{
				ContractAddress:    felt.NewUnsafeFromString[felt.Felt]("0x078466c2444176f0be70650b3d1f520e19a095ca5fa6ff124ddc49f27a30bdac"),
				EntryPointSelector: felt.NewUnsafeFromString[felt.Felt]("0xe3f5e9e1456ffa52a3fbc7e8c296631d4cc2120c0be1e2829301c0d8fa026b"),
				CallData: []*felt.Felt{
					felt.NewUnsafeFromString[felt.Felt]("0xeaea1710a78bd93bf022fda3e95100dc12973b1b"),
					felt.NewUnsafeFromString[felt.Felt]("0x8c1e1e5b47980d214965f3bd8ea34c413e120ae4"),
					felt.NewUnsafeFromString[felt.Felt]("0x1"),
				},
			},
		},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, hex.EncodeToString(test.tx.MessageHash()))
	}
}

func TestDeclareV0TransactionHash(t *testing.T) {
	network := utils.Goerli
	gw := adaptfeeder.New(feeder.NewTestClient(t, &network))
	ctx := t.Context()

	b, err := gw.BlockByNumber(ctx, 231579)
	require.NoError(t, err)

	decTx, ok := b.Transactions[30].(*core.DeclareTransaction)
	require.True(t, ok)

	expectedHash := decTx.Hash()

	decTx.TransactionHash = nil

	got, err := core.TransactionHash(decTx, &network)
	require.NoError(t, err)
	assert.Equal(t, *expectedHash, got)
}
