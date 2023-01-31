package blockchain

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata/gateway"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/mainnet_block_0.json
	mainnetBlock0 []byte
	//go:embed testdata/mainnet_state_update_0.json
	mainnetStateUpdate0 []byte
	//go:embed testdata/mainnet_block_1.json
	mainnetBlock1 []byte
	//go:embed testdata/mainnet_state_update_1.json
	mainnetStateUpdate1 []byte
)

func TestNewBlockchain(t *testing.T) {
	t.Run("empty blockchain's head is nil", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		assert.Equal(t, utils.MAINNET, chain.network)
		b, err := chain.Head()
		assert.Nil(t, b)
		assert.EqualError(t, err, db.ErrKeyNotFound.Error())
	})
	t.Run("non-empty blockchain gets head from db", func(t *testing.T) {
		clientBlock0, clientStateUpdate0 := new(clients.Block), new(clients.StateUpdate)
		if err := json.Unmarshal(mainnetBlock0, clientBlock0); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(mainnetStateUpdate0, clientStateUpdate0); err != nil {
			t.Fatal(err)
		}
		block0, err := gateway.AdaptBlock(clientBlock0)
		if err != nil {
			t.Fatal(err)
		}
		stateUpdate0, err := gateway.AdaptStateUpdate(clientStateUpdate0)
		if err != nil {
			t.Fatal(err)
		}
		testDB := db.NewTestDb()
		chain := NewBlockchain(testDB, utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		chain = NewBlockchain(testDB, utils.MAINNET)
		b, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, block0, b)
	})
}

func TestHeight(t *testing.T) {
	t.Run("return nil if blockchain is empty", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.GOERLI)
		_, err := chain.Height()
		assert.Error(t, err)
	})
	t.Run("return height of the blockchain's head", func(t *testing.T) {
		clientBlock0, clientStateUpdate0 := new(clients.Block), new(clients.StateUpdate)
		if err := json.Unmarshal(mainnetBlock0, clientBlock0); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(mainnetStateUpdate0, clientStateUpdate0); err != nil {
			t.Fatal(err)
		}
		block0, err := gateway.AdaptBlock(clientBlock0)
		if err != nil {
			t.Fatal(err)
		}
		stateUpdate0, err := gateway.AdaptStateUpdate(clientStateUpdate0)
		if err != nil {
			t.Fatal(err)
		}
		testDB := db.NewTestDb()
		chain := NewBlockchain(testDB, utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		chain = NewBlockchain(testDB, utils.MAINNET)
		height, err := chain.Height()
		assert.NoError(t, err)
		assert.Equal(t, block0.Number, height)
	})
}

