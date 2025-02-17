package rpcv7

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
)

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L268-L280
type Class struct {
	SierraProgram        []*felt.Felt `json:"sierra_program,omitempty"`
	Program              string       `json:"program,omitempty"`
	ContractClassVersion string       `json:"contract_class_version,omitempty"`
	EntryPoints          EntryPoints  `json:"entry_points_by_type"`
	Abi                  any          `json:"abi"`
}

type EntryPoints struct {
	Constructor []EntryPoint `json:"CONSTRUCTOR"`
	External    []EntryPoint `json:"EXTERNAL"`
	L1Handler   []EntryPoint `json:"L1_HANDLER"`
}

type EntryPoint struct {
	Index    *uint64    `json:"function_idx,omitempty"`
	Offset   *felt.Felt `json:"offset,omitempty"`
	Selector *felt.Felt `json:"selector"`
}

// https://github.com/starkware-libs/starknet-specs/blob/v0.3.0/api/starknet_api_openrpc.json#L2344
type FunctionCall struct {
	ContractAddress    felt.Felt   `json:"contract_address"`
	EntryPointSelector felt.Felt   `json:"entry_point_selector"`
	Calldata           []felt.Felt `json:"calldata"`
}

func adaptDeclaredClass(declaredClass json.RawMessage) (core.Class, error) {
	var feederClass starknet.ClassDefinition
	err := json.Unmarshal(declaredClass, &feederClass)
	if err != nil {
		return nil, err
	}

	switch {
	case feederClass.V1 != nil:
		compiledClass, cErr := compiler.Compile(feederClass.V1)
		if cErr != nil {
			return nil, cErr
		}
		return sn2core.AdaptCairo1Class(feederClass.V1, compiledClass)
	case feederClass.V0 != nil:
		program := feederClass.V0.Program

		// strip the quotes
		if len(program) < 2 {
			return nil, errors.New("invalid program")
		}
		base64Program := string(program[1 : len(program)-1])

		feederClass.V0.Program, err = utils.Gzip64Decode(base64Program)
		if err != nil {
			return nil, err
		}

		return sn2core.AdaptCairo0Class(feederClass.V0)
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
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L248
func (h *Handler) Class(id BlockID, classHash felt.Felt) (*Class, *jsonrpc.Error) {
	state, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClass")

	declared, err := state.Class(&classHash)
	if err != nil {
		return nil, rpccore.ErrClassHashNotFound
	}

	var rpcClass *Class
	switch c := declared.Class.(type) {
	case *core.Cairo0Class:
		adaptEntryPoint := func(ep core.EntryPoint) EntryPoint {
			return EntryPoint{
				Offset:   ep.Offset,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:     c.Abi,
			Program: c.Program,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.Constructors, adaptEntryPoint),
				External:    utils.Map(c.Externals, adaptEntryPoint),
				L1Handler:   utils.Map(c.L1Handlers, adaptEntryPoint),
			},
		}
	case *core.Cairo1Class:
		adaptEntryPoint := func(ep core.SierraEntryPoint) EntryPoint {
			return EntryPoint{
				Index:    &ep.Index,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.EntryPoints.Constructor, adaptEntryPoint),
				External:    utils.Map(c.EntryPoints.External, adaptEntryPoint),
				L1Handler:   utils.Map(c.EntryPoints.L1Handler, adaptEntryPoint),
			},
		}
	default:
		return nil, rpccore.ErrClassHashNotFound
	}

	return rpcClass, nil
}
