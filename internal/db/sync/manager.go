package sync

import (
	"encoding/binary"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	dbBlock "github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

const (
	latestBlockNumberKey string = "LatestBlockNumber"
)

// Manager is a Sync database manager to save and search the blocks.
type Manager struct {
	database db.DatabaseTransactional
}

// NewManager returns a new Sync manager using the given database.
func NewManager(database db.DatabaseTransactional) *Manager {
	return &Manager{database: database}
}

func (manager *Manager) GetLatestBlockNumber() (uint64, error) {
	blockHashBytes, err := manager.database.Get([]byte(latestBlockNumberKey))
	if err != nil {
		return 0, err
	}
	blockHash := binary.LittleEndian.Uint64(blockHashBytes)
	return blockHash, nil
}

func (manager *Manager) SetLatestBlockNumber(blockNumber uint64) error {
	var blockNumBytes [8]byte
	binary.LittleEndian.PutUint64(blockNumBytes[:], blockNumber)
	return manager.database.Put([]byte(latestBlockNumberKey), blockNumBytes[:])
}

func (manager *Manager) UpdateState(update types.StateUpdate, contractHashMap map[string]*big.Int) error {
	return manager.database.RunTxn(func(txn db.DatabaseOperations) error {
		_, err := updateState(txn, update, contractHashMap)
		return err
	})
}

func updateState(txn db.DatabaseOperations, update types.StateUpdate, contractHashMap map[string]*big.Int) (*string, error) {
	stateTrie := newTrie(txn, "state_trie_")

	for _, deployedContract := range update.StateDiff.DeployedContracts {
		contractHash, ok := new(big.Int).SetString(remove0x(deployedContract.Hash), 16)
		if !ok {
			// notest
			log.Default.Panic("Couldn't get contract hash")
		}
		storageTrie := newTrie(txn, remove0x(deployedContract.Address))
		storageRoot := storageTrie.Commitment()
		address, ok := new(big.Int).SetString(remove0x(deployedContract.Address), 16)
		if !ok {
			// notest
			log.Default.With("Address", deployedContract.Address).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractStateValue := contractState(contractHash, storageRoot)
		stateTrie.Put(address, contractStateValue)
	}

	for k, v := range update.StateDiff.StorageDiffs {
		formattedAddress := remove0x(k)
		storageTrie := newTrie(txn, formattedAddress)
		for _, storageSlots := range v {
			key, ok := new(big.Int).SetString(remove0x(storageSlots.Address), 16)
			if !ok {
				// notest
				log.Default.With("Storage Slot Key", storageSlots.Address).
					Panic("Couldn't get the ")
			}
			val, ok := new(big.Int).SetString(remove0x(storageSlots.Value), 16)
			if !ok {
				// notest
				log.Default.With("Storage Slot Value", storageSlots.Value).
					Panic("Couldn't get the contract Hash")
			}
			storageTrie.Put(key, val)
		}
		storageRoot := storageTrie.Commitment()

		address, ok := new(big.Int).SetString(formattedAddress, 16)
		if !ok {
			// notest
			log.Default.With("Address", formattedAddress).
				Panic("Couldn't convert Address to Big.Int ")
		}
		contractHash := contractHashMap[formattedAddress]
		contractStateValue := contractState(contractHash, storageRoot)

		stateTrie.Put(address, contractStateValue)
	}

	stateCommitment := remove0x(stateTrie.Commitment().Text(16))

	if update.NewRoot != "" && stateCommitment != remove0x(update.NewRoot) {
		// notest
		log.Default.With("State Commitment", stateCommitment, "State Root from API", remove0x(update.NewRoot)).
			Panic("stateRoot not equal to the one provided")
	}
	log.Default.With("State Root", stateCommitment).Info("Got State commitment")

	return &stateCommitment, nil
}

func (manager *Manager) Close() {
	manager.database.Close()
}

func marshalBlock(block *types.Block) ([]byte, error) {
	protoBlock := dbBlock.Block{
		Hash:             block.BlockHash.Bytes(),
		BlockNumber:      block.BlockNumber,
		ParentBlockHash:  block.ParentHash.Bytes(),
		Status:           block.Status.String(),
		SequencerAddress: block.Sequencer.Bytes(),
		GlobalStateRoot:  block.NewRoot.Bytes(),
		OldRoot:          block.OldRoot.Bytes(),
		AcceptedTime:     block.AcceptedTime,
		TimeStamp:        block.TimeStamp,
		TxCount:          block.TxCount,
		TxCommitment:     block.TxCommitment.Bytes(),
		TxHashes:         marshalBlockTxHashes(block.TxHashes),
		EventCount:       block.EventCount,
		EventCommitment:  block.EventCommitment.Bytes(),
	}
	return proto.Marshal(&protoBlock)
}

func marshalBlockTxHashes(txHashes []types.TransactionHash) [][]byte {
	out := make([][]byte, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = txHash.Bytes()
	}
	return out
}

func unmarshalBlock(data []byte) (*types.Block, error) {
	var protoBlock dbBlock.Block
	err := proto.Unmarshal(data, &protoBlock)
	if err != nil {
		return nil, err
	}
	block := types.Block{
		BlockHash:       types.BytesToBlockHash(protoBlock.Hash),
		ParentHash:      types.BytesToBlockHash(protoBlock.ParentBlockHash),
		BlockNumber:     protoBlock.BlockNumber,
		Status:          types.StringToBlockStatus(protoBlock.Status),
		Sequencer:       types.BytesToAddress(protoBlock.SequencerAddress),
		NewRoot:         types.BytesToFelt(protoBlock.GlobalStateRoot),
		OldRoot:         types.BytesToFelt(protoBlock.OldRoot),
		AcceptedTime:    protoBlock.AcceptedTime,
		TimeStamp:       protoBlock.TimeStamp,
		TxCount:         protoBlock.TxCount,
		TxCommitment:    types.BytesToFelt(protoBlock.TxCommitment),
		TxHashes:        unmarshalBlockTxHashes(protoBlock.TxHashes),
		EventCount:      protoBlock.EventCount,
		EventCommitment: types.BytesToFelt(protoBlock.EventCommitment),
	}
	return &block, nil
}

func unmarshalBlockTxHashes(txHashes [][]byte) []types.TransactionHash {
	out := make([]types.TransactionHash, len(txHashes))
	for i, txHash := range txHashes {
		out[i] = types.BytesToTransactionHash(txHash)
	}
	return out
}

// removeOx remove the initial zeros and x at the beginning of the string
func remove0x(s string) string {
	answer := ""
	found := false
	for _, char := range s {
		found = found || (char != '0' && char != 'x')
		if found {
			answer = answer + string(char)
		}
	}
	if len(answer) == 0 {
		return "0"
	}
	return answer
}

// newTrie returns a new Trie
func newTrie(database db.DatabaseOperations, prefix string) trie.Trie {
	store := db.NewKeyValueStore(database, prefix)
	return trie.New(store, 251)
}

// contractState define the function that calculates the values stored in the
// leaf of the Merkle Patricia Tree that represent the State in StarkNet
func contractState(contractHash, storageRoot *big.Int) *big.Int {
	// Is defined as:
	// h(h(h(contract_hash, storage_root), 0), 0).
	val := pedersen.Digest(contractHash, storageRoot)
	val = pedersen.Digest(val, big.NewInt(0))
	val = pedersen.Digest(val, big.NewInt(0))
	return val
}
