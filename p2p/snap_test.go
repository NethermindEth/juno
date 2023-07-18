package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
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
	SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error
	SetAddress(path *felt.Felt, nodeHash *felt.Felt, storageRoot *felt.Felt, classHash *felt.Felt, nonce *felt.Felt) error
	SetStorage(storagePath *felt.Felt, path *felt.Felt, value *felt.Felt) error
}

func CopyTrieProcedure(server core.SnapServer, targetTrie MutableStorage, blockHash *felt.Felt) error {
	// TODO: refresh
	rootInfo, err := server.GetTrieRootAt(blockHash)
	if err != nil {
		return err
	}

	startAddr := &felt.Zero
	hasNext := true
	for hasNext {
		response, err := server.GetClassRange(rootInfo.ClassRoot, startAddr, nil)
		if err != nil {
			return err
		}

		// TODO: Verify class hashes
		hasNext, err = trie.VerifyTrie(rootInfo.ClassRoot, response.Paths, response.ClassHashes, response.Proofs, crypto.Poseidon)
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
	hasNext = true
	for hasNext {
		response, err := server.GetAddressRange(rootInfo.StorageRoot, startAddr, nil)
		if err != nil {
			return err
		}

		// TODO: Verify hashes
		hasNext, err = trie.VerifyTrie(rootInfo.StorageRoot, response.Paths, response.Hashes, response.Proofs, crypto.Pedersen)
		if err != nil {
			return err
		}

		for i, path := range response.Paths {
			err := targetTrie.SetAddress(path, response.Hashes[i], response.Leaves[i].StorageRoot, response.Leaves[i].ClassHash, response.Leaves[i].Nonce)
			if err != nil {
				return err
			}

			storageJobs = append(storageJobs, &core.StorageRangeRequest{
				Path:      path,
				Hash:      response.Leaves[i].StorageRoot,
				StartAddr: &felt.Zero,
			})
		}

		startAddr = response.Paths[len(response.Paths)-1]
	}

	longRangeJobs := make([]*core.StorageRangeRequest, 0)

	curIdx := 0
	for curIdx < len(storageJobs) {
		batchSize := 1
		batch := make([]*core.StorageRangeRequest, 0)
		for i := 0; i < batchSize; i++ {
			if i+curIdx >= len(storageJobs) {
				break
			}

			batch = append(batch, storageJobs[i+curIdx])
		}

		requests := make([]*core.StorageRangeRequest, 0, len(batch))
		for _, job := range batch {
			requests = append(requests, job)
		}

		responses, err := server.GetContractRange(rootInfo.StorageRoot, requests)
		if err != nil {
			return err
		}

		// TODO: it could be only some is available
		curIdx += len(responses)

		for i, response := range responses {
			request := requests[i]
			hasNext, err := trie.VerifyTrie(request.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
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
		hasNext = true
		for hasNext {
			responses, err := server.GetContractRange(rootInfo.StorageRoot, []*core.StorageRangeRequest{job})
			if err != nil {
				return err
			}

			response := responses[0] // TODO: it can return nothing

			// TODO: Verify hashes
			hasNext, err = trie.VerifyTrie(rootInfo.StorageRoot, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
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

func TestSnapCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch/smalljuno", utils.NewNopZapLogger())
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	state, closer, err := bc.HeadState()
	if err != nil {
		panic(err)
	}
	defer closer()

	err = CopyTrieProcedure(state.(core.SnapServer), &noopTrie{}, &felt.Zero)
	assert.NoError(t, err)
}

func TestCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch/smalljuno", utils.NewNopZapLogger())
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

	r, _ := storageTrie.Root()
	fmt.Printf("Root is %s", r.String())

	startTime := time.Now()
	idx := 0
	err = storageTrie.Iterate(&felt.Zero, func(key *felt.Felt, value *felt.Felt) (bool, error) {
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
			return false, err
		}
		return true, nil
	})

	assert.NoError(t, err)

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

type noopTrie struct {
}

func (n noopTrie) SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error {
	fmt.Printf("Class %s %s %s\n", path.String(), classHash.String(), class)
	return nil
}

func (n noopTrie) SetAddress(path *felt.Felt, nodeHash *felt.Felt, storageRoot *felt.Felt, classHash *felt.Felt, nonce *felt.Felt) error {
	fmt.Printf("Address %s %s %s %s %s\n", path.String(), nodeHash.String(), storageRoot.String(), classHash.String(), nonce.String())
	return nil
}

func (n noopTrie) SetStorage(storagePath *felt.Felt, path *felt.Felt, value *felt.Felt) error {
	fmt.Printf("Storage %s %s %s\n", storagePath.String(), path.String(), value.String())
	return nil
}

var _ MutableStorage = &noopTrie{}
