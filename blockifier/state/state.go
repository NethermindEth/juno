package state

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

var ONE = new(felt.Felt).SetUint64(1)

// TransactionalState enables transactional state operations.
type TransactionalState struct {
	*CachedState
	stateCommitter StateCommitter
}

// NewTransactionalState creates a new TransactionalState.
func NewTransactionalState(
	cachedState *CachedState,
	stateCommitter StateCommitter,
) TransactionalState {
	return TransactionalState{
		CachedState:    cachedState,
		stateCommitter: stateCommitter,
	}
}

// Commit commits all changes to the underlying state.
func (ts *TransactionalState) Commit() error {
	if err := ts.stateCommitter.ApplyWritesToState(&ts.writes, ts.classHashToClass); err != nil {
		return err
	}
	return ts.stateCommitter.Commit()
}

// Abort aborts the transaction.
func (ts *TransactionalState) Abort() {
	ts.stateCommitter.Abort()
}

// Note: the read and write functions are inlined because we expect
// them to be called many times throughout execution.
// This isn't the case for StateCommitter.

// StateReader contains all reader function types
type StateReader struct {
	GetClassHashFn func(addr felt.Felt) (felt.Felt, error)
	GetNonceFn     func(addr felt.Felt) (felt.Felt, error)
	GetStorageFn   func(addr, key felt.Felt) (felt.Felt, error)
	// GetCoreClassFn       func(classHash felt.Felt) (core.Class, error)    // Todo: this uses a pesky interface, check if we can use GetClassFn below
	// GetV0ClassFn         func(classHash felt.Felt) (core.Cairo0Class, error) // Todo: implement?
	// GetV1ClassFn         func(classHash felt.Felt) (core.Cairo1Class, error) // Todo: implement?
	GetClassFn           func(classHash felt.Felt) (ClassBox, error)
	GetFeeTokenBalanceFn func(contractAddress, feeTokenAddress felt.Felt) (felt.Felt, error)
}

type ClassBoxType int

const (
	ClassBoxTypeV0 ClassBoxType = iota
	ClassBoxTypeV1
)

// We use ClassBox to constrain the set of "class" types this pkg can use. It allows us to avoid using interfaces throughout this pkg.
// We want to avoid interfaces for a variety of reasons. This type should have either V0 or V1.
type ClassBox struct {
	V0      core.Cairo0Class
	V1      core.Cairo1Class
	Version ClassBoxType
}

// StateWriter contains all writer function types
type StateWriter struct {
	SetStorageFn           func(contractAddress felt.Felt, key felt.Felt, value felt.Felt) error
	IncrementNonceFn       func(contractAddress felt.Felt) error
	SetClassHashFn         func(contractAddress felt.Felt, classHash felt.Felt) error
	SetContractClassFn     func(classHash felt.Felt, contractClass core.Class) error       // Todo: we use interfaces here. Can we replace with functions that take value types?
	SetContractV0ClassFn   func(classHash felt.Felt, contractClass core.Cairo0Class) error // Todo: implement
	SetContractV1ClassFn   func(classHash felt.Felt, contractClass core.Cairo1Class) error // Todo: implement
	SetCompiledClassHashFn func(classHash felt.Felt, compiledClassHash felt.Felt) error
}

// StateCommitter
type StateCommitter interface {
	ApplyWritesToState(writes *StateMaps, classHashToClass map[felt.Felt]core.Class) error
	Commit() error
	Abort()
}

// CachedState manages state with caching capabilities.
type CachedState struct {
	StateReader
	cachedReads      StateMaps
	writes           StateMaps
	classHashToClass map[felt.Felt]core.Class
}

// NewCachedState creates a new CachedState.
func NewCachedState(stateReader StateReader) *CachedState {
	return &CachedState{
		StateReader:      stateReader,
		cachedReads:      NewStateMaps(),
		writes:           NewStateMaps(),
		classHashToClass: make(map[felt.Felt]core.Class, 32),
	}
}

