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
		result.WriteString(fmt.Sprintf("TransactionHash: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n", r1.TransactionHash.String(), r2.TransactionHash.String()))
	}

	if r1.Fee.Equal(r2.Fee) {
		result.WriteString("Fee: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("Fee: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n", r1.Fee.String(), r2.Fee.String()))
	}

	if MessagesSentHash(r1.L2ToL1Message).Equal(MessagesSentHash(r2.L2ToL1Message)) {
		result.WriteString("MessagesSentHash: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("MessagesSentHash: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n", MessagesSentHash(r1.L2ToL1Message).String(), MessagesSentHash(r2.L2ToL1Message).String()))
	}

	if r1.Reverted == r2.Reverted {
		result.WriteString("Reverted: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("Reverted: DIFFERENT\n\t- r1: %v\n\t- r2: %v\n", r1.Reverted, r2.Reverted))
	}

	if r1.RevertReason == r2.RevertReason {
		result.WriteString("RevertReason: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("RevertReason: DIFFERENT\n\t- r1: %s\n\t- r2: %s\n", r1.RevertReason, r2.RevertReason))
	}

	r1Gas := r1.ExecutionResources.TotalGasConsumed
	r2Gas := r2.ExecutionResources.TotalGasConsumed
	if r1Gas == nil {
		panic("r1Gas is inil")
	}
	if r2Gas == nil {
		panic("r2Gas is inil")
	}
	if r1Gas.L1Gas == r2Gas.L1Gas {
		result.WriteString("L1Gas: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("L1Gas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n", r1Gas.L1Gas, r2Gas.L1Gas))
	}

	if r1Gas.L1DataGas == r2Gas.L1DataGas {
		result.WriteString("L1DataGas: EQUAL\n")
	} else {
		foundDiff = true
		result.WriteString(fmt.Sprintf("L1DataGas: DIFFERENT\n\t- g1: %d\n\t- g2: %d\n", r1Gas.L1DataGas, r2Gas.L1DataGas))
	}
	return foundDiff, result.String()
}

// func (tr *TransactionReceipt) Diff(other *TransactionReceipt, left, right string) (bool, string) {
// 	var diffs []string
// 	found := false
// 	diffs = append(diffs, fmt.Sprintf("%s vs %s", left, right))
// 	if diffFound, diff := compareFelt(tr.Fee, other.Fee, "Fee"); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if tr.FeeUnit != other.FeeUnit {
// 		diffs = append(diffs, fmt.Sprintf("  Fee Unit differs: %s vs %s", tr.FeeUnit, other.FeeUnit))
// 		found = true
// 	}
// 	if diffFound, diff := compareFelt(tr.TransactionHash, other.TransactionHash, "Transaction Hash"); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if tr.Reverted != other.Reverted {
// 		diffs = append(diffs, fmt.Sprintf("  Reverted status differs: %v vs %v", tr.Reverted, other.Reverted))
// 		found = true
// 	}
// 	if tr.RevertReason != other.RevertReason {
// 		diffs = append(diffs, fmt.Sprintf("  Revert Reason differs: %s vs %s", tr.RevertReason, other.RevertReason))
// 		found = true
// 	}

// 	if len(tr.Events) != len(other.Events) {
// 		diffs = append(diffs, fmt.Sprintf("  Number of Events differs: %d vs %d", len(tr.Events), len(other.Events)))
// 		found = true
// 	} else {
// 		for i := range tr.Events {
// 			if diffFound, diff := tr.Events[i].Diff(other.Events[i], i); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}

// 	if tr.ExecutionResources != nil && other.ExecutionResources != nil {
// 		if diffFound, diff := tr.ExecutionResources.Diff(other.ExecutionResources); diffFound {
// 			diffs = append(diffs, diff)
// 			found = true
// 		}
// 	} else if tr.ExecutionResources != other.ExecutionResources {
// 		diffs = append(diffs, "  Execution Resources differ: one is nil while the other is not")
// 		found = true
// 	}

// 	if tr.L1ToL2Message != nil && other.L1ToL2Message != nil {
// 		if diffFound, diff := tr.L1ToL2Message.Diff(other.L1ToL2Message); diffFound {
// 			diffs = append(diffs, diff)
// 			found = true
// 		}
// 	} else if tr.L1ToL2Message != other.L1ToL2Message {
// 		diffs = append(diffs, "  L1 To L2 Message differs: one is nil while the other is not")
// 		found = true
// 	}

// 	if len(tr.L2ToL1Message) != len(other.L2ToL1Message) {
// 		diffs = append(diffs, fmt.Sprintf("  Number of L2 To L1 Messages differs: %d vs %d", len(tr.L2ToL1Message), len(other.L2ToL1Message)))
// 		found = true
// 	} else {
// 		for i := range tr.L2ToL1Message {
// 			if diffFound, diff := tr.L2ToL1Message[i].Diff(other.L2ToL1Message[i], i); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (e *Event) Diff(other *Event, index int) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if diffFound, diff := compareFelt(e.From, other.From, fmt.Sprintf("Event %d From", index+1)); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if len(e.Data) != len(other.Data) {
// 		diffs = append(diffs, fmt.Sprintf("    Event %d Data length differs: %d vs %d", index+1, len(e.Data), len(other.Data)))
// 		found = true
// 	} else {
// 		for j := range e.Data {
// 			if diffFound, diff := compareFelt(e.Data[j], other.Data[j], fmt.Sprintf("Event %d Data %d", index+1, j+1)); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}
// 	if len(e.Keys) != len(other.Keys) {
// 		diffs = append(diffs, fmt.Sprintf("    Event %d Keys length differs: %d vs %d", index+1, len(e.Keys), len(other.Keys)))
// 		found = true
// 	} else {
// 		for j := range e.Keys {
// 			if diffFound, diff := compareFelt(e.Keys[j], other.Keys[j], fmt.Sprintf("Event %d Key %d", index+1, j+1)); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (er *ExecutionResources) Diff(other *ExecutionResources) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if er.MemoryHoles != other.MemoryHoles {
// 		diffs = append(diffs, fmt.Sprintf("  Memory Holes differ: %d vs %d", er.MemoryHoles, other.MemoryHoles))
// 		found = true
// 	}
// 	if er.Steps != other.Steps {
// 		diffs = append(diffs, fmt.Sprintf("  Steps differ: %d vs %d", er.Steps, other.Steps))
// 		found = true
// 	}
// 	if diffFound, diff := er.BuiltinInstanceCounter.Diff(&other.BuiltinInstanceCounter); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}

// 	if er.DataAvailability != nil && other.DataAvailability != nil {
// 		if diffFound, diff := er.DataAvailability.Diff(other.DataAvailability); diffFound {
// 			diffs = append(diffs, diff)
// 			found = true
// 		}
// 	} else if er.DataAvailability != other.DataAvailability {
// 		diffs = append(diffs, "  Data Availability differs: one is nil while the other is not")
// 		found = true
// 	}

// 	if er.TotalGasConsumed != nil && other.TotalGasConsumed != nil {
// 		if diffFound, diff := er.TotalGasConsumed.Diff(other.TotalGasConsumed); diffFound {
// 			diffs = append(diffs, diff)
// 			found = true
// 		}
// 	} else if er.TotalGasConsumed != other.TotalGasConsumed {
// 		diffs = append(diffs, "  Total Gas Consumed differs: one is nil while the other is not")
// 		found = true
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (bic *BuiltinInstanceCounter) Diff(other *BuiltinInstanceCounter) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if bic.Pedersen != other.Pedersen {
// 		diffs = append(diffs, fmt.Sprintf("  Pedersen differs: %d vs %d", bic.Pedersen, other.Pedersen))
// 		found = true
// 	}
// 	if bic.RangeCheck != other.RangeCheck {
// 		diffs = append(diffs, fmt.Sprintf("  RangeCheck differs: %d vs %d", bic.RangeCheck, other.RangeCheck))
// 		found = true
// 	}
// 	if bic.Bitwise != other.Bitwise {
// 		diffs = append(diffs, fmt.Sprintf("  Bitwise differs: %d vs %d", bic.Bitwise, other.Bitwise))
// 		found = true
// 	}
// 	if bic.Output != other.Output {
// 		diffs = append(diffs, fmt.Sprintf("  Output differs: %d vs %d", bic.Output, other.Output))
// 		found = true
// 	}
// 	if bic.Ecsda != other.Ecsda {
// 		diffs = append(diffs, fmt.Sprintf("  Ecsda differs: %d vs %d", bic.Ecsda, other.Ecsda))
// 		found = true
// 	}
// 	if bic.EcOp != other.EcOp {
// 		diffs = append(diffs, fmt.Sprintf("  EcOp differs: %d vs %d", bic.EcOp, other.EcOp))
// 		found = true
// 	}
// 	if bic.Keccak != other.Keccak {
// 		diffs = append(diffs, fmt.Sprintf("  Keccak differs: %d vs %d", bic.Keccak, other.Keccak))
// 		found = true
// 	}
// 	if bic.Poseidon != other.Poseidon {
// 		diffs = append(diffs, fmt.Sprintf("  Poseidon differs: %d vs %d", bic.Poseidon, other.Poseidon))
// 		found = true
// 	}
// 	if bic.SegmentArena != other.SegmentArena {
// 		diffs = append(diffs, fmt.Sprintf("  SegmentArena differs: %d vs %d", bic.SegmentArena, other.SegmentArena))
// 		found = true
// 	}
// 	if bic.AddMod != other.AddMod {
// 		diffs = append(diffs, fmt.Sprintf("  AddMod differs: %d vs %d", bic.AddMod, other.AddMod))
// 		found = true
// 	}
// 	if bic.MulMod != other.MulMod {
// 		diffs = append(diffs, fmt.Sprintf("  MulMod differs: %d vs %d", bic.MulMod, other.MulMod))
// 		found = true
// 	}
// 	if bic.RangeCheck96 != other.RangeCheck96 {
// 		diffs = append(diffs, fmt.Sprintf("  RangeCheck96 differs: %d vs %d", bic.RangeCheck96, other.RangeCheck96))
// 		found = true
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (gc *GasConsumed) Diff(other *GasConsumed) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if gc.L1Gas != other.L1Gas {
// 		diffs = append(diffs, fmt.Sprintf("    L1 Gas differs: %d vs %d", gc.L1Gas, other.L1Gas))
// 		found = true
// 	}
// 	if gc.L1DataGas != other.L1DataGas {
// 		diffs = append(diffs, fmt.Sprintf("    L1 Data Gas differs: %d vs %d", gc.L1DataGas, other.L1DataGas))
// 		found = true
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (da *DataAvailability) Diff(other *DataAvailability) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if da.L1Gas != other.L1Gas {
// 		diffs = append(diffs, fmt.Sprintf("    L1 Gas differs: %d vs %d", da.L1Gas, other.L1Gas))
// 		found = true
// 	}
// 	if da.L1DataGas != other.L1DataGas {
// 		diffs = append(diffs, fmt.Sprintf("    L1 Data Gas differs: %d vs %d", da.L1DataGas, other.L1DataGas))
// 		found = true
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (msg *L1ToL2Message) Diff(other *L1ToL2Message) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if msg.From != other.From {
// 		diffs = append(diffs, fmt.Sprintf("  L1ToL2Message From differs: %s vs %s", msg.From.String(), other.From.String()))
// 		found = true
// 	}
// 	if diffFound, diff := compareFelt(msg.Nonce, other.Nonce, "L1ToL2Message Nonce"); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if len(msg.Payload) != len(other.Payload) {
// 		diffs = append(diffs, fmt.Sprintf("  L1ToL2Message Payload length differs: %d vs %d", len(msg.Payload), len(other.Payload)))
// 		found = true
// 	} else {
// 		for i := range msg.Payload {
// 			if diffFound, diff := compareFelt(msg.Payload[i], other.Payload[i], fmt.Sprintf("L1ToL2Message Payload %d", i+1)); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}
// 	if diffFound, diff := compareFelt(msg.Selector, other.Selector, "L1ToL2Message Selector"); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if diffFound, diff := compareFelt(msg.To, other.To, "L1ToL2Message To"); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}

// 	return found, strings.Join(diffs, "\n")
// }

// func (msg *L2ToL1Message) Diff(other *L2ToL1Message, index int) (bool, string) {
// 	var diffs []string
// 	found := false

// 	if diffFound, diff := compareFelt(msg.From, other.From, fmt.Sprintf("L2ToL1Message %d From", index+1)); diffFound {
// 		diffs = append(diffs, diff)
// 		found = true
// 	}
// 	if len(msg.Payload) != len(other.Payload) {
// 		diffs = append(diffs, fmt.Sprintf("  L2ToL1Message %d Payload length differs: %d vs %d", index+1, len(msg.Payload), len(other.Payload)))
// 		found = true
// 	} else {
// 		for i := range msg.Payload {
// 			if diffFound, diff := compareFelt(msg.Payload[i], other.Payload[i], fmt.Sprintf("L2ToL1Message %d Payload %d", index+1, i+1)); diffFound {
// 				diffs = append(diffs, diff)
// 				found = true
// 			}
// 		}
// 	}
// 	if msg.To != other.To {
// 		diffs = append(diffs, fmt.Sprintf("  L2ToL1Message %d To differs: %s vs %s", index+1, msg.To.String(), other.To.String()))
// 		found = true
// 	}

//		return found, strings.Join(diffs, "\n")
//	}
//
//	func compareFelt(a, b *felt.Felt, name string) (bool, string) {
//		if !a.Equal(b) {
//			return true, fmt.Sprintf("  %s differs: %s vs %s", name, a.String(), b.String())
//		}
//		return false, ""
//	}
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
