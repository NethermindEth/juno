package snap

import (
	"encoding/json"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snap/p2pproto"
	"github.com/pkg/errors"
)

func coreClassToProtobufClass(hash *felt.Felt, theclass *core.DeclaredClass) (*p2pproto.ContractClass, error) {
	return coreUndeclaredClassToProtobufClass(hash, theclass.Class)
}

func coreUndeclaredClassToProtobufClass(hash *felt.Felt, theclass core.Class) (*p2pproto.ContractClass, error) {
	if theclass == nil {
		return nil, nil
	}

	switch class := theclass.(type) {
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
		return nil, errors.Errorf("unsupported class type %T", theclass)
	}
}

func protobufClassToCoreClass(class *p2pproto.ContractClass) (*felt.Felt, core.Class, error) {
	switch v := class.Class.(type) {
	case *p2pproto.ContractClass_Cairo0:
		hash := fieldElementToFelt(v.Cairo0.Hash)

		abiraw := json.RawMessage{}
		err := json.Unmarshal([]byte(v.Cairo0.Abi), &abiraw)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error unmarshalling cairo0 abi")
		}

		coreClass := &core.Cairo0Class{
			Abi:          abiraw,
			Externals:    MapValueViaReflect[[]core.EntryPoint](v.Cairo0.ExternalEntryPoints),
			L1Handlers:   MapValueViaReflect[[]core.EntryPoint](v.Cairo0.L1HandlerEntryPoints),
			Constructors: MapValueViaReflect[[]core.EntryPoint](v.Cairo0.ConstructorEntryPoints),
			Program:      v.Cairo0.Program,
		}
		return hash, coreClass, nil
	case *p2pproto.ContractClass_Cairo1:
		coreClass := &core.Cairo1Class{
			Abi: v.Cairo1.Abi,
			EntryPoints: struct {
				Constructor []core.SierraEntryPoint
				External    []core.SierraEntryPoint
				L1Handler   []core.SierraEntryPoint
			}{
				Constructor: MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.ConstructorEntryPoints),
				External:    MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.ExternalEntryPoints),
				L1Handler:   MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.L1HandlerEntryPoints),
			},
			Program:         fieldElementsToFelts(v.Cairo1.Program),
			SemanticVersion: v.Cairo1.SemanticVersioning,
		}

		abihash, err := crypto.StarknetKeccak([]byte(coreClass.Abi))
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to fetch abi hash")
		}
		coreClass.AbiHash = abihash
		coreClass.ProgramHash = crypto.PoseidonArray(coreClass.Program...)

		hash := fieldElementToFelt(v.Cairo1.Hash)

		return hash, coreClass, nil
	}

	return nil, nil, errors.Errorf("unknown class type %T", class.Class)
}
