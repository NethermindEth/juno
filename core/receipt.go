package core

import (
	"fmt"

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
}

func (tr *TransactionReceipt) Print() {
	fmt.Println("Transaction Receipt:")
	fmt.Println("  Fee:", tr.Fee.String())
	fmt.Println("  Fee Unit:", tr.FeeUnit)
	fmt.Println("  Transaction Hash:", tr.TransactionHash.String())
	fmt.Println("  Reverted:", tr.Reverted)
	if tr.Reverted {
		fmt.Println("  Revert Reason:", tr.RevertReason)
	}

	fmt.Println("  Events:")
	for i, event := range tr.Events {
		fmt.Printf("    Event %d:\n", i+1)
		fmt.Println("      From:", event.From.String())
		fmt.Println("      Data:")
		for j, data := range event.Data {
			fmt.Printf("        Data %d: %s\n", j+1, data.String())
		}
		fmt.Println("      Keys:")
		for j, key := range event.Keys {
			fmt.Printf("        Key %d: %s\n", j+1, key.String())
		}
	}

	if tr.ExecutionResources != nil {
		fmt.Println("  Execution Resources:")
		tr.ExecutionResources.Print()
	}

	fmt.Println("  L1 To L2 Message: TODO")

	fmt.Println("  L2 To L1 Messages: TODO")
}
func (r *TransactionReceipt) hash() *felt.Felt {
	revertReasonHash := &felt.Zero
	if r.Reverted {
		revertReasonHash = crypto.StarknetKeccak([]byte(r.RevertReason))
	}

	var totalGasConsumed GasConsumed
	// pre 0.13.2 TotalGasConsumed property is not set, in this case we rely on zero value above
	if r.ExecutionResources != nil && r.ExecutionResources.TotalGasConsumed != nil {
		totalGasConsumed = *r.ExecutionResources.TotalGasConsumed
	}

	return crypto.PoseidonArray(
		r.TransactionHash,
		r.Fee,
		messagesSentHash(r.L2ToL1Message),
		revertReasonHash,
		&felt.Zero, // L2 gas consumed
		new(felt.Felt).SetUint64(totalGasConsumed.L1Gas),
		new(felt.Felt).SetUint64(totalGasConsumed.L1DataGas),
	)
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

	return commitment, trie.RunOnTempTriePoseidon(commitmentTrieHeight, func(trie *trie.Trie) error {
		for i, receipt := range receipts {
			receiptTrieKey := new(felt.Felt).SetUint64(uint64(i))
			_, err := trie.Put(receiptTrieKey, receipt.hash())
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
