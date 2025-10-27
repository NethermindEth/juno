package rpcv9

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
)

type CalldataInputs = rpccore.LimitSlice[felt.Felt, rpccore.FunctionCalldataLimit]

// https://github.com/starkware-libs/starknet-specs/blob/v0.3.0/api/starknet_api_openrpc.json#L2344
type FunctionCall struct {
	ContractAddress    felt.Felt      `json:"contract_address"`
	EntryPointSelector felt.Felt      `json:"entry_point_selector"`
	Calldata           CalldataInputs `json:"calldata"`
}

func adaptDeclaredClass(declaredClass json.RawMessage) (core.ClassDefinition, error) {
	var feederClass starknet.ClassDefinition
	err := json.Unmarshal(declaredClass, &feederClass)
	if err != nil {
		return nil, err
	}

	switch {
	case feederClass.Sierra != nil:
		compiledClass, cErr := compiler.Compile(feederClass.Sierra)
		if cErr != nil {
			return nil, cErr
		}
		return sn2core.AdaptSierraClass(feederClass.Sierra, compiledClass)
	case feederClass.DeprecatedCairo != nil:
		program := feederClass.DeprecatedCairo.Program

		// strip the quotes
		if len(program) < 2 {
			return nil, errors.New("invalid program")
		}
		base64Program := string(program[1 : len(program)-1])

		feederClass.DeprecatedCairo.Program, err = utils.Gzip64Decode(base64Program)
		if err != nil {
			return nil, err
		}

		return sn2core.AdaptDeprecatedCairoClass(feederClass.DeprecatedCairo)
	default:
		return nil, errors.New("empty class")
	}
}

/****************************************************
		Class Handlers
*****************************************************/

// Class gets the contract class definition in the given block associated with the given hash
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L410
func (h *Handler) Class(id *BlockID, classHash *felt.Felt) (*rpcv6.Class, *jsonrpc.Error) {
	state, stateCloser, rpcErr := h.stateByBlockID(id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClass")

	declared, err := state.Class(classHash)
	if err != nil {
		return nil, rpccore.ErrClassHashNotFound
	}

	var rpcClass *rpcv6.Class
	switch c := declared.Class.(type) {
	case *core.DeprecatedCairoClass:
		rpcClass = &rpcv6.Class{
			Abi:     c.Abi,
			Program: c.Program,
			EntryPoints: rpcv6.EntryPoints{
				Constructor: adaptDeprecatedCairoEntryPoints(c.Constructors),
				External:    adaptDeprecatedCairoEntryPoints(c.Externals),
				L1Handler:   adaptDeprecatedCairoEntryPoints(c.L1Handlers),
			},
		}
	case *core.SierraClass:
		rpcClass = &rpcv6.Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints: rpcv6.EntryPoints{
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

// ClassAt gets the contract class definition in the given block instantiated by the given contract address
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L499
func (h *Handler) ClassAt(id *BlockID, address *felt.Felt) (*rpcv6.Class, *jsonrpc.Error) {
	classHash, err := h.ClassHashAt(id, address)
	if err != nil {
		return nil, err
	}
	return h.Class(id, classHash)
}

// ClassHashAt gets the class hash for the contract deployed at the given address in the given block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L459
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

func adaptDeprecatedCairoEntryPoints(entryPoints []core.DeprecatedEntryPoint) []rpcv6.EntryPoint {
	adaptedEntryPoints := make([]rpcv6.EntryPoint, len(entryPoints))
	for i, entryPoint := range entryPoints {
		adaptedEntryPoints[i] = rpcv6.EntryPoint{
			Offset:   entryPoint.Offset,
			Selector: entryPoint.Selector,
		}
	}
	return adaptedEntryPoints
}

func adaptCairo1EntryPoints(entryPoints []core.SierraEntryPoint) []rpcv6.EntryPoint {
	adaptedEntryPoints := make([]rpcv6.EntryPoint, len(entryPoints))
	for i, entryPoint := range entryPoints {
		adaptedEntryPoints[i] = rpcv6.EntryPoint{
			Index:    &entryPoint.Index,
			Selector: entryPoint.Selector,
		}
	}
	return adaptedEntryPoints
}
