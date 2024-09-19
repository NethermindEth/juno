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

func CompareReceipts(r1, r2 *TransactionReceipt) (bool, string) {
	var result strings.Builder
	foundDiff := false

	if r1.TransactionHash.Equal(r2.TransactionHash) {
		result.WriteString("TransactionHash: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"TransactionHash: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n",
			r1.TransactionHash.String(),
			r2.TransactionHash.String()))
	}

	if r1.Fee.Equal(r2.Fee) {
		result.WriteString("Fee: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Fee: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n",
			r1.Fee.String(),
			r2.Fee.String()))
	}

	if MessagesSentHash(r1.L2ToL1Message).Equal(MessagesSentHash(r2.L2ToL1Message)) {
		result.WriteString("MessagesSentHash: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"MessagesSentHash: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n",
			MessagesSentHash(r1.L2ToL1Message).String(),
			MessagesSentHash(r2.L2ToL1Message).String()))
	}

	if r1.Reverted == r2.Reverted {
		result.WriteString("Reverted: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Reverted: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n",
			r1.Reverted,
			r2.Reverted))
	}

	if r1.RevertReason == r2.RevertReason {
		result.WriteString("RevertReason: EQUAL\n")
	} else {
		result.WriteString(fmt.Sprintf(
			"RevertReason: DIFFERENT\n\t- r1: %s\n\t- r2: %s\n",
			r1.RevertReason,
			r2.RevertReason))
	}

	r1Gas := r1.ExecutionResources.TotalGasConsumed
	r2Gas := r2.ExecutionResources.TotalGasConsumed
	if r1Gas == nil {
		panic("r1 TotalGasConsumed is inil")
	}
	if r2Gas == nil {
		panic("r2 TotalGasConsumed is inil")
	}
	if r1Gas.L1Gas == r2Gas.L1Gas {
		result.WriteString("TotalGasConsumed.L1Gas: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"TotalGasConsumed.L1Gas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n",
			r1Gas.L1Gas,
			r2Gas.L1Gas))
	}

	if r1Gas.L1DataGas == r2Gas.L1DataGas {
		result.WriteString("TotalGasConsumed.L1DataGas: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"TotalGasConsumed.L1DataGas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n",
			r1Gas.L1DataGas,
			r2Gas.L1DataGas))
	}
	if len(r1.Events) != len(r2.Events) {
		foundDiff = true
		result.WriteString(fmt.Sprintf(
			"Events have DIFFERENT length \n\t- g1: %d\n\t- g2: %d\n",
			len(r1.Events),
			len(r1.Events)))
	} else {
		for ind, event := range r1.Events {
			if !event.From.Equal(r2.Events[ind].From) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT From value at index %d \n\t- g1: %v\n\t- g2: %v\n",
					ind, event.From,
					r2.Events[ind].From))
				printEvents(r1.Events, r2.Events)
				break
			}
			if !equalSlices(r1.Events[ind].Keys, r2.Events[ind].Keys) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT Keys \n\t- g1: %v\n\t- g2: %v\n",
					event.Keys,
					r2.Events[ind].Keys))
				printEvents(r1.Events, r2.Events)
				break
			}
			if !equalSlices(r1.Events[ind].Data, r2.Events[ind].Data) {
				foundDiff = true
				result.WriteString(fmt.Sprintf(
					"Events have DIFFERENT Data \n\t- g1: %v\n\t- g2: %v\n",
					event.Keys,
					r2.Events[ind].Keys))
				printEvents(r1.Events, r2.Events)
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
