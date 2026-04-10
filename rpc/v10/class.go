package rpcv10

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

// https://github.com/starkware-libs/starknet-specs/blob/release/v0.10.2/api/starknet_api_openrpc.json#L506-L522
type Class struct {
	SierraProgram        []*felt.Felt           `json:"sierra_program,omitempty"`
	Program              string                 `json:"program,omitempty"`
	ContractClassVersion string                 `json:"contract_class_version,omitempty"`
	EntryPoints          ClassEntryPointsByType `json:"entry_points_by_type"`
	Abi                  any                    `json:"abi"`
}

type ClassEntryPointsByType struct {
	Constructor []ClassEntryPoint `json:"CONSTRUCTOR"`
	External    []ClassEntryPoint `json:"EXTERNAL"`
	L1Handler   []ClassEntryPoint `json:"L1_HANDLER"`
}

type ClassEntryPoint struct {
	Index    *uint64    `json:"function_idx,omitempty"`
	Offset   *felt.Felt `json:"offset,omitempty"`
	Selector *felt.Felt `json:"selector"`
}

/****************************************************
		Class Handlers
*****************************************************/

// Class gets the contract class definition in the given block associated with the given hash
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/release/v0.10.2/api/starknet_api_openrpc.json#L484
func (h *Handler) Class(id *BlockID, classHash *felt.Felt) (*Class, *jsonrpc.Error) {
	state, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClass")

	declared, err := state.Class(classHash)
	if err != nil {
		return nil, rpccore.ErrClassHashNotFound
	}

	var rpcClass *Class
	switch c := declared.Class.(type) {
	case *core.DeprecatedCairoClass:
		rpcClass = &Class{
			Abi:     c.Abi,
			Program: c.Program,
			EntryPoints: ClassEntryPointsByType{
				Constructor: adaptDeprecatedCairoEntryPoints(c.Constructors),
				External:    adaptDeprecatedCairoEntryPoints(c.Externals),
				L1Handler:   adaptDeprecatedCairoEntryPoints(c.L1Handlers),
			},
		}
	case *core.SierraClass:
		rpcClass = &Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints: ClassEntryPointsByType{
				Constructor: adaptCairo1EntryPoints(c.EntryPoints.Constructor),
				External:    adaptCairo1EntryPoints(c.EntryPoints.External),
				L1Handler:   adaptCairo1EntryPoints(c.EntryPoints.L1Handler),
			},
		}
	default:
		return nil, rpccore.ErrClassHashNotFound
	}

	return rpcClass, nil
}

// ClassAt gets the contract class definition in the given block instantiated by the
// given contract address
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/release/v0.10.2/api/starknet_api_openrpc.json#L573
func (h *Handler) ClassAt(id *BlockID, address *felt.Felt) (*Class, *jsonrpc.Error) {
	classHash, err := h.ClassHashAt(id, address)
	if err != nil {
		return nil, err
	}
	return h.Class(id, classHash)
}

// ClassHashAt gets the class hash for the contract deployed at the given address
// in the given block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/release/v0.10.2/api/starknet_api_openrpc.json#L533
func (h *Handler) ClassHashAt(id *BlockID, address *felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClassHashAt")

	classHash, err := stateReader.ContractClassHash(address)
	if err != nil {
		return nil, rpccore.ErrContractNotFound
	}

	return &classHash, nil
}

func adaptDeprecatedCairoEntryPoints(entryPoints []core.DeprecatedEntryPoint) []ClassEntryPoint {
	adaptedEntryPoints := make([]ClassEntryPoint, len(entryPoints))
	for i, entryPoint := range entryPoints {
		adaptedEntryPoints[i] = ClassEntryPoint{
			Offset:   entryPoint.Offset,
			Selector: entryPoint.Selector,
		}
	}
	return adaptedEntryPoints
}

func adaptCairo1EntryPoints(entryPoints []core.SierraEntryPoint) []ClassEntryPoint {
	adaptedEntryPoints := make([]ClassEntryPoint, len(entryPoints))
	for i, entryPoint := range entryPoints {
		adaptedEntryPoints[i] = ClassEntryPoint{
			Index:    &entryPoint.Index,
			Selector: entryPoint.Selector,
		}
	}
	return adaptedEntryPoints
}
