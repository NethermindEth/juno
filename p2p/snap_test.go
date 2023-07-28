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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"math/big"
	"os"
	"testing"
	"time"
)

type MutableStorage interface {
	SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error
	SetAddress(paths []*felt.Felt, nodeHashes []*felt.Felt, classHashes []*felt.Felt, nonces []*felt.Felt) error
	SetStorage(storagePath *felt.Felt, paths []*felt.Felt, values []*felt.Felt) error
	GetStateRoot() (*felt.Felt, error)
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

		if len(response.Paths) == 0 {
			// TODO: Usually we try another peer in this case
			break
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

	maxint := big.NewInt(1)
	maxint.Lsh(maxint, 251)
	startAddr = &felt.Zero
	hasNext = true
	for hasNext {
		theint := startAddr.BigInt(big.NewInt(0))
		theint.Mul(theint, big.NewInt(100))
		theint.Div(theint, maxint)

		curstateroot, err := targetTrie.GetStateRoot()
		if err != nil {
			return err
		}

		fmt.Printf("Startadd is %s%% %s \n", theint.String(), curstateroot.String())

		response, err := server.GetAddressRange(rootInfo.StorageRoot, startAddr, nil)
		if err != nil {
			return errors.Wrap(err, "error get address range")
		}

		// TODO: Verify hashes
		hasNext, err = trie.VerifyTrie(rootInfo.StorageRoot, response.Paths, response.Hashes, response.Proofs, crypto.Pedersen)
		if err != nil {
			return errors.Wrap(err, "error verifying tree")
		}

		classHashes := make([]*felt.Felt, 0)
		nonces := make([]*felt.Felt, 0)

		for i := range response.Paths {
			classHashes = append(classHashes, response.Leaves[i].ClassHash)
			nonces = append(nonces, response.Leaves[i].Nonce)
		}

		err = targetTrie.SetAddress(response.Paths, response.Hashes, classHashes, nonces)
		if err != nil {
			return errors.Wrap(err, "error setting address")
		}

		for i, path := range response.Paths {
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

		curstateroot, err := targetTrie.GetStateRoot()
		if err != nil {
			return err
		}

		fmt.Printf("storage jobs %d/%d %s\n", curIdx, len(storageJobs), curstateroot.String())

		batchSize := 1000
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
			fmt.Printf("Contract range failed\n")
			request := requests[0]
			fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
			return err
		}

		// TODO: it could be only some is available
		curIdx += len(responses)

		for i, response := range responses {
			request := requests[i]
			if len(response.Paths) == 0 {
				// TODO: need to check if its really empty
				continue
			}

			hasNext, err := trie.VerifyTrie(request.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			if err != nil {
				fmt.Printf("Verification failed\n")
				fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
				for i, path := range response.Paths {
					fmt.Printf("S %s -> %s\n", path.String(), response.Values[i].String())
				}

				return err
			}
			if hasNext {
				fmt.Printf("Adding next\n")
				batch[i].StartAddr = response.Paths[len(response.Paths)-1]
				longRangeJobs = append(longRangeJobs, batch[i])
			}

			err = targetTrie.SetStorage(request.Path, response.Paths, response.Values)
			if err != nil {
				fmt.Printf("Set storage failed\n")
				fmt.Printf("Request %s %s\n", request.Hash.String(), request.Path.String())
				return err
			}
		}
	}

	fmt.Printf("Starting long range\n")
	for lri, job := range longRangeJobs {

		curstateroot, err := targetTrie.GetStateRoot()
		if err != nil {
			return err
		}

		fmt.Printf("long range %d/%d %s\n", lri, len(longRangeJobs), curstateroot.String())

		startAddr = job.StartAddr
		hasNext = true
		for hasNext {
			theint := startAddr.BigInt(big.NewInt(0))
			theint.Mul(theint, big.NewInt(100))
			theint.Div(theint, maxint)
			fmt.Printf("long range %s %s %s%% %s %v\n", job.Path, job.Hash, theint.String(), startAddr.String(), hasNext)

			job.StartAddr = startAddr
			responses, err := server.GetContractRange(rootInfo.StorageRoot, []*core.StorageRangeRequest{job})
			if err != nil {
				fmt.Printf("Contract range failed\n")
				return err
			}

			response := responses[0] // TODO: it can return nothing

			// TODO: Verify hashes
			hasNext, err = trie.VerifyTrie(job.Hash, response.Paths, response.Values, response.Proofs, crypto.Pedersen)
			if err != nil {
				fmt.Printf("Contract range verify failed %s %s\n", rootInfo.StorageRoot.String(), rootInfo.ClassRoot.String())
				return err
			}

			err = targetTrie.SetStorage(job.Path, response.Paths, response.Values)
			if err != nil {
				fmt.Printf("Contract range storage failed\n")
				return err
			}

			startAddr = response.Paths[len(response.Paths)-1]
		}
	}

	stateroot, err := targetTrie.GetStateRoot()
	if err != nil {
		return err
	}
	if !stateroot.Equal(rootInfo.StorageRoot) {
		fmt.Printf("State root match mismatch %s %s\n", stateroot.String(), rootInfo.StorageRoot.String())
	} else {
		fmt.Printf("State root match\n")
	}

	return nil
}

func TestSnapCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/fastworkscratch/largejuno", utils.NewNopZapLogger())
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	state, closer, err := bc.HeadState()
	if err != nil {
		panic(err)
	}
	defer closer()

	os.RemoveAll("/home/amirul/fastworkscratch/atragetJuno")
	var d2 db.DB
	d2, _ = pebble.New("/home/amirul/fastworkscratch/atragetJuno", utils.NewNopZapLogger())
	bc2 := blockchain.New(d2, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	target := &noopTrie{
		blockchain: bc2,
	}

	err = CopyTrieProcedure(state.(core.SnapServer), target, &felt.Zero)
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
	addrCount  int
	blockchain *blockchain.Blockchain
}

func (n *noopTrie) GetStateRoot() (*felt.Felt, error) {
	state, close, err := n.blockchain.HeadState()
	if err == db.ErrKeyNotFound {
		return &felt.Zero, nil
	}
	if err != nil {
		return nil, err
	}

	trie, closer2, err := state.(core.StateReaderStorage).StorageTrie()
	if err != nil {
		return nil, err
	}

	root, err := trie.Root()
	if err != nil {
		return nil, err
	}

	closer2()
	close()

	return root, nil
}

func (n *noopTrie) SetClasss(path *felt.Felt, classHash *felt.Felt, class core.Class) error {
	fmt.Printf("Class %s %s %s\n", path.String(), classHash.String(), class)
	return nil
}

func (n *noopTrie) SetAddress(paths []*felt.Felt, nodeHashes []*felt.Felt, classHashes []*felt.Felt, nonces []*felt.Felt) error {
	n.addrCount++
	return n.blockchain.StoreDirect(paths, classHashes, nodeHashes, nonces)
}

func (n *noopTrie) SetStorage(storagePath *felt.Felt, path []*felt.Felt, value []*felt.Felt) error {
	return n.blockchain.StoreStorageDirect(storagePath, path, value)
}

var _ MutableStorage = &noopTrie{}
