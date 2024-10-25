package starknet

import (
	"iter"

	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"google.golang.org/protobuf/proto"
)

type SnapProvider interface {
	GetClassRange(request *spec.ClassRangeRequest) (iter.Seq[proto.Message], error)
	GetContractRange(request *spec.ContractRangeRequest) (iter.Seq[proto.Message], error)
	GetStorageRange(request *spec.ContractStorageRequest) (iter.Seq[proto.Message], error)
	GetClasses(request *spec.ClassHashesRequest) (iter.Seq[proto.Message], error)
}
