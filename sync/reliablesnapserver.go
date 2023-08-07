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
			r.log.Warnw("error fetching trie root", "err", err)
			continue
		}

		if rootInfo == nil || rootInfo.StorageRoot == nil {
			r.log.Warnw("nil root info")
			continue
		}

		combinedRoot := core.CalculateCombinedRoot(rootInfo.StorageRoot, rootInfo.ClassRoot)
		if !block.GlobalStateRoot.Equal(combinedRoot) {
			r.log.Warnw("global state root mismatched")
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
			r.log.Warnw("error fetching class range", "err", err)
			continue
		}

		// TODO: Verify hashes
		var hasNext bool
		hasNext, err = trie.VerifyTrie(classTrieRootHash, response.Paths, response.ClassCommitments, response.Proofs, 251, crypto.Poseidon)
		if err != nil {
			r.log.Warnw("error verifying trie", "err", err)
			continue
		}

		return hasNext, response, nil
	}
}

func (r *reliableSnapServer) GetClasses(ctx context.Context, classes []*felt.Felt) ([]core.Class, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		response, err := r.innerServer.GetClasses(classes)
		if err != nil {
			r.log.Warnw("error fetching class range", "err", err)
			continue
		}

		// TODO: Verify hashes
		return response, nil
	}
}

func (r *reliableSnapServer) GetAddressRange(ctx context.Context, rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (bool, *blockchain.AddressRangeResult, []*felt.Felt, error) {
	for {
		select {
		case <-ctx.Done():
			return false, nil, nil, ctx.Err()
		default:
		}

		starttime := time.Now()
		response, err := r.innerServer.GetAddressRange(rootHash, startAddr, limitAddr, maxNodes)
		addressDurations.WithLabelValues("get").Observe(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			r.log.Warnw("error fetching address range", "err", err)
			continue
		}

		hashes := make([]*felt.Felt, len(response.Leaves))
		for i, leaf := range response.Leaves {
			hashes[i] = core.CalculateContractCommitment(leaf.ContractStorageRoot, leaf.ClassHash, leaf.Nonce)
		}

		starttime = time.Now()
		hasNext, err := trie.VerifyTrie(rootHash, response.Paths, hashes, response.Proofs, 251, crypto.Pedersen)
		addressDurations.WithLabelValues("verify").Observe(float64(time.Now().Sub(starttime).Microseconds()))
		if err != nil {
			r.log.Warnw("error verifying trie", "err", err)
			continue
		}

		return hasNext, response, nil, nil
	}

}

func (r *reliableSnapServer) GetContractRange(ctx context.Context, storageTrieRootHash *felt.Felt, requests []*blockchain.StorageRangeRequest, maxNodes, maxNodesPerContract uint64) ([]*blockchain.StorageRangeResult, []bool, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		// TODO: sometimes the request.Hash is zero and it has no path at all. Might as well not request at all
		responses, err := r.innerServer.GetContractRange(storageTrieRootHash, requests, maxNodes, maxNodesPerContract)
		if err != nil {
			r.log.Warnw("error fetching class range", "err", err)
			continue
		}

		hasnexes := make([]bool, 0)

		for i, response := range responses {
			request := requests[i]

			if len(response.Paths) == 0 {
				hasnexes = append(hasnexes, false)
				continue
			}

			if response.UpdatedContract != nil {
				commitment := core.CalculateContractCommitment(response.UpdatedContract.ContractStorageRoot, response.UpdatedContract.ClassHash, response.UpdatedContract.Nonce)
				r.log.Infow("Updating hash in storage", "path", request.Path.String(), "to", commitment.String(), "nonce", response.UpdatedContract.Nonce.String(), "classHash", response.UpdatedContract.ClassHash, "storageRoot", response.UpdatedContract.ContractStorageRoot)
				_, err := trie.VerifyTrie(storageTrieRootHash, []*felt.Felt{request.Path}, []*felt.Felt{commitment}, response.UpdatedContractProof, 251, crypto.Pedersen)
				if err != nil {
					r.log.Warnw("error fetching storage range. updated contract verification failed", "err", err)
					continue
				}

				request.Hash = response.UpdatedContract.ContractStorageRoot
			}

			starttime := time.Now()
			hasNext, err := trie.VerifyTrie(request.Hash, response.Paths, response.Values, response.Proofs, 251, crypto.Pedersen)
			storageDurations.WithLabelValues("verify").Add(float64(time.Now().Sub(starttime).Microseconds()))
			if err != nil {
				r.log.Warnw("error fetching storage range. updated contract verification failed", "err", err)
				continue
			}

			hasnexes = append(hasnexes, hasNext)
		}

		return responses, hasnexes, nil
	}
}
