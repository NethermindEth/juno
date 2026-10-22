package felt

// Phantom kinds: each alias below is a distinct type with the full Typed method set.
type (
	addressKind         struct{}
	hashKind            struct{}
	classHashKind       struct{}
	sierraClassHashKind struct{}
	casmClassHashKind   struct{}
	transactionHashKind struct{}
	stateRootHashKind   struct{}
)

type (
	Address         = Typed[addressKind]
	Hash            = Typed[hashKind]
	ClassHash       = Typed[classHashKind]
	SierraClassHash = Typed[sierraClassHashKind]
	CasmClassHash   = Typed[casmClassHashKind]
	TransactionHash = Typed[transactionHashKind]
	StateRootHash   = Typed[stateRootHashKind]
)
