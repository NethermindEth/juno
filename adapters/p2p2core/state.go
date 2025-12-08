package p2p2core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/class"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/state"
)

func AdaptStateDiff(
	reader core.CommonStateReader,
	contractDiffs []*state.ContractDiff,
	classes []*class.Class,
) (*core.StateDiff, error) {
	var (
		declaredV0Classes []*felt.Felt
		declaredV1Classes = make(map[felt.Felt]*felt.Felt)
	)

	for _, cls := range classes {
		class, err := AdaptClass(cls)
		if err != nil {
			return nil, fmt.Errorf("unsupported class: %w", err)
		}
		h, err := class.Hash()
		if err != nil {
			return nil, fmt.Errorf("unexpected error when calculating class hash: %w", err)
		}
		switch c := class.(type) {
		case *core.DeprecatedCairoClass:
			declaredV0Classes = append(declaredV0Classes, &h)
		case *core.SierraClass:
			casmHash := c.Compiled.Hash(core.HashVersionV1)
			declaredV1Classes[h] = &casmHash
			// todo add type?
		}
	}

	var deployedContracts, replacedClasses []addrToClassHash
	// addr -> {key -> value, ...}
	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	nonces := make(map[felt.Felt]*felt.Felt)
	for _, diff := range contractDiffs {
		address := AdaptAddress(diff.Address)

		if diff.Nonce != nil {
			nonces[*address] = AdaptFelt(diff.Nonce)
		}
		if diff.Values != nil {
			storageDiffs[*address] = utils.ToMap(diff.Values, adaptStoredValue)
		}

		if diff.ClassHash != nil {
			addrToClsHash := addrToClassHash{
				addr:      diff.Address,
				classHash: diff.ClassHash,
			}

			stateClassHash := felt.Zero
			if reader != nil {
				var err error
				stateClassHash, err = reader.ContractClassHash(address)
				if err != nil {
					if !errors.Is(err, db.ErrKeyNotFound) {
						return nil, fmt.Errorf("unexpected error when calculating contract class hash: %w", err)
					}
				}
			}

			if !stateClassHash.IsZero() {
				replacedClasses = append(replacedClasses, addrToClsHash)
			} else {
				deployedContracts = append(deployedContracts, addrToClsHash)
			}
		}
	}

	return &core.StateDiff{
		StorageDiffs:      storageDiffs,
		Nonces:            nonces,
		DeployedContracts: utils.ToMap(deployedContracts, adaptAddrToClassHash),
		DeclaredV0Classes: declaredV0Classes,
		DeclaredV1Classes: declaredV1Classes,
		ReplacedClasses:   utils.ToMap(replacedClasses, adaptAddrToClassHash),
		MigratedClasses:   nil, // todo(rdr): unsure of p2p relationship
	}, nil
}

func adaptStoredValue(v *state.ContractStoredValue) (felt.Felt, *felt.Felt) {
	return *AdaptFelt(v.Key), AdaptFelt(v.Value)
}

type addrToClassHash struct {
	addr      *common.Address
	classHash *common.Hash
}

// todo rename
func adaptAddrToClassHash(v addrToClassHash) (felt.Felt, *felt.Felt) {
	return *AdaptAddress(v.addr), AdaptHash(v.classHash)
}
