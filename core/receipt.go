package core

import (
	"runtime"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
)

type GasConsumed struct {
	L1Gas     uint64 `cbor:"1,keyasint"`
	L1DataGas uint64 `cbor:"2,keyasint"`
	L2Gas     uint64 `cbor:"3,keyasint"`
}

type TransactionReceipt struct {
	Fee                *felt.Felt          `cbor:"1,keyasint"`
	FeeUnit            FeeUnit             `cbor:"2,keyasint"`
	Events             []*Event            `cbor:"3,keyasint"`
	ExecutionResources *ExecutionResources `cbor:"4,keyasint"`
	L1ToL2Message      *L1ToL2Message      `cbor:"5,keyasint"`
	L2ToL1Message      []*L2ToL1Message    `cbor:"6,keyasint"`
	TransactionHash    *felt.Felt          `cbor:"7,keyasint"`
	Reverted           bool                `cbor:"8,keyasint"`
	RevertReason       string              `cbor:"9,keyasint"`
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
	return calculateCommitment(
		receipts,
		trie.RunOnTempTriePoseidon,
		func(receipt *TransactionReceipt) *felt.Felt {
			return receipt.hash()
		},
	)
}

type (
	onTempTrieFunc     func(uint8, func(*trie.Trie) error) error
	processFunc[T any] func(T) *felt.Felt
)

// General function for parallel processing of items and calculation of a commitment
func calculateCommitment[T any](items []T, runOnTempTrie onTempTrieFunc, process processFunc[T]) (*felt.Felt, error) {
	var commitment *felt.Felt
	return commitment, runOnTempTrie(commitmentTrieHeight, func(trie *trie.Trie) error {
		numWorkers := min(runtime.GOMAXPROCS(0), len(items))
		results := make([]*felt.Felt, len(items))
		var wg sync.WaitGroup
		wg.Add(numWorkers)

		jobs := make(chan int, len(items))
		for idx := range items {
			jobs <- idx
		}
		close(jobs)

		for range numWorkers {
			go func() {
				defer wg.Done()
				for i := range jobs {
					results[i] = process(items[i])
				}
			}()
		}

		wg.Wait()

		for i, res := range results {
			key := new(felt.Felt).SetUint64(uint64(i))
			if _, err := trie.Put(key, res); err != nil {
				return err
			}
		}

		root, err := trie.Root()
		if err != nil {
			return err
		}
		commitment = &root

		return nil
	})
}
