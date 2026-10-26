package felt

// Phantom kinds: each alias below is a distinct type with the full Typed method set.
type (
	feltKind            struct{}
	addressKind         struct{}
	hashKind            struct{}
	classHashKind       struct{}
	sierraClassHashKind struct{}
	casmClassHashKind   struct{}
	transactionHashKind struct{}
	stateRootHashKind   struct{}
)

type (
	// Felt is the untyped field element.
	Felt            = Typed[feltKind]
	Address         = Typed[addressKind]
	Hash            = Typed[hashKind]
	ClassHash       = Typed[classHashKind]
	SierraClassHash = Typed[sierraClassHashKind]
	CasmClassHash   = Typed[casmClassHashKind]
	TransactionHash = Typed[transactionHashKind]
	StateRootHash   = Typed[stateRootHashKind]
)
