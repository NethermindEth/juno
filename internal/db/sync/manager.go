package sync

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/golang/protobuf/proto"

	"github.com/NethermindEth/juno/internal/db"
)

var (
	DbError                        = errors.New("database error")
	UnmarshalError                 = errors.New("unmarshal error")
	MarshalError                   = errors.New("marshal error")
	latestBlockSavedKey            = []byte("latestBlockSaved")
	latestBlockSyncKey             = []byte("latestBlockSync")
	blockOfLatestEventProcessedKey = []byte("blockOfLatestEventProcessed")
	latestStateRoot                = []byte("latestStateRoot")
	stateDiffPrefix                = []byte("stateDiff:")
)

// Manager is a Block database manager to save and search the blocks.
type Manager struct {
	database db.Database
}

// NewManager returns a new Block manager using the given database.
func NewManager(database db.Database) *Manager {
	return &Manager{database: database}
}

// StoreLatestBlockSaved stores the latest block sync.
func (m *Manager) StoreLatestBlockSaved(latestBlockSaved int64) {
	// Marshal the latest block sync
	value, err := json.Marshal(latestBlockSaved)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(latestBlockSavedKey, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestBlockSaved returns the latest block sync.
func (m *Manager) GetLatestBlockSaved() int64 {
	// Query to database
	data, err := m.database.Get(latestBlockSavedKey)
	if err != nil {
		// notest
		if db.ErrNotFound == err {
			return 0
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	if data == nil {
		// notest
		return 0
	}
	// Unmarshal the data from database
	latestBlockSaved := new(int64)
	if err := json.Unmarshal(data, latestBlockSaved); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *latestBlockSaved
}

// StoreLatestBlockSync stores the latest block sync.
func (m *Manager) StoreLatestBlockSync(latestBlockSync int64) {
	// Marshal the latest block sync
	value, err := json.Marshal(latestBlockSync)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(latestBlockSyncKey, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestBlockSync returns the latest block sync.
func (m *Manager) GetLatestBlockSync() int64 {
	// Query to database
	data, err := m.database.Get(latestBlockSyncKey)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return 0
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	latestBlockSync := new(int64)
	if err := json.Unmarshal(data, latestBlockSync); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *latestBlockSync
}

// StoreLatestStateRoot stores the latest state root.
func (m *Manager) StoreLatestStateRoot(stateRoot string) {
	// Store the latest state root
	err := m.database.Put(latestStateRoot, []byte(stateRoot))
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetLatestStateRoot returns the latest state root.
func (m *Manager) GetLatestStateRoot() string {
	// Query to database
	data, err := m.database.Get(latestStateRoot)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return ""
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	return string(data)
}

// StoreBlockOfProcessedEvent stores the block of the latest event processed,
func (m *Manager) StoreBlockOfProcessedEvent(starknetFact, l1Block int64) {
	key := []byte(fmt.Sprintf("%s%d", blockOfLatestEventProcessedKey, starknetFact))
	// Marshal the latest block sync
	value, err := json.Marshal(l1Block)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", MarshalError, err)))
	}

	// Store the latest block sync
	err = m.database.Put(key, value)
	if err != nil {
		panic(any(fmt.Errorf("%w: %s", DbError, err.Error())))
	}
}

// GetBlockOfProcessedEvent returns the block of the latest event processed,
func (m *Manager) GetBlockOfProcessedEvent(starknetFact int64) int64 {
	// Query to database
	key := []byte(fmt.Sprintf("%s%d", blockOfLatestEventProcessedKey, starknetFact))
	data, err := m.database.Get(key)
	if err != nil {
		// notest
		if errors.Is(err, db.ErrNotFound) {
			return 0
		}
		panic(any(fmt.Errorf("%w: %s", DbError, err)))
	}
	// Unmarshal the data from database
	blockSync := new(int64)
	if err := json.Unmarshal(data, blockSync); err != nil {
		// notest
		panic(any(fmt.Errorf("%w: %s", UnmarshalError, err.Error())))
	}
	return *blockSync
}

// StoreStateDiff stores the state diff for the given block.
func (m *Manager) StoreStateDiff(stateDiff *types.StateDiff, blockHash *felt.Felt) error {
	data, err := marshalStateUpdate(stateDiff)
	if err != nil {
		return err
	}
	return m.database.Put(stateDbKey(blockHash), data)
}

func (m *Manager) GetStateUpdate(blockHash *felt.Felt) (*types.StateDiff, error) {
	data, err := m.database.Get(stateDbKey(blockHash))
	if err != nil {
		return nil, err
	}
	return unmarshalStateUpdate(data)
}

// Close closes the Manager.
func (m *Manager) Close() {
	m.database.Close()
}

func stateDbKey(blockHash *felt.Felt) []byte {
	return append(stateDiffPrefix, blockHash.ByteSlice()...)
}

func marshalStateUpdate(s *types.StateDiff) ([]byte, error) {
	fmt.Println("marshalStateUpdate")
	stateUpdateProto := &StateUpdate{
		BlockHash:         s.BlockHash.ByteSlice(),
		NewRoot:           s.NewRoot.ByteSlice(),
		OldRoot:           s.OldRoot.ByteSlice(),
		StorageDiffs:      make(map[string]*StorageDiffs, 0),
		DeclaredContracts: make([][]byte, len(s.DeclaredContracts)),
		DeployedContracts: make([]*DeployedContract, len(s.DeployedContracts)),
		Nonces:            make([]*Nonce, 0),
	}
	for address, diffs := range s.StorageDiff {
		diffsProto := make([]*StorageDiff, len(diffs))
		for i, diff := range diffs {
			diffsProto[i] = &StorageDiff{
				Key:   diff.Address.ByteSlice(),
				Value: diff.Value.ByteSlice(),
			}
		}
		fmt.Printf("Address: %s\n", address.Hex())
		stateUpdateProto.StorageDiffs[address.Hex()] = &StorageDiffs{
			Diffs: diffsProto,
		}
	}
	for i, deployedContract := range s.DeployedContracts {
		deployedProto := &DeployedContract{
			Address:   deployedContract.Address.ByteSlice(),
			ClassHash: deployedContract.Hash.ByteSlice(),
		}
		stateUpdateProto.DeployedContracts[i] = deployedProto
	}
	for i, contractAddress := range s.DeclaredContracts {
		stateUpdateProto.DeclaredContracts[i] = contractAddress.ByteSlice()
	}
	return proto.Marshal(stateUpdateProto)
}

func unmarshalStateUpdate(data []byte) (*types.StateDiff, error) {
	fmt.Println("unmarshalStateUpdate")
	var stateUpdateProto StateUpdate
	if err := proto.Unmarshal(data, &stateUpdateProto); err != nil {
		return nil, err
	}
	stateUpdate := &types.StateDiff{
		BlockHash:         new(felt.Felt).SetBytes(stateUpdateProto.BlockHash),
		NewRoot:           new(felt.Felt).SetBytes(stateUpdateProto.NewRoot),
		OldRoot:           new(felt.Felt).SetBytes(stateUpdateProto.OldRoot),
		StorageDiff:       make(map[felt.Felt][]types.MemoryCell, len(stateUpdateProto.StorageDiffs)),
		DeclaredContracts: make([]*felt.Felt, len(stateUpdateProto.DeclaredContracts)),
		DeployedContracts: make([]types.DeployedContract, len(stateUpdateProto.DeployedContracts)),
	}
	for address, diffsProto := range stateUpdateProto.StorageDiffs {
		diffs := make([]types.MemoryCell, len(diffsProto.Diffs))
		for i, diffProto := range diffsProto.Diffs {
			diffs[i] = types.MemoryCell{
				Address: new(felt.Felt).SetBytes(diffProto.Key),
				Value:   new(felt.Felt).SetBytes(diffProto.Value),
			}
		}
		addressF := new(felt.Felt).SetHex(address)
		stateUpdate.StorageDiff[*addressF] = diffs
	}
	for i, deployedProto := range stateUpdateProto.DeployedContracts {
		deployed := types.DeployedContract{
			Address: new(felt.Felt).SetBytes(deployedProto.Address),
			Hash:    new(felt.Felt).SetBytes(deployedProto.ClassHash),
		}
		stateUpdate.DeployedContracts[i] = deployed
	}
	for i, delcaredContract := range stateUpdateProto.DeclaredContracts {
		stateUpdate.DeclaredContracts[i] = new(felt.Felt).SetBytes(delcaredContract)
	}
	return stateUpdate, nil
}
