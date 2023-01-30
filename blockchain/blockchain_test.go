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
	"github.com/fxamacker/cbor/v2"
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
		assert.Nil(t, chain.Height())
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
		assert.Equal(t, block0.Number, *chain.Height())
	})
}

func TestBlockDbKey(t *testing.T) {
	bytes := [32]byte{}
	for i := 0; i < 32; i++ {
		bytes[i] = byte(i + 1)
	}
	key := &BlockDbKey{
		Number: 44,
		Hash:   new(felt.Felt).SetBytes(bytes[:]),
	}

	keyB, err := key.MarshalBinary()
	expectedKeyB := []byte{
		byte(db.Blocks), 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 44, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9,
		0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c,
		0x1d, 0x1e, 0x1f, 0x20,
	}
	assert.Equal(t, expectedKeyB, keyB)
	assert.NoError(t, err)
	keyUnmarshaled := new(BlockDbKey)
	require.NoError(t, keyUnmarshaled.UnmarshalBinary(keyB))
	assert.Equal(t, key, keyUnmarshaled)

	keyB[0] = byte(db.State)
	assert.EqualError(t, keyUnmarshaled.UnmarshalBinary(keyB), "wrong prefix")
	keyB = append(keyB, 0)
	assert.EqualError(t, keyUnmarshaled.UnmarshalBinary(keyB), "key should be 41 bytes long")
}

