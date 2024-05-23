package sync

import (
	"context"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

type ContractRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	Range         []*spec.ContractState
	RangeProof    *spec.PatriciaRangeProof
}

type StorageRangeRequest struct {
	StateRoot     *felt.Felt
	ChunkPerProof uint64 // Missing in spec
	Queries       []*spec.StorageRangeQuery
}

type StorageRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	Range         []*spec.ContractStoredValue
	RangeProof    *spec.PatriciaRangeProof
}

type ClassRangeStreamingResult struct {
	ContractsRoot *felt.Felt
	ClassesRoot   *felt.Felt
	Range         *spec.Classes
	RangeProof    *spec.PatriciaRangeProof
}

type SnapServer interface {
	GetContractRange(ctx context.Context, request *spec.ContractRangeRequest) (chan *ContractRangeStreamingResult, error)
	GetStorageRange(ctx context.Context, request *StorageRangeRequest) (chan *StorageRangeStreamingResult, error)
	GetClassRange(ctx context.Context, request *spec.ClassRangeRequest) (chan *ClassRangeStreamingResult, error)
	GetClasses(ctx context.Context, classHashes []*felt.Felt) ([]*spec.Class, error)
}
