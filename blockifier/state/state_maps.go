package state

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	// Todo: monitor typical map size and set heuristically
	DefaultNonceMapCapacity             = 100
	DefaultClassHashMapCapacity         = 100
	DefaultStorageMapCapacity           = 100
	DefaultCompiledClassHashMapCapacity = 100
	DefaultDeclaredContractMapCapacity  = 10
)

// StorageEntry represents a key-value pair in contract storage.
type StorageEntry struct {
	ContractAddress felt.Felt
	Key             felt.Felt
}

// Todo: sync.Maps
// (Big) Todo: think about this more. Can we remove locks for non-parallel execution?
type StateMaps struct {
	Nonces   map[felt.Felt]felt.Felt
	noncesMu sync.RWMutex

	ClassHashes   map[felt.Felt]felt.Felt
	classHashesMu sync.RWMutex

	Storage   map[StorageEntry]felt.Felt
	storageMu sync.RWMutex

	CompiledClassHashes   map[felt.Felt]felt.Felt
	compiledClassHashesMu sync.RWMutex

	DeclaredContracts   map[felt.Felt]bool
	declaredContractsMu sync.RWMutex
}

// NewStateMaps creates a new instance of StateMaps with initialized maps.
// (Big) Todo: Should we use a custom / non-map struct? This can likely be improved upon.
func NewStateMaps() StateMaps {
	return StateMaps{
		Nonces:              make(map[felt.Felt]felt.Felt, DefaultNonceMapCapacity),
		ClassHashes:         make(map[felt.Felt]felt.Felt, DefaultClassHashMapCapacity),
		Storage:             make(map[StorageEntry]felt.Felt, DefaultStorageMapCapacity),
		CompiledClassHashes: make(map[felt.Felt]felt.Felt, DefaultCompiledClassHashMapCapacity),
		DeclaredContracts:   make(map[felt.Felt]bool, DefaultDeclaredContractMapCapacity),
	}
}

// GetNonce retrieves the nonce for a given contract address.
func (sm *StateMaps) GetNonce(contractAddress felt.Felt) (felt.Felt, bool) {
	sm.noncesMu.RLock()
	defer sm.noncesMu.RUnlock()
	nonce, exists := sm.Nonces[contractAddress]
	return nonce, exists
}

// SetNonce sets the nonce for a given contract address.
func (sm *StateMaps) SetNonce(contractAddress felt.Felt, nonce felt.Felt) {
	sm.noncesMu.Lock()
	defer sm.noncesMu.Unlock()
	sm.Nonces[contractAddress] = nonce
}

// GetClassHash retrieves the class hash for a given contract address.
func (sm *StateMaps) GetClassHash(contractAddress felt.Felt) (felt.Felt, bool) {
	sm.classHashesMu.RLock()
	defer sm.classHashesMu.RUnlock()
	classHash, exists := sm.ClassHashes[contractAddress]
	return classHash, exists
}

// SetClassHash sets the class hash for a given contract address.
func (sm *StateMaps) SetClassHash(contractAddress felt.Felt, classHash felt.Felt) {
	sm.classHashesMu.Lock()
	defer sm.classHashesMu.Unlock()
	sm.ClassHashes[contractAddress] = classHash
}

// GetStorage retrieves the storage value for a given storage entry.
func (sm *StateMaps) GetStorage(entry StorageEntry) (felt.Felt, bool) {
	sm.storageMu.RLock()
	defer sm.storageMu.RUnlock()
	value, exists := sm.Storage[entry]
	return value, exists
}

// Todo: remove setters, and just inline these calls??
// SetStorage sets the storage value for a given storage entry.
func (sm *StateMaps) SetStorage(entry StorageEntry, value felt.Felt) {
	sm.storageMu.Lock()
	defer sm.storageMu.Unlock()
	sm.Storage[entry] = value
}

// GetCompiledClassHash retrieves the compiled class hash for a given class hash.
func (sm *StateMaps) GetCompiledClassHash(classHash felt.Felt) (felt.Felt, bool) {
	sm.compiledClassHashesMu.RLock()
	defer sm.compiledClassHashesMu.RUnlock()
	compiledClassHash, exists := sm.CompiledClassHashes[classHash]
	return compiledClassHash, exists
}

// SetCompiledClassHash sets the compiled class hash for a given class hash.
func (sm *StateMaps) SetCompiledClassHash(classHash felt.Felt, compiledClassHash felt.Felt) {
	sm.compiledClassHashesMu.Lock()
	defer sm.compiledClassHashesMu.Unlock()
	sm.CompiledClassHashes[classHash] = compiledClassHash
}

// IsContractDeclared checks if a contract is declared for a given class hash.
func (sm *StateMaps) IsContractDeclared(classHash felt.Felt) bool {
	sm.declaredContractsMu.RLock()
	defer sm.declaredContractsMu.RUnlock()
	declared, exists := sm.DeclaredContracts[classHash]
	return exists && declared
}

// DeclareContract sets a contract as declared for a given class hash.
func (sm *StateMaps) DeclareContract(classHash felt.Felt) {
	sm.declaredContractsMu.Lock()
	defer sm.declaredContractsMu.Unlock()
	sm.DeclaredContracts[classHash] = true
}
