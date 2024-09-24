package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionCommitmentPoseidon(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		c, err := transactionCommitmentPoseidon(nil)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, c)
	})
	t.Run("txs with signature", func(t *testing.T) {
		var txs []Transaction

		type signature = []*felt.Felt
		// nil signature, empty signature and signature with some non-empty value
		for i, sign := range []signature{nil, make(signature, 0), {new(felt.Felt).SetUint64(uint64(3))}} {
			invoke := &InvokeTransaction{
				TransactionHash:      new(felt.Felt).SetUint64(uint64(0 + i*3)),
				TransactionSignature: sign,
			}
			deployAccount := &DeployAccountTransaction{
				DeployTransaction: DeployTransaction{
					TransactionHash: new(felt.Felt).SetUint64(uint64(1 + i*3)),
				},
				TransactionSignature: sign,
			}
			declare := &DeclareTransaction{
				TransactionHash:      new(felt.Felt).SetUint64(uint64(2 + i*3)),
				TransactionSignature: sign,
			}
			txs = append(txs, invoke, deployAccount, declare)
		}

		c, err := transactionCommitmentPoseidon(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x602c33ad4fecd30bc8857e87554ae3a1b87dd090f93b4c5ffd5940e98cb712e")
		assert.Equal(t, expected, c, "expected: %s, got: %s", expected, c)
	})
	t.Run("txs without signature", func(t *testing.T) {
		txs := []Transaction{
			&L1HandlerTransaction{
				TransactionHash: utils.HexToFelt(t, "0x1"),
			},
			&DeployTransaction{
				TransactionHash: utils.HexToFelt(t, "0x2"),
			},
		}

		c, err := transactionCommitmentPoseidon(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x6e067f82eefc8efa75b4ad389253757f4992eee0f81f0b43815fa56135ca801")
		assert.Equal(t, expected, c, "expected: %s, got: %s", expected, c)
	})
}
