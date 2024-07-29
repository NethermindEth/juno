package core

import (
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReceiptHash(t *testing.T) {
	t.Run("with gas_consumed", func(t *testing.T) {
		receipt := &TransactionReceipt{
			Fee:                new(felt.Felt).SetUint64(99804),
			ExecutionResources: nil,
			L2ToL1Message: []*L2ToL1Message{
				createL2ToL1Message(34),
				createL2ToL1Message(56),
			},
			TransactionHash: new(felt.Felt).SetUint64(1234),
			Reverted:        true,
			RevertReason:    "aborted",
			TotalGasConsumed: &GasConsumed{
				L1Gas:     16580,
				L1DataGas: 32,
			},
		}

		hash, err := receipt.Hash()
		require.NoError(t, err)

		expected := utils.HexToFelt(t, "0x6276abf21e7c68b2eecfdc8a845b11b44401901f5f040efe10c60d625049646")
		assert.Equal(t, expected, hash)
	})
	t.Run("gas_consumed is nil", func(t *testing.T) {
		receipt := &TransactionReceipt{
			Fee:                new(felt.Felt).SetUint64(99804),
			ExecutionResources: nil,
			L2ToL1Message: []*L2ToL1Message{
				createL2ToL1Message(34),
				createL2ToL1Message(56),
			},
			TransactionHash: new(felt.Felt).SetUint64(1234),
			Reverted:        true,
			RevertReason:    "aborted",
		}

		hash, err := receipt.Hash()
		require.NoError(t, err)

		expected := utils.HexToFelt(t, "0x37268b86bd55df0d9aa93dc3072c6785f06a272c35c30167662b0248f69dbce")
		assert.Equal(t, expected, hash)
	})
}

func TestReceiptCommitment(t *testing.T) {
	receipt := &TransactionReceipt{
		Fee:                new(felt.Felt).SetUint64(99804),
		ExecutionResources: nil,
		L2ToL1Message: []*L2ToL1Message{
			createL2ToL1Message(34),
			createL2ToL1Message(56),
		},
		TransactionHash: new(felt.Felt).SetUint64(1234),
		Reverted:        true,
		RevertReason:    "aborted",
		TotalGasConsumed: &GasConsumed{
			L1Gas:     16580,
			L1DataGas: 32,
		},
	}

	hash, err := receipt.Hash()
	require.NoError(t, err)

	expectedHash := utils.HexToFelt(t, "0x6276abf21e7c68b2eecfdc8a845b11b44401901f5f040efe10c60d625049646")
	require.Equal(t, expectedHash, hash)

	root, err := receiptCommitment([]*TransactionReceipt{receipt})
	require.NoError(t, err)

	expectedRoot := utils.HexToFelt(t, "0x31963cb891ebb825e83514deb748c89b6967b5368cbc48a9b56193a1464ca87")
	assert.Equal(t, expectedRoot, root)
}

func TestMessagesSentHash(t *testing.T) {
	messages := []*L2ToL1Message{
		createL2ToL1Message(0),
		createL2ToL1Message(1),
	}

	hash := messagesSentHash(messages)
	expected := utils.HexToFelt(t, "0x00c89474a9007dc060aed76caf8b30b927cfea1ebce2d134b943b8d7121004e4")
	assert.Equal(t, expected, hash)
}

func createL2ToL1Message(seed uint64) *L2ToL1Message {
	addrBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(addrBytes, seed+1)

	return &L2ToL1Message{
		From: new(felt.Felt).SetUint64(seed),
		To:   common.BytesToAddress(addrBytes),
		Payload: []*felt.Felt{
			new(felt.Felt).SetUint64(seed + 2),
			new(felt.Felt).SetUint64(seed + 3),
		},
	}
}
