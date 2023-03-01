package core

import "github.com/NethermindEth/juno/core/felt"

type StateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt
	StateDiff *StateDiff
}

type StateDiff struct {
	StorageDiffs      map[felt.Felt][]StorageDiff
	Nonces            map[felt.Felt]*felt.Felt
	DeployedContracts []DeployedContract
	DeclaredV0Classes []*felt.Felt
	DeclaredV1Classes []DeclaredV1Class
	ReplacedClasses   []ReplacedClass
}

type StorageDiff struct {
	Key   *felt.Felt
	Value *felt.Felt
}

type DeployedContract struct {
	Address   *felt.Felt
	ClassHash *felt.Felt
}

type DeclaredV1Class struct {
	ClassHash         *felt.Felt
	CompiledClassHash *felt.Felt
}

type ReplacedClass struct {
	Address   *felt.Felt
	ClassHash *felt.Felt
}
