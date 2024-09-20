package core

import (
	"fmt"
	"strings"

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

func CompareReceipts(seq, ref *TransactionReceipt) (bool, string) {
	var result strings.Builder
	foundDiff := false

	if seq.TransactionHash.Equal(ref.TransactionHash) {
		result.WriteString("TransactionHash: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"TransactionHash: DIFFERENT\n\t- seq: %v\n\t- ref: %v\n",
			seq.TransactionHash.String(),
			ref.TransactionHash.String()))
	}

	if seq.Fee.Equal(ref.Fee) {
		result.WriteString("Fee: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Fee: DIFFERENT\n\t- seq: %v\n\t- ref: %v\n",
			seq.Fee.String(),
			ref.Fee.String()))
	}

	if MessagesSentHash(seq.L2ToL1Message).Equal(MessagesSentHash(ref.L2ToL1Message)) {
		result.WriteString("MessagesSentHash: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"MessagesSentHash: DIFFERENT\n\t- seq: %v\n\t- ref: %v\n",
			MessagesSentHash(seq.L2ToL1Message).String(),
			MessagesSentHash(ref.L2ToL1Message).String()))
	}

	if seq.Reverted == ref.Reverted {
		result.WriteString("Reverted: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Reverted: DIFFERENT\n\t- seq: %v\n\t- ref: %v\n",
			seq.Reverted,
			ref.Reverted))
	}

	if seq.RevertReason == ref.RevertReason {
		result.WriteString("RevertReason: EQUAL\n")
	} else {
		result.WriteString(fmt.Sprintf(
			"RevertReason: DIFFERENT\n\t- seq: %s\n\t- ref: %s\n",
			seq.RevertReason,
			ref.RevertReason))
	}

	seqGas := seq.ExecutionResources.TotalGasConsumed
	refGas := ref.ExecutionResources.TotalGasConsumed
	if refGas != nil {
		if seqGas == nil {
			foundDiff = true
			result.WriteString(fmt.Sprintf(
				"TotalGasConsumed: DIFFERENT\n\t- g1: nil\n\t- g2: %d\n", refGas))
		} else {
			if seqGas.L1Gas == refGas.L1Gas {
				result.WriteString("TotalGasConsumed.L1Gas: EQUAL\n")
			} else {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"TotalGasConsumed.L1Gas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n",
					seqGas.L1Gas,
					refGas.L1Gas))
			}

			if seqGas.L1DataGas == refGas.L1DataGas {
				result.WriteString("TotalGasConsumed.L1DataGas: EQUAL\n")
			} else {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"TotalGasConsumed.L1DataGas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n",
					seqGas.L1DataGas,
					refGas.L1DataGas))
			}
		}
	}

	if len(seq.Events) != len(ref.Events) {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Events have DIFFERENT length \n\t- g1: %d\n\t- g2: %d\n",
			len(seq.Events),
			len(seq.Events)))
	} else {
		for ind, event := range seq.Events {
			if !event.From.Equal(ref.Events[ind].From) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT From value at index %d \n\t- g1: %v\n\t- g2: %v\n",
					ind, event.From,
					ref.Events[ind].From))
				printEvents(seq.Events, ref.Events)
				break
			}
			if !equalSlices(seq.Events[ind].Keys, ref.Events[ind].Keys) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT Keys \n\t- g1: %v\n\t- g2: %v\n",
					event.Keys,
					ref.Events[ind].Keys))
				printEvents(seq.Events, ref.Events)
				break
			}
			if !equalSlices(seq.Events[ind].Data, ref.Events[ind].Data) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT Data \n\t- g1: %v\n\t- g2: %v\n",
					event.Keys,
					ref.Events[ind].Keys))
				printEvents(seq.Events, ref.Events)
				break
			}
		}
	}
	return foundDiff, result.String()
}

func printEvents(events1, events2 []*Event) {
	fmt.Println("EVENTS 1 - SEQUENER")
	for _, event := range events1 {
		fmt.Printf("\nevent, from %v, key %v, data %v\n", event.From.String(), event.Keys, event.Data)
	}
	fmt.Println("EVENTS 1 - SEPOLIA")
	for _, event := range events2 {
		fmt.Printf("\nevent, from %v, key %v, data %v\n", event.From.String(), event.Keys, event.Data)
	}
}

func equalSlices(a, b []*felt.Felt) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

func (r *TransactionReceipt) Print() {
	fmt.Println("Transaction Receipt:")
	fmt.Println("  Fee:", r.Fee.String())
	fmt.Println("  Fee Unit:", r.FeeUnit)
	fmt.Println("  Transaction Hash:", r.TransactionHash.String())
	fmt.Println("  Reverted:", r.Reverted)
	if r.Reverted {
		fmt.Println("  Revert Reason:", r.RevertReason)
	}

	fmt.Println("  Events:")
	for i, event := range r.Events {
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

	if r.ExecutionResources != nil {
		fmt.Println("  Execution Resources:")
		r.ExecutionResources.Print()
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
		MessagesSentHash(r.L2ToL1Message),
		revertReasonHash,
		&felt.Zero, // L2 gas consumed
		new(felt.Felt).SetUint64(totalGasConsumed.L1Gas),
		new(felt.Felt).SetUint64(totalGasConsumed.L1DataGas),
	)
}

func MessagesSentHash(messages []*L2ToL1Message) *felt.Felt {
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