// GetClassHash gets a class hash for an address with caching.
//
//go:inline
func (cs *CachedState) GetClassHash(addr felt.Felt) (felt.Felt, error) {
	// First check writes
	classHash, exists := cs.writes.GetClassHash(addr)
	if exists {
		return classHash, nil
	}

	// Then check cached reads
	classHash, exists = cs.cachedReads.GetClassHash(addr)
	if exists {
		return classHash, nil
	}

	// Direct function call - no interface dispatch
	classHash, err := cs.GetClassHashFn(addr)
	if err != nil {
		return felt.Felt{}, err
	}

	// Cache the result
	cs.cachedReads.SetClassHash(addr, classHash)

	return classHash, nil
}

// GetNonce gets a nonce for an address with caching.
//
//go:inline
func (cs *CachedState) GetNonce(addr felt.Felt) (felt.Felt, error) {
	// First check writes
	nonce, exists := cs.writes.GetNonce(addr)
	if exists {
		return nonce, nil
	}

	// Then check cached reads
	nonce, exists = cs.cachedReads.GetNonce(addr)
	if exists {
		return nonce, nil
	}

	// Direct function call - no interface dispatch
	nonce, err := cs.GetNonceFn(addr)
	if err != nil {
		return felt.Felt{}, err
	}

	// Cache the result
	cs.cachedReads.SetNonce(addr, nonce)

	return nonce, nil
}

// GetStorage gets a storage value with caching.
//
//go:inline
func (cs *CachedState) GetStorage(addr, key felt.Felt) (felt.Felt, error) {
	entry := StorageEntry{ContractAddress: addr, Key: key}

	// First check writes
	value, exists := cs.writes.GetStorage(entry)
	if exists {
		return value, nil
	}

	// Then check cached reads
	value, exists = cs.cachedReads.GetStorage(entry)
	if exists {
		return value, nil
	}

	// Direct function call - no interface dispatch
	value, err := cs.GetStorageFn(addr, key)
	if err != nil {
		return felt.Felt{}, err
	}

	// Cache the result
	cs.cachedReads.SetStorage(entry, value)

	return value, nil
}

// Todo: fix, using Box type broke this
// Class gets a class by its hash with caching.
func (cs *CachedState) Class(classHash felt.Felt) (core.Class, error) {
	// Check class cache first - need to keep our own mutex for this map
	// class, exists := cs.classHashToClass[classHash]
	// if exists {
	// 	return class, nil
	// }

	// // Direct function call - no interface dispatch
	// class, err := cs.GetClassFn(classHash)
	// if err != nil {
	// 	return nil, err
	// }

	// // Cache the result
	// cs.classHashToClass[classHash] = class

	return nil, nil
}

// GetFeeTokenBalance gets the fee token balance.
//
//go:inline
func (cs *CachedState) GetFeeTokenBalance(contractAddress, feeTokenAddress felt.Felt) (felt.Felt, error) {
	// Direct function call - no interface dispatch
	return cs.GetFeeTokenBalanceFn(contractAddress, feeTokenAddress)
}

// SetStorage sets a storage value.
//
//go:inline
func (cs *CachedState) SetStorage(contractAddress felt.Felt, key felt.Felt, value felt.Felt) error {
	entry := StorageEntry{ContractAddress: contractAddress, Key: key}
	cs.writes.SetStorage(entry, value)
	return nil
}

// IncrementNonce increments a contract's nonce.
//
//go:inline
func (cs *CachedState) IncrementNonce(contractAddress felt.Felt) error {
	nonce, err := cs.GetNonce(contractAddress)
	if err != nil {
		return err
	}

	cs.writes.SetNonce(contractAddress, *nonce.Add(&nonce, ONE))
	return nil
}

// SetClassHash sets a class hash for a contract.
//
//go:inline
func (cs *CachedState) SetClassHash(contractAddress felt.Felt, classHash felt.Felt) error {
	cs.writes.SetClassHash(contractAddress, classHash)
	return nil
}

// SetContractClass sets a contract class.
//
//go:inline
func (cs *CachedState) SetContractClass(classHash felt.Felt, contractClass core.Class) error {
	// This still needs direct map access as StateMaps doesn't have this method
	cs.classHashToClass[classHash] = contractClass
	return nil
}

// SetCompiledClassHash sets a compiled class hash.
//
//go:inline
func (cs *CachedState) SetCompiledClassHash(classHash felt.Felt, compiledClassHash felt.Felt) error {
	cs.writes.SetCompiledClassHash(classHash, compiledClassHash)
	return nil
}
