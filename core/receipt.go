package core

import (
	"runtime"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
)

type GasConsumed struct {
	L1Gas     uint64
	L1DataGas uint64
	L2Gas     uint64
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

func (r *TransactionReceipt) hash() felt.Felt {
	revertReasonHash := &felt.Zero
	if r.Reverted {
		hash := crypto.StarknetKeccak([]byte(r.RevertReason))
		revertReasonHash = &hash
	}

	var totalGasConsumed GasConsumed
	// pre 0.13.2 TotalGasConsumed property is not set, in this case we rely on zero value above
	if r.ExecutionResources != nil && r.ExecutionResources.TotalGasConsumed != nil {
		totalGasConsumed = *r.ExecutionResources.TotalGasConsumed
	}

	sentMessageHash := messagesSentHash(r.L2ToL1Message)
	l1GasFelt := felt.FromUint64[felt.Felt](totalGasConsumed.L1Gas)
	l1DataGasFelt := felt.FromUint64[felt.Felt](totalGasConsumed.L1DataGas)
	return crypto.PoseidonArray(
		r.TransactionHash,
		r.Fee,
		&sentMessageHash,
		revertReasonHash,
		&felt.Zero, // L2 gas consumed
		&l1GasFelt,
		&l1DataGasFelt,
	)
}

func messagesSentHash(messages []*L2ToL1Message) felt.Felt {
	chain := []*felt.Felt{
		new(felt.Felt).SetUint64(uint64(len(messages))),
	}
	for _, msg := range messages {
		msgTo := felt.FromBytes[felt.Felt](msg.To.Bytes())
		payloadSize := felt.FromUint64[felt.Felt](uint64(len(msg.Payload)))
		chain = append(chain, msg.From, &msgTo, &payloadSize)
		chain = append(chain, msg.Payload...)
	}

	return crypto.PoseidonArray(chain...)
}

func receiptCommitment(receipts []*TransactionReceipt) (felt.Felt, error) {
	return calculateCommitment(
		receipts,
		trie2.RunOnTempTriePoseidon,
		func(receipt *TransactionReceipt) felt.Felt {
			return receipt.hash()
		},
	)
}

type (
	onTempTrieFunc     func(uint8, func(*trie2.Trie) error) error
	processFunc[T any] func(T) felt.Felt
)

// General function for parallel processing of items and calculation of a commitment
func calculateCommitment[T any](
	items []T,
	runOnTempTrie onTempTrieFunc,
	process processFunc[T],
) (felt.Felt, error) {
	var commitment *felt.Felt
	return *commitment, runOnTempTrie(commitmentTrieHeight, func(trie *trie2.Trie) error {
		numWorkers := min(runtime.GOMAXPROCS(0), len(items))
		results := make([]felt.Felt, len(items))
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
			key := felt.FromUint64[felt.Felt](uint64(i))
			if err := trie.Update(&key, &res); err != nil {
				return err
			}
		}

		root, err := trie.Hash()
		if err != nil {
			return err
		}
		commitment = &root

		return nil
	})
}
