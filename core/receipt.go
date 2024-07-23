package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

type GasConsumed struct {
	L1Gas     uint64
	L1DataGas uint64
}

type TransactionReceipt struct {
	Fee                *felt.Felt
	FeeUnit            FeeUnit
	Events             []*Event
	ExecutionResources *ExecutionResources
	L1ToL2Message      *L1ToL2Message
	L2ToL1Message      []*L2ToL1Message
	TransactionHash    *felt.Felt
	Reverted           bool
	RevertReason       string
	TotalGasConsumed   *GasConsumed
}

func (r *TransactionReceipt) Hash() (*felt.Felt, error) {
	revertReasonHash := &felt.Zero
	if r.Reverted {
		// todo remove error because it's unneccessary
		var err error
		revertReasonHash, err = crypto.StarknetKeccak([]byte(r.RevertReason))
		if err != nil {
			return nil, err
		}
	}

	var totalGasConsumed GasConsumed
	// pre 0.13.2 this property is not set, in this case we use zero values in totalGasConsumed
	if r.TotalGasConsumed != nil {
		totalGasConsumed = *r.TotalGasConsumed
	}

	return crypto.PoseidonArray(
		r.TransactionHash,
		r.Fee,
		messagesSentHash(r.L2ToL1Message),
		revertReasonHash,
		&felt.Zero, // L2 gas consumed
		new(felt.Felt).SetUint64(totalGasConsumed.L1Gas),
		new(felt.Felt).SetUint64(totalGasConsumed.L1DataGas),
	), nil
}

func messagesSentHash(messages []*L2ToL1Message) *felt.Felt {
	chain := []*felt.Felt{
		new(felt.Felt).SetUint64(uint64(len(messages))),
	}
	for _, msg := range messages {
		msgTo := new(felt.Felt).SetBytes(msg.To.Bytes())
		payloadSize := new(felt.Felt).SetUint64(uint64(len(msg.Payload)))
		chain = append(chain, msg.From, msgTo, payloadSize)
		chain = append(chain, msg.Payload...)
	}

	return crypto.PoseidonArray(chain...)
}

func receiptCommitment(receipts []*TransactionReceipt) (*felt.Felt, error) {
	var commitment *felt.Felt
	return commitment, trie.RunOnTempTrie(commitmentTrieHeight, func(trie *trie.Trie) error {
		for i, receipt := range receipts {
			hash, err := receipt.Hash()
			if err != nil {
				return err
			}

			receiptTrieKey := new(felt.Felt).SetUint64(uint64(i))
			_, err = trie.Put(receiptTrieKey, hash)
			if err != nil {
				return err
			}
		}

		root, err := trie.Root()
		if err != nil {
			return err
		}
		commitment = root

		return nil
	})
}
