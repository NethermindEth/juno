package customsync

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknetdata"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
)

type CustomSynchronizer struct {
	blockchain          *blockchain.Blockchain
	starknetData        starknetdata.StarknetData
	startingBlockNumber uint64
}

// Main entry point into the synchronizer
// Starts the Synchronizer, or returns an error if service is already running
func (cs *CustomSynchronizer) Run() error {
	cs.syncBlocks()
	return nil
}

func NewCustomSynchronizer() *CustomSynchronizer {
	starknetdata := adaptfeeder.New(feeder.NewClient(utils.Sepolia.FeederURL))

	db, err := pebble.New("my_custom_db")
	if err != nil {
		return nil
	}
	blockchain := blockchain.New(db, &utils.Sepolia, nil)

	return &CustomSynchronizer{
		blockchain:   blockchain,
		starknetData: starknetdata,
	}
}

func (cs *CustomSynchronizer) syncBlocks() {
	height, heightErr := cs.blockchain.Height()
	if heightErr != nil && !errors.Is(heightErr, db.ErrKeyNotFound) {
		return
	}

	cs.startingBlockNumber = height

	nextBlockToSync := cs.startingBlockNumber
	for {
		// Get block + state update from Sequencer
		stateUpdate, block, stateUpdateErr := cs.starknetData.StateUpdateWithBlock(context.Background(), nextBlockToSync)
		if stateUpdateErr != nil {
			continue
		}

		// Get new classes from Sequencer
		newClasses, newClassesErr := cs.fetchNewClasses(stateUpdate)
		if newClassesErr != nil {
			continue
		}

		verifAndStoreErr := cs.verifyAndStoreBlockWithStateUpdate(block, stateUpdate, newClasses)
		if verifAndStoreErr != nil {
			continue
		}

		nextBlockToSync++
	}
}

func (cs *CustomSynchronizer) verifyAndStoreBlockWithStateUpdate(block *core.Block, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class) error {
	// Verify block integrity
	blockCommitments, sanityCheckErr := cs.blockchain.SanityCheckNewHeight(block, stateUpdate, newClasses)
	if sanityCheckErr != nil {
		return sanityCheckErr
	}

	// Update blockchain database
	// Store:
	// - newly declared classes, classes trie, new contract in storage, contractStorageTrie & stateTrie,
	// 	 update contracts storages (bc of stateUpdates) + global state trie root,
	// - block header
	// - txs & receipts
	// - state updates
	// - block commitments
	// - l1 handler msg hashes
	// - ChainHeight
	storeErr := cs.blockchain.Store(block, blockCommitments, stateUpdate, newClasses)
	if storeErr != nil {
		fmt.Println("Error when trying to store the block")
		// revert head here
		return storeErr
	} else {
		fmt.Println("Store block: {'number': ", block.Number, ", 'hash': ", block.Hash.ShortString(), ", 'root': ", block.GlobalStateRoot.ShortString(), "}")
	}

	return nil
}

func (cs *CustomSynchronizer) fetchNewClasses(stateUpdate *core.StateUpdate) (map[felt.Felt]core.Class, error) {
	newClasses := make(map[felt.Felt]core.Class)

	// Register newly declared v0 classes
	for _, v0ClassHash := range stateUpdate.StateDiff.DeclaredV0Classes {
		class, classErr := cs.fetchClassIfNotFoundLocally(v0ClassHash, newClasses)
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
		class, classErr := cs.fetchClassIfNotFoundLocally(v1ClassHash, newClasses)
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

func (cs *CustomSynchronizer) fetchClassIfNotFoundLocally(classHash *felt.Felt, newClasses map[felt.Felt]core.Class) (*core.Class, error) {
	// Make sure class has not already been added
	if _, classHashPresent := newClasses[*classHash]; classHashPresent {
		return nil, nil
	}

	// Get latest local blockchain state reader
	stateReader, stateCloser, stateErr := cs.blockchain.HeadState()
	if stateErr != nil {
		if !errors.Is(stateErr, db.ErrKeyNotFound) {
			return nil, stateErr
		}
		stateCloser = func() error {
			return nil
		}
	}
	defer func() { _ = stateCloser() }()

	// Get class if already in our local database
	classErr := db.ErrKeyNotFound
	// if DB not empty
	if stateReader != nil {
		_, classErr = stateReader.Class(classHash)
	}

	if !errors.Is(classErr, db.ErrKeyNotFound) {
		return nil, classErr
	}

	// class is not found locally, fetch it from sequencer
	class, fetchedClassErr := cs.starknetData.Class(context.Background(), classHash)
	if fetchedClassErr != nil {
		return nil, fetchedClassErr
	}

	return &class, nil
}
