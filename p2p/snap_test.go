package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

type MutableStorage interface {
	SetClasss(path *felt.Felt, classHash *felt.Felt, class *core.Class) error
	SetAddress(path *felt.Felt, storageRoot *felt.Felt, classHash *felt.Felt, nonce *felt.Felt) error
	SetStorage(storagePath *felt.Felt, path *felt.Felt, value *felt.Felt) error
}

func CopyTrieProcedure(server core.SnapServer, targetTrie MutableStorage, blockHash *felt.Felt) error {
	// TODO: refresh
	rootInfo, err := server.GetTrieRootAt(blockHash)
	if err != nil {
		return err
	}

	startAddr := &felt.Zero
	hasNext := false
	for hasNext {
		response, err := server.GetClassRange(rootInfo.ClassRoot, startAddr, nil)
		if err != nil {
			return err
		}

		// TODO: Verify class hashes
		hasNext, err = verifyTrie(rootInfo.ClassRoot, response.Paths, response.ClassHashes, response.Proofs)
		if err != nil {
			return err
		}

		for i, path := range response.Paths {
			err := targetTrie.SetClasss(path, response.ClassHashes[i], response.Classes[i])
			if err != nil {
				return err
			}
		}

		startAddr = response.Paths[len(response.Paths)-1]
	}

	storageJobs := make([]*core.StorageRangeRequest, 0)

	startAddr = &felt.Zero
	hasNext = false
	for hasNext {
		response, err := server.GetAddressRange(rootInfo.ClassRoot, startAddr, nil)
		if err != nil {
			return err
		}

		// TODO: Verify hashes
		hasNext, err = verifyTrie(rootInfo.ClassRoot, response.Paths, response.Hashes, response.Proofs)
		if err != nil {
			return err
		}

		for i, path := range response.Paths {
			err := targetTrie.SetAddress(path, response.Leaves[i].StorageRoot, response.Leaves[i].ClassHash, response.Leaves[i].Nonce)
			if err != nil {
				return err
			}

			storageJobs = append(storageJobs, &core.StorageRangeRequest{
				Path: path,
				Hash: response.Leaves[i].StorageRoot,
			})
		}

		startAddr = response.Paths[len(response.Paths)-1]
	}

	longRangeJobs := make([]*core.StorageRangeRequest, 0)

	curIdx := 0
	for curIdx < len(storageJobs) {
		batchSize := 1024
		batch := make([]*core.StorageRangeRequest, 0)
		for i := 0; i < batchSize; i++ {
			if i+curIdx >= len(storageJobs) {
				break
			}

			batch = append(batch, storageJobs[i+curIdx])
		}

		requests := make([]*core.StorageRangeRequest, len(batch))
		for _, job := range batch {
			requests = append(requests, job)
		}

		responses, err := server.GetContractRange(rootInfo.StorageRoot, requests)
		if err != nil {
			return err
		}

		// TODO: it could be only some is available
		curIdx = len(responses)

		for i, response := range responses {
			request := requests[i]
			hasNext, err := verifyTrie(request.Hash, response.Paths, response.Values, response.Proofs)
			if err != nil {
				return err
			}
			if hasNext {
				batch[i].StartAddr = response.Paths[len(response.Paths)-1]
				longRangeJobs = append(longRangeJobs, batch[i])
			}

			for idx, path := range response.Paths {
				err = targetTrie.SetStorage(request.Path, path, response.Paths[idx])
				if err != nil {
					return err
				}
			}
		}
	}

	for _, job := range longRangeJobs {
		startAddr = job.StartAddr
		hasNext = false
		for hasNext {
			responses, err := server.GetContractRange(rootInfo.StorageRoot, []*core.StorageRangeRequest{job})
			if err != nil {
				return err
			}

			response := responses[0] // TODO: it can return nothing

			// TODO: Verify hashes
			hasNext, err = verifyTrie(rootInfo.ClassRoot, response.Paths, response.Values, response.Proofs)
			if err != nil {
				return err
			}

			for idx, path := range response.Paths {
				err = targetTrie.SetStorage(job.Path, path, response.Values[idx])
				if err != nil {
					return err
				}
			}

			startAddr = response.Paths[len(response.Paths)-1]
		}
	}

	return nil
}

func verifyTrie(expectedRoot *felt.Felt, paths []*felt.Felt, hashes []*felt.Felt, proofs []trie.ProofNode) (bool, error) {

}

func TestCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch/largejuno", utils.NewNopZapLogger())
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	state, closer, err := bc.HeadState()
	if err != nil {
		panic(err)
	}
	defer closer()

	stateAsStorage := state.(core.StateReaderStorage)

	storageTrie, closer, err := stateAsStorage.StorageTrie()
	if err != nil {
		panic(err)
	}
	defer closer()

	os.RemoveAll("/home/amirul/fastworkscratch/scratchtrie")
	db2, _ := pebble.New("/home/amirul/fastworkscratch/scratchtrie", utils.NewNopZapLogger())
	tx2 := db2.NewTransaction(true)
	state2 := core.NewState(tx2)
	storageTrie2, closer2, err := state2.StorageTrie()
	if err != nil {
		panic(err)
	}
	defer closer2()

	startTime := time.Now()
	idx := 0
	err = storageTrie.Iterate(&felt.Zero, func(key *felt.Felt, value *felt.Felt) bool {
		idx++
		if idx%1000 == 0 {
			duration := time.Now().Sub(startTime)
			startTime = time.Now()

			storageTrie2.Root()

			duration2 := time.Now().Sub(startTime)
			startTime = time.Now()

			fmt.Printf("Progress %d %s %s\n", idx, duration, duration2)
		}
		_, err := storageTrie2.Put(key, value)
		if err != nil {
			panic(err)
		}
		return true
	})

	startTime = time.Now()
	rt1, err := storageTrie.Root()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Calculating root\n")
	rt2, err := storageTrie2.Root()
	if err != nil {
		panic(err)
	}

	duration := time.Now().Sub(startTime)
	startTime = time.Now()
	fmt.Printf("Root calculated in %s\n", duration)

	assert.Equal(t, rt1, rt2)

	if err != nil {
		panic(err)
	}
}
