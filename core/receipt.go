package core

import (
	"runtime"
	"sync"

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

func ReceiptCommitment(receipts []*TransactionReceipt) (*felt.Felt, error) {
	type result struct {
		key   *felt.Felt
		value *felt.Felt
	}
	var commitment *felt.Felt
	return commitment, trie.RunOnTempTriePoseidon(commitmentTrieHeight, func(trie *trie.Trie) error {
		numWorkers := min(runtime.GOMAXPROCS(0), len(receipts))
		resultChan := make(chan result, len(receipts))
		var trieMutex sync.Mutex

		errChan := make(chan error, numWorkers)

		var wg sync.WaitGroup
		wg.Add(numWorkers)

		receiptsPerWorker := max(1, (len(receipts)+numWorkers-1)/numWorkers)

		for workerID := 0; workerID < numWorkers; workerID++ {
			go func(id int) {
				defer wg.Done()
				startIdx := id * receiptsPerWorker
				endIdx := min(startIdx+receiptsPerWorker, len(receipts))

				for i, receipt := range receipts[startIdx:endIdx] {
					receiptTrieKey := new(felt.Felt).SetUint64(uint64(startIdx + i))
					receiptHash := receipt.hash()
					resultChan <- result{receiptTrieKey, receiptHash}
				}

				errChan <- nil
			}(workerID)
		}

		go func() {
			wg.Wait()
			close(resultChan)
		}()

		go func() {
			for result := range resultChan {
				trieMutex.Lock()
				_, err := trie.Put(result.key, result.value)
				trieMutex.Unlock()
				if err != nil {
					errChan <- err
					return
				}
			}
			errChan <- nil
		}()

		for i := 0; i < numWorkers+1; i++ {
			if err := <-errChan; err != nil {
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
