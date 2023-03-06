package rpc

import (
	"github.com/NethermindEth/juno/core/felt"
)

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
type Transaction struct {
	Hash                *felt.Felt    `json:"transaction_hash,omitempty"`
	Type                string        `json:"type,omitempty"`
	Version             *felt.Felt    `json:"version,omitempty"`
	Nonce               *felt.Felt    `json:"nonce,omitempty"`
	MaxFee              *felt.Felt    `json:"max_fee,omitempty"`
	ContractAddress     *felt.Felt    `json:"contract_address,omitempty"`
	ContractAddressSalt *felt.Felt    `json:"contract_address_salt,omitempty"`
	ClassHash           *felt.Felt    `json:"class_hash,omitempty"`
	ConstructorCalldata []*felt.Felt  `json:"constructor_calldata,omitempty"`
	SenderAddress       *felt.Felt    `json:"sender_address,omitempty"`
	Signature           *[]*felt.Felt `json:"signature,omitempty"`
	Calldata            *[]*felt.Felt `json:"calldata,omitempty"`
	EntryPointSelector  *felt.Felt    `json:"entry_point_selector,omitempty"`
	CompiledClassHash   *felt.Felt    `json:"compiled_class_hash,omitempty"`
}
