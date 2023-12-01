package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"

	"github.com/NethermindEth/juno/utils"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptClass(class core.Class, cairo0ClassHash *felt.Felt) *spec.Class {
	if class == nil {
		return nil
	}

	switch v := class.(type) {
	case *core.Cairo0Class:
		return &spec.Class{
			Class: &spec.Class_Cairo0{
				Cairo0: &spec.Cairo0Class{
					Abi:          v.Abi,
					Externals:    utils.Map(v.Externals, adaptEntryPoint),
					L1Handlers:   utils.Map(v.L1Handlers, adaptEntryPoint),
					Constructors: utils.Map(v.Constructors, adaptEntryPoint),
					Program:      []byte(v.Program),
				},
			},
		}
	case *core.Cairo1Class:
		return &spec.Class{
			Class: &spec.Class_Cairo1{
				Cairo1: &spec.Cairo1Class{
					Abi: []byte(v.Abi),
					EntryPoints: &spec.Cairo1EntryPoints{
						Externals:    utils.Map(v.EntryPoints.External, adaptSierra),
						L1Handlers:   utils.Map(v.EntryPoints.L1Handler, adaptSierra),
						Constructors: utils.Map(v.EntryPoints.Constructor, adaptSierra),
					},
					Program:              utils.Map(v.Program, AdaptFelt),
					ContractClassVersion: v.SemanticVersion,
					Compiled:             nil,
				},
			},
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
		Offset:   AdaptFelt(e.Offset),
	}
}
