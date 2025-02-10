package customsync

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
)

type CustomSynchronizer struct {
	blockchain *blockchain.Blockchain
	// db                  db.DB
	readOnlyBlockchain  bool
	starknetData        starknetdata.StarknetData
	startingBlockNumber uint64
	highestBlockHeader  *core.Header
}

// Main entry point into the synchronizer
// Starts the Synchronizer, or returns an error if service is already running
func (cs *CustomSynchronizer) Run(ctx context.Context) error {
	cs.syncBlocks(ctx)
	return nil
}

func (cs *CustomSynchronizer) syncBlocks(ctx context.Context) {
	tipHeader, headerErr := cs.blockchain.HeadsHeader()
	if headerErr != nil {
		return
	}
	cs.highestBlockHeader = tipHeader
	cs.startingBlockNumber = tipHeader.Number + 1

	nextBlockToSync := cs.startingBlockNumber
	for {
		// Get block + state update from Sequencer
		stateUpdate, block, err := cs.starknetData.StateUpdateWithBlock(ctx, nextBlockToSync)
		if err != nil {
			continue
		}

		// Get new classes from Sequencer
		newClasses, newClassesErr := cs.fetchNewClasses(ctx, stateUpdate)
		if newClassesErr != nil {
			continue
		}

		cs.verifyAndStoreBlockWithStateUpdate(ctx, block, stateUpdate, newClasses)

		nextBlockToSync++
	}
}

func (cs *CustomSynchronizer) verifyAndStoreBlockWithStateUpdate(ctx context.Context, block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) {
	// Verify block integrity
	blockCommitments, checkErr := cs.blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	if checkErr != nil {
		// do smth
	}

	// Update blockchain database
	// Store:
	// - newly declared classes, classes trie, new contract in storage, (maybe contractStorageTrie also) & stateTrie,
	// 	 update contracts storages (bc of stateUpdates) + global state trie root,
	// - block header (stringified kinda)
	// - txs & receipts (stringified kinda)
	// - state updates (stringified kinda)
	// - block commitments (stringified kinda)
	// - l1 handler msg hashes: `handler msg hash` -> `tx hash`
	// - ChainHeight
	storeErr := cs.blockchain.Store(block, blockCommitments, stateUpdate, newClasses)
	if storeErr != nil {
		// do smth
		if errors.Is(storeErr, blockchain.ErrParentDoesNotMatchHead) {
			revertHeadErr := cs.blockchain.RevertHead()
			if revertHeadErr != nil {
				// cs.lo
			}
		}
	}

}

func (cs *CustomSynchronizer) fetchNewClasses(ctx context.Context, stateUpdate *core.StateUpdate) (map[felt.Felt]core.Class, error) {
	newClasses := make(map[felt.Felt]core.Class)

	// Register newly declared v0 classes
	for _, v0ClassHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		class, classErr := cs.fetchClassIfNotFoundLocally(ctx, v0ClassHash, newClasses)
		if classErr != nil {
			return nil, classErr
		}
		// class == nil means it is already in `newClasses` map
		if class != nil {
			newClasses[*v0ClassHash] = *class
		}
	}

	// Register newly declared v1 classes
	for _, v1ClassHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		class, classErr := cs.fetchClassIfNotFoundLocally(ctx, v1ClassHash, newClasses)
		if classErr != nil {
			return nil, classErr
		}
		// class == nil means it is already in `newClasses` map
		if class != nil {
			newClasses[*v1ClassHash] = *class
		}
	}

	return newClasses, nil
}

func (cs *CustomSynchronizer) fetchClassIfNotFoundLocally(ctx context.Context, classHash *felt.Felt, newClasses map[felt.Felt]core.Class) (*core.Class, error) {
	// Make sure class has not already been added
	if _, classHashPresent := newClasses[*classHash]; classHashPresent {
		return nil, nil
	}

	// Get latest local blockchain state reader
	stateReader, stateCloser, stateErr := cs.blockchain.HeadState()
	defer stateCloser()
	if stateErr != nil {
		return nil, stateErr
	}

	// Get class if already in our local database
	declaredClass, classErr := stateReader.Class(classHash)
	if !errors.Is(classErr, db.ErrKeyNotFound) {
		return nil, classErr
	}
	if classErr == nil {
		return &declaredClass.Class, nil
	}

	// Fetch class from Sequencer
	classDef, fetchedClassErr := cs.starknetData.Class(ctx, classHash)
	return &classDef, fetchedClassErr
}
