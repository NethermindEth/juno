package sync

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestSnapCopyTrie(t *testing.T) {
	var d db.DB
	d, _ = pebble.New("/home/amirul/workscratch/smalljuno", utils.NewNopZapLogger())
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	targetdir := "/home/amirul/fastworkscratch3/targetjuno"
	os.RemoveAll(targetdir)

	var d2 db.DB
	d2, _ = pebble.New(targetdir, utils.NewNopZapLogger())
	bc2 := blockchain.New(d2, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	target := &noopTrie{
		blockchain: bc2,
	}

	logger, err := utils.NewZapLogger(utils.DEBUG, false)
	assert.NoError(t, err)

	syncer := SnapSyncher{
		consensus:  &localConsensus{bc},
		snapServer: bc,
		targetTrie: target,
		log:        logger,
	}

	err = syncer.Run(context.Background())
	assert.NoError(t, err)
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

func (n *noopTrie) ApplyStateUpdate(update *core.StateUpdate, validate bool) error {
	return nil
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

type localConsensus struct {
	blockchain *blockchain.Blockchain
}

func (n *localConsensus) GetStateUpdateForBlock(blockNumber uint64) (*core.StateUpdate, error) {
	return n.blockchain.StateUpdateByNumber(blockNumber)
}

func (n *localConsensus) GetCurrentHead() (*core.Header, error) {
	return n.blockchain.HeadsHeader()
}

var _ Consensus = &localConsensus{}
