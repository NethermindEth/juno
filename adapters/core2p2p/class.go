package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/class"
)

func AdaptClass(cls core.Class) *class.Class {
	if cls == nil {
		return nil
	}

	hash, err := cls.Hash()
	if err != nil {
		panic(fmt.Errorf("failed to hash %t: %w", cls, err))
	}

	switch v := cls.(type) {
	case *core.Cairo0Class:
		return &class.Class{
			Class: &class.Class_Cairo0{
				Cairo0: &class.Cairo0Class{
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
		return &class.Class{
			Class: &class.Class_Cairo1{
				Cairo1: &class.Cairo1Class{
					Abi: v.Abi,
					EntryPoints: &class.Cairo1EntryPoints{
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
		panic(fmt.Errorf("unsupported cairo class %T (version=%d)", v, cls.Version()))
	}
}

func adaptSierra(e core.SierraEntryPoint) *class.SierraEntryPoint {
	return &class.SierraEntryPoint{
		Index:    e.Index,
		Selector: AdaptFelt(e.Selector),
	}
}

func adaptEntryPoint(e core.EntryPoint) *class.EntryPoint {
	return &class.EntryPoint{
		Selector: AdaptFelt(e.Selector),
		Offset:   e.Offset.Uint64(),
	}
}
