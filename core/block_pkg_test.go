package core

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:dupl
func TestTransactionCommitmentPoseidon0134(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		c, err := transactionCommitmentPoseidon0134(nil)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, c)
	})
	t.Run("txs with signature", func(t *testing.T) {
		var txs []Transaction

		type signature = []*felt.Felt
		// actual tx hash is irrelevant so it's ok to have different transactions with the same hash
		txHash := utils.HexToFelt(t, "0xCAFEBABE")
		// nil signature, empty signature and signature with some non-empty value
		for _, sign := range []signature{nil, make(signature, 0), {new(felt.Felt).SetUint64(uint64(3))}} {
			invoke := &InvokeTransaction{
				TransactionHash:      txHash,
				TransactionSignature: sign,
			}
			deployAccount := &DeployAccountTransaction{
				DeployTransaction: DeployTransaction{
					TransactionHash: txHash,
				},
				TransactionSignature: sign,
			}
			declare := &DeclareTransaction{
				TransactionHash:      txHash,
				TransactionSignature: sign,
			}
			txs = append(txs, invoke, deployAccount, declare)
		}

		c, err := transactionCommitmentPoseidon0134(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x4ca6d4ceb367bf070d896a1479190d3c7b751f525e69a46ee2c83f0afe7cb8")
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

		c, err := transactionCommitmentPoseidon0134(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x5ecb75d7a86984ec8ef9d5fbbe49ef8737c37246d33cf73037df1ceb412244e")
		assert.Equal(t, expected, c, "expected: %s, got: %s", expected, c)
	})
}

func TestTransactionCommitmentPoseidon0132(t *testing.T) { //nolint:dupl
	t.Run("nil", func(t *testing.T) {
		c, err := transactionCommitmentPoseidon0132(nil)
		require.NoError(t, err)
		assert.Equal(t, &felt.Zero, c)
	})
	t.Run("txs with signature", func(t *testing.T) {
		var txs []Transaction

		type signature = []*felt.Felt
		// actual tx hash is irrelevant so it's ok to have different transactions with the same hash
		txHash := utils.HexToFelt(t, "0xCAFEBABE")
		// nil signature, empty signature and signature with some non-empty value
		for _, sign := range []signature{nil, make(signature, 0), {new(felt.Felt).SetUint64(uint64(3))}} {
			invoke := &InvokeTransaction{
				TransactionHash:      txHash,
				TransactionSignature: sign,
			}
			deployAccount := &DeployAccountTransaction{
				DeployTransaction: DeployTransaction{
					TransactionHash: txHash,
				},
				TransactionSignature: sign,
			}
			declare := &DeclareTransaction{
				TransactionHash:      txHash,
				TransactionSignature: sign,
			}
			txs = append(txs, invoke, deployAccount, declare)
		}

		c, err := transactionCommitmentPoseidon0132(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x68303856fce63d62acb85da0766b370c03754aa316b0b5bce05982f9561b73d")
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

		c, err := transactionCommitmentPoseidon0132(txs)
		require.NoError(t, err)
		expected := utils.HexToFelt(t, "0x6e067f82eefc8efa75b4ad389253757f4992eee0f81f0b43815fa56135ca801")
		assert.Equal(t, expected, c, "expected: %s, got: %s", expected, c)
	})
}
