package core_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
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
		t.Error("not a transaction")
	}
}
