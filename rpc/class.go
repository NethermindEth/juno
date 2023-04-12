package rpc

import "github.com/NethermindEth/juno/core/felt"

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

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L2023
type FunctionCall struct {
	ContractAddress    *felt.Felt   `json:"contract_address"`
	EntryPointSelector *felt.Felt   `json:"entry_point_selector"`
	Calldata           []*felt.Felt `json:"calldata"`
}