func TestVerifyBlock(t *testing.T) {
	h1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	h2, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	h3, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	sr1, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	sr2, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	chain := NewBlockchain(db.NewTestDb(), utils.GOERLI)

	t.Run("error if chain is empty and incoming block number is not 0", func(t *testing.T) {
		block := &core.Block{Number: 10}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with number more than 0 in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	t.Run("error if chain is empty and incoming block parent's hash is not 0", func(t *testing.T) {
		block := &core.Block{ParentHash: h2}
		expectedErr := &ErrIncompatibleBlock{"cannot insert a block with non-zero parent hash in an empty blockchain"}
		assert.EqualError(t, chain.VerifyBlock(block, nil), expectedErr.Error())
	})

	t.Run("error if difference between incoming block number and head is not 1",
		func(t *testing.T) {
			headBlock := &core.Block{Number: 2, Hash: h1}
			incomingBlock := &core.Block{Number: 10, ParentHash: h2}

			assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
				blockBinary, err := cbor.Marshal(headBlock)
				if err != nil {
					return err
				}
				return txn.Set(db.HeadBlock.Key(), blockBinary)
			}))
			expectedErr := &ErrIncompatibleBlock{
				"block number difference between head and incoming block is not 1",
			}
			assert.EqualError(t, chain.VerifyBlock(incomingBlock, nil), expectedErr.Error())
		})

	t.Run("error when head hash does not match incoming block's parent hash", func(t *testing.T) {
		headBlock := &core.Block{Hash: h1, Number: 1}
		incomingBlock := &core.Block{ParentHash: h2, Number: 2}
		assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
			blockBinary, err := cbor.Marshal(headBlock)
			if err != nil {
				return err
			}
			return txn.Set(db.HeadBlock.Key(), blockBinary)
		}))
		expectedErr := &ErrIncompatibleBlock{
			"block's parent hash does not match head block hash",
		}
		assert.EqualError(t, chain.VerifyBlock(incomingBlock, nil), expectedErr.Error())
	})

	t.Run("error when block hash does not match state update's block hash", func(t *testing.T) {
		headBlock := &core.Block{Hash: h1}
		block := &core.Block{Number: 1, ParentHash: h1, Hash: h2}
		stateUpdate := &core.StateUpdate{BlockHash: h3}
		assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
			blockBinary, err := cbor.Marshal(headBlock)
			if err != nil {
				return err
			}
			return txn.Set(db.HeadBlock.Key(), blockBinary)
		}))
		expectedErr := ErrIncompatibleBlockAndStateUpdate{"block hashes do not match"}
		assert.EqualError(t, chain.VerifyBlock(block, stateUpdate), expectedErr.Error())
	})

	t.Run("error when block global state root does not match state update's new root",
		func(t *testing.T) {
			headBlock := &core.Block{Hash: h1}
			block := &core.Block{Number: 1, ParentHash: h1, Hash: h2, GlobalStateRoot: sr1}
			stateUpdate := &core.StateUpdate{BlockHash: h2, NewRoot: sr2}
			assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
				blockBinary, err := cbor.Marshal(headBlock)
				if err != nil {
					return err
				}
				return txn.Set(db.HeadBlock.Key(), blockBinary)
			}))
			expectedErr := ErrIncompatibleBlockAndStateUpdate{
				"block's GlobalStateRoot does not match state update's NewRoot",
			}
			assert.EqualError(t, chain.VerifyBlock(block, stateUpdate), expectedErr.Error())
		})
	t.Run("no error if block is unverifiable", func(t *testing.T) {
		headBlock := &core.Block{Number: 119801, Hash: h1}
		block := &core.Block{Number: 119802, ParentHash: h1, Hash: h2, GlobalStateRoot: sr1}
		stateUpdate := &core.StateUpdate{BlockHash: h2, NewRoot: sr1}
		assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
			blockBinary, err := cbor.Marshal(headBlock)
			if err != nil {
				return err
			}
			return txn.Set(db.HeadBlock.Key(), blockBinary)
		}))
		assert.NoError(t, chain.VerifyBlock(block, stateUpdate))
	})
	t.Run("error if block hash has not being calculated properly", func(t *testing.T) {
		headBlock := &core.Block{Number: 999, Hash: h1}
		block := &core.Block{
			Hash:                  h2,
			Number:                1000,
			ParentHash:            h1,
			TransactionCount:      new(felt.Felt).SetUint64(10),
			TransactionCommitment: new(felt.Felt).SetUint64(1009485),
			GlobalStateRoot:       sr1,
		}
		stateUpdate := &core.StateUpdate{BlockHash: h2, NewRoot: sr1}
		assert.NoError(t, chain.database.Update(func(txn db.Transaction) error {
			blockBinary, err := cbor.Marshal(headBlock)
			if err != nil {
				return err
			}
			return txn.Set(db.HeadBlock.Key(), blockBinary)
		}))
		h, err := core.BlockHash(block, utils.GOERLI)
		assert.NoError(t, err)
		expectedErr := &ErrIncompatibleBlock{fmt.Sprintf(
			"incorrect block hash: block.Hash = %v and BlockHash(block) = %v",
			block.Hash.Text(16), h.Text(16))}
		assert.EqualError(t, chain.VerifyBlock(block, stateUpdate), expectedErr.Error())
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

		assert.NoError(t, chain.database.View(func(txn db.Transaction) error {
			databaseHeadBlock, err := txn.Get(db.HeadBlock.Key())
			if err != nil {
				return err
			}

			headBlock := new(core.Block)
			if err = cbor.Unmarshal(databaseHeadBlock, headBlock); err != nil {
				return err
			}
			assert.Equal(t, headBlock, block0)

			block0Key := &BlockDbKey{block0.Number, block0.Hash}
			k, err := block0Key.MarshalBinary()
			if err != nil {
				return err
			}

			databaseBlock0, err := txn.Get(k)
			if err != nil {
				return err
			}

			block := new(core.Block)
			if err = cbor.Unmarshal(databaseBlock0, block); err != nil {
				return err
			}
			assert.Equal(t, block, block0)
			return nil
		}))
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

		assert.NoError(t, chain.database.View(func(txn db.Transaction) error {
			databaseHeadBlock, err := txn.Get(db.HeadBlock.Key())
			if err != nil {
				return err
			}

			headBlock := new(core.Block)
			if err = cbor.Unmarshal(databaseHeadBlock, headBlock); err != nil {
				return err
			}
			assert.Equal(t, headBlock, block1)

			block1Key := &BlockDbKey{block1.Number, block1.Hash}
			k, err := block1Key.MarshalBinary()
			if err != nil {
				return err
			}

			databaseBlock0, err := txn.Get(k)
			if err != nil {
				return err
			}

			block := new(core.Block)
			if err = cbor.Unmarshal(databaseBlock0, block); err != nil {
				return err
			}
			assert.Equal(t, block, block1)
			return nil
		}))
	})
}
