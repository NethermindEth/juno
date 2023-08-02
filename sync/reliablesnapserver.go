package sync

import (
	"context"
	"errors"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

type reliableSnapServer struct {
	innerServer blockchain.SnapServer
	log         utils.Logger
}

var RetryableErr = errors.New("retryable error")

func isErrorRetryable(err error) bool {
	return errors.Is(err, RetryableErr)
}

func (r *reliableSnapServer) GetTrieRootAt(ctx context.Context, block *core.Header) (*blockchain.TrieRootInfo, error) {
	first := true

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !first {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		first = false

		rootInfo, err := r.innerServer.GetTrieRootAt(block.Hash)
		if err != nil {
			log.Warn("error fetching trie root", "err", err)
			continue
		}

		if rootInfo == nil || rootInfo.StorageRoot == nil {
			log.Warn("nil root info")
			continue
		}

		combinedRoot := core.CalculateCombinedRoot(rootInfo.StorageRoot, rootInfo.ClassRoot)
		if !block.GlobalStateRoot.Equal(combinedRoot) {
			log.Warn("global state root mismatched")
			continue
		}

		return rootInfo, nil
	}
}

func (r *reliableSnapServer) GetClassRange(ctx context.Context, classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (bool, *blockchain.ClassRangeResult, error) {
	for {
		select {
		case <-ctx.Done():
			return false, nil, ctx.Err()
		default:
		}

		response, err := r.innerServer.GetClassRange(classTrieRootHash, startAddr, limitAddr, maxNodes)
		if err != nil {
			log.Warn("error fetching class range", "err", err)
			continue
		}

		// TODO: Verify hashes
		var hasNext bool
		hasNext, err = trie.VerifyTrie(classTrieRootHash, response.Paths, response.ClassHashes, response.Proofs, crypto.Poseidon)
		if err != nil {
			log.Warn("error verifying trie", "err", err)
			continue
		}

		return hasNext, response, nil
	}
}

func (r *reliableSnapServer) GetAddressRange(ctx context.Context, rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (bool, *blockchain.AddressRangeResult, error) {

	for {
		select {
		case <-ctx.Done():
			return false, nil, ctx.Err()
		default:
		}

		starttime := time.Now()
		response, err := r.innerServer.GetAddressRange(rootHash, startAddr, limitAddr, maxNodes)
		addressDurations.WithLabelValues("get").Observe(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			log.Warn("error fetching address range", "err", err)
			continue
		}

		// TODO: Verify hashes
		starttime = time.Now()
		hasNext, err := trie.VerifyTrie(rootHash, response.Paths, response.Hashes, response.Proofs, crypto.Pedersen)
		addressDurations.WithLabelValues("verify").Observe(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			log.Warn("error verifying trie", "err", err)
			continue
		}

		return hasNext, response, nil
	}

}

func (r *reliableSnapServer) GetContractRange(storageTrieRootHash *felt.Felt, requests []*blockchain.StorageRangeRequest, maxNodes uint64) ([]*blockchain.StorageRangeResult, error) {
	return r.innerServer.GetContractRange(storageTrieRootHash, requests, maxNodes)
}