func TestVerifyBlock(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	h2, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := NewBlockchain(db.NewTestDb(), utils.UNITTEST)

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Number: 10}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with number more than 0 in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{ParentHash: h1}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with non-zero parent hash in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	headBlock := &core.Block{
		Number:                0,
		ParentHash:            &felt.Zero,
		GlobalStateRoot:       &felt.Zero,
		TransactionCount:      &felt.Zero,
		TransactionCommitment: &felt.Zero,
	}
	headBlock.Hash, err = core.BlockHash(headBlock, utils.UNITTEST)
	assert.NoError(t, err)
	headStateUpdate := &core.StateUpdate{
		BlockHash: headBlock.Hash,
		OldRoot:   &felt.Zero,
		NewRoot:   &felt.Zero,
		StateDiff: new(core.StateDiff),
	}
	assert.NoError(t, chain.Store(headBlock, headStateUpdate))

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			incomingBlock := &core.Block{Number: 10, ParentHash: headBlock.Hash}

			expectedErr := &ErrIncompatibleBlock{
				"block number difference between head and incoming block is not 1",
			}
			assert.Equal(t, chain.VerifyBlock(incomingBlock, nil), expectedErr)
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		incomingBlock := &core.Block{ParentHash: h1, Number: 1}

		expectedErr := &ErrIncompatibleBlock{
			"block's parent hash does not match head block hash",
		}
		assert.Equal(t, chain.VerifyBlock(incomingBlock, nil), expectedErr)
	})

	validNextBlock := *headBlock
	validNextBlock.Number = 1
	validNextBlock.ParentHash = headBlock.Hash
	validNextBlock.Hash, err = core.BlockHash(&validNextBlock, utils.UNITTEST)
	assert.NoError(t, err)

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		stateUpdate := &core.StateUpdate{BlockHash: h1}
		expectedErr := ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
		assert.Equal(t, chain.VerifyBlock(&validNextBlock, stateUpdate), expectedErr)
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			block := validNextBlock
			block.GlobalStateRoot = h1
			stateUpdate := &core.StateUpdate{BlockHash: block.Hash, NewRoot: h2}

			expectedErr := ErrIncompatibleBlockAndStateUpdate{
				"block's GlobalStateRoot does not match state update's NewRoot",
			}
			assert.Equal(t, chain.VerifyBlock(&block, stateUpdate), expectedErr)
		})

	validNextStateUpdate := *headStateUpdate
	validNextStateUpdate.BlockHash = validNextBlock.Hash

	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		wrongHashBlock := validNextBlock
		wrongHashBlock.Hash = h1

		wrongHashStateUpdate := validNextStateUpdate
		wrongHashStateUpdate.BlockHash = wrongHashBlock.Hash

		expectedErr := &ErrIncompatibleBlock{fmt.Sprintf(
			"incorrect block hash: block.Hash = %v and BlockHash(block) = %v",
			wrongHashBlock.Hash.Text(16), validNextBlock.Hash.Text(16))}
		assert.Equal(t, chain.VerifyBlock(&wrongHashBlock, &wrongHashStateUpdate), expectedErr)
	})

	assert.NoError(t, chain.Store(&validNextBlock, &validNextStateUpdate))

	t.Run("no error if block is unverifiable", func(t *testing.T) {
		unverifBlock := validNextBlock
		unverifBlock.Number = 2
		unverifBlock.ParentHash = validNextBlock.Hash
		unverifBlock.Hash = h1

		unverifStateUpdate := validNextStateUpdate
		unverifStateUpdate.BlockHash = unverifBlock.Hash

		assert.NoError(t, chain.VerifyBlock(&unverifBlock, &unverifStateUpdate))
	})
}

func TestStore(t *testing.T) {
	clientBlock0, clientStateUpdate0 := new(clients.Block), new(clients.StateUpdate)
	if err := json.Unmarshal(mainnetBlock0, clientBlock0); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(mainnetStateUpdate0, clientStateUpdate0); err != nil {
		t.Fatal(err)
	}
	block0, err := gateway.AdaptBlock(clientBlock0)
	if err != nil {
		t.Fatal(err)
	}
	stateUpdate0, err := gateway.AdaptStateUpdate(clientStateUpdate0)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("add block to empty blockchain", func(t *testing.T) {
		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, headBlock, block0)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()
		root, err := state.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate0.NewRoot, root)

		gotHeadBlock, err := chain.Head()
		assert.NoError(t, err)
		got0Block, err := chain.GetBlockByNumber(0)
		assert.NoError(t, err)
		assert.Equal(t, gotHeadBlock, got0Block)
	})
	t.Run("add block to non-empty blockchain", func(t *testing.T) {
		clientBlock1, clientStateUpdate1 := new(clients.Block), new(clients.StateUpdate)
		if err := json.Unmarshal(mainnetBlock1, clientBlock1); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(mainnetStateUpdate1, clientStateUpdate1); err != nil {
			t.Fatal(err)
		}
		block1, err := gateway.AdaptBlock(clientBlock1)
		if err != nil {
			t.Fatal(err)
		}
		stateUpdate1, err := gateway.AdaptStateUpdate(clientStateUpdate1)
		if err != nil {
			t.Fatal(err)
		}

		chain := NewBlockchain(db.NewTestDb(), utils.MAINNET)
		assert.NoError(t, chain.Store(block0, stateUpdate0))
		assert.NoError(t, chain.Store(block1, stateUpdate1))

		headBlock, err := chain.Head()
		assert.NoError(t, err)
		assert.Equal(t, headBlock, block1)

		txn := chain.database.NewTransaction(false)
		defer txn.Discard()
		root, err := state.NewState(txn).Root()
		assert.NoError(t, err)
		assert.Equal(t, stateUpdate1.NewRoot, root)

		gotHeadBlock, err := chain.Head()
		assert.NoError(t, err)
		got1Block, err := chain.GetBlockByNumber(1)
		assert.NoError(t, err)
		assert.Equal(t, gotHeadBlock, got1Block)
	})
}
