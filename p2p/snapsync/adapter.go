package snap

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snapsync/p2pproto"
	"github.com/pkg/errors"
)

func coreClassToProto(hash *felt.Felt, class core.Class) (*p2pproto.ContractClass, error) {
	if class == nil {
		return nil, nil
	}
	switch class := class.(type) {
	case *core.Cairo0Class:
		abistr, err := class.Abi.MarshalJSON()
		if err != nil {
			return nil, err
		}

		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo0{
				Cairo0: &p2pproto.Cairo0Class{
					ConstructorEntryPoints: MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Constructors),
					ExternalEntryPoints:    MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Externals),
					L1HandlerEntryPoints:   MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.L1Handlers),
					Program:                class.Program,
					Abi:                    string(abistr),
					Hash:                   feltToFieldElement(hash),
				},
			},
		}, nil
	case *core.Cairo1Class:
		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo1{
				Cairo1: &p2pproto.Cairo1Class{
					ConstructorEntryPoints: MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.Constructor),
					ExternalEntryPoints:    MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.External),
					L1HandlerEntryPoints:   MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.L1Handler),
					Program:                feltsToFieldElements(class.Program),
					ProgramHash:            feltToFieldElement(class.ProgramHash),
					Abi:                    class.Abi,
					Hash:                   feltToFieldElement(hash),
					SemanticVersioning:     class.SemanticVersion,
				},
			},
		}, nil
	default:
		return nil, errors.Errorf("unsupported class type %T", class)
	}
}
