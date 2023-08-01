package sync

import (
	"context"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

type reliableSnapServer struct {
	innerServer blockchain.SnapServer
	log         utils.Logger
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

func (r *reliableSnapServer) GetClassRange(classTrieRootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (*blockchain.ClassRangeResult, error) {
	return r.innerServer.GetClassRange(classTrieRootHash, startAddr, limitAddr, maxNodes)
}

func (r *reliableSnapServer) GetAddressRange(rootHash *felt.Felt, startAddr *felt.Felt, limitAddr *felt.Felt, maxNodes uint64) (*blockchain.AddressRangeResult, error) {
	return r.innerServer.GetAddressRange(rootHash, startAddr, limitAddr, maxNodes)
}

func (r *reliableSnapServer) GetContractRange(storageTrieRootHash *felt.Felt, requests []*blockchain.StorageRangeRequest, maxNodes uint64) ([]*blockchain.StorageRangeResult, error) {
	return r.innerServer.GetContractRange(storageTrieRootHash, requests, maxNodes)
}
