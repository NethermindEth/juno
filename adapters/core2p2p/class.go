package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptClass(class core.Class) *spec.Class {
	if class == nil {
		return nil
	}

	hash, err := class.Hash()
	if err != nil {
		panic(fmt.Errorf("failed to hash %t: %w", class, err))
	}

	switch v := class.(type) {
	case *core.Cairo0Class:
		hash, err := v.Hash()
		if err != nil {
			panic(fmt.Errorf("failed to hash Cairo0Class: %w", err))
		}
		AdaptHash(hash)
		return &spec.Class{
			Class: &spec.Class_Cairo0{
				Cairo0: &spec.Cairo0Class{
					Abi:          string(v.Abi),
					Externals:    utils.Map(v.Externals, adaptEntryPoint),
					L1Handlers:   utils.Map(v.L1Handlers, adaptEntryPoint),
					Constructors: utils.Map(v.Constructors, adaptEntryPoint),
					Program:      v.Program,
				},
			},
			Domain:    0, // todo(kirill) recheck
			ClassHash: AdaptHash(hash),
		}
	case *core.Cairo1Class:
		hash, err := v.Hash()
		if err != nil {
			panic(fmt.Errorf("failed to hash Cairo1Class: %w", err))
		}
		return &spec.Class{
			Class: &spec.Class_Cairo1{
				Cairo1: &spec.Cairo1Class{
					Abi: v.Abi,
					EntryPoints: &spec.Cairo1EntryPoints{
						Externals:    utils.Map(v.EntryPoints.External, adaptSierra),
						L1Handlers:   utils.Map(v.EntryPoints.L1Handler, adaptSierra),
						Constructors: utils.Map(v.EntryPoints.Constructor, adaptSierra),
					},
					Program:              utils.Map(v.Program, AdaptFelt),
					ContractClassVersion: v.SemanticVersion,
				},
			},
			Domain:    0, // todo(kirill) recheck
			ClassHash: AdaptHash(hash),
		}
	default:
		panic(fmt.Errorf("unsupported cairo class %T (version=%d)", v, class.Version()))
	}
}

func adaptSierra(e core.SierraEntryPoint) *spec.SierraEntryPoint {
	return &spec.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptEntryPoint(e core.EntryPoint) *spec.EntryPoint {
	return &spec.EntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   e.Offset.Uint64(),
	}
}
