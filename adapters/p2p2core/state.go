package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptStateDiff(s *spec.StateDiff, classes []*spec.Class) *core.StateDiff {
	/**

	Address    *Address               `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Nonce      *Felt252               `protobuf:"bytes,2,opt,name=nonce,proto3,oneof" json:"nonce,omitempty"`                              // Present only if the nonce was updated
	ClassHash  *Hash                  `protobuf:"bytes,3,opt,name=class_hash,json=classHash,proto3,oneof" json:"class_hash,omitempty"`     // Present only if the contract was deployed or replaced in this block.
	IsReplaced *bool                  `protobuf:"varint,4,opt,name=is_replaced,json=isReplaced,proto3,oneof" json:"is_replaced,omitempty"` // Present only if the contract was deployed or replaced, in order to determine whether the contract was deployed or replaced.
	Values     []*ContractStoredValue `protobuf:"bytes,5,rep,name=values,proto3" json:"values,omitempty"`
	Domain     uint32
	*/

	var (
		declaredV0Classes []*felt.Felt
		declaredV1Classes = make(map[felt.Felt]*felt.Felt)
	)

	for _, class := range utils.Map(classes, AdaptClass) {
		h, err := class.Hash()
		if err != nil {
			panic(fmt.Errorf("unexpected error: %v when calculating class hash", err))
		}
		switch c := class.(type) {
		case *core.Cairo0Class:
			declaredV0Classes = append(declaredV0Classes, h)
		case *core.Cairo1Class:
			declaredV1Classes[*h] = c.Compiled.Hash()
		}
	}

	// addr -> {key -> value, ...}
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	nonces := make(map[felt.Felt]*felt.Felt)
	for _, diff := range s.ContractDiffs {
		address := AdaptAddress(diff.Address)

		if diff.Nonce != nil {
			nonces[*address] = AdaptFelt(diff.Nonce)
		}
		storageDiffs[*address] = utils.ToMap(diff.Values, adaptStoredValue)
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: utils.ToMap(s.DeployedContracts, adaptAddrToClassHash),
		DeclaredV0Classes: declaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   utils.ToMap(s.ReplacedClasses, adaptAddrToClassHash),
	}
}

func adaptStoredValue(v *spec.ContractStoredValue) (felt.Felt, *felt.Felt) {
	return *AdaptFelt(v.Key), AdaptFelt(v.Value)
}

// todo rename
func adaptAddrToClassHash(v *spec.StateDiff_ContractAddrToClassHash) (felt.Felt, *felt.Felt) {
	return *AdaptAddress(v.ContractAddr), AdaptHash(v.ClassHash)
}
