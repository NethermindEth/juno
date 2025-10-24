package rpcv7

import (
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
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
	case feederClass.V1 != nil:
		compiledClass, cErr := compiler.Compile(feederClass.V1)
		if cErr != nil {
			return nil, cErr
		}
		return sn2core.AdaptSierraClass(feederClass.V1, compiledClass)
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

		return sn2core.AdaptDeprecatedCairoClass(feederClass.V0)
	default:
		return nil, errors.New("empty class")
	}
}
