package starknet

import "github.com/NethermindEth/juno/core/felt"

// Block object returned by the feeder in JSON format for "get_block" endpoint
type Block struct {
	Hash             *felt.Felt            `json:"block_hash"`
	ParentHash       *felt.Felt            `json:"parent_block_hash"`
	Number           uint64                `json:"block_number"`
	StateRoot        *felt.Felt            `json:"state_root"`
	Status           string                `json:"status"`
	Transactions     []*Transaction        `json:"transactions"`
	Timestamp        uint64                `json:"timestamp"`
	Version          string                `json:"starknet_version"`
	Receipts         []*TransactionReceipt `json:"transaction_receipts"`
	SequencerAddress *felt.Felt            `json:"sequencer_address"`
	GasPriceSTRK     *felt.Felt            `json:"strk_l1_gas_price"`

	// TODO we can remove the GasPrice method and the GasPriceLegacy field
	// once v0.13 lands on mainnet. In the meantime, we include both to support
	// pre-v0.13 jsons, where `eth_l1_gas_price` was called `gas_price`.
	GasPriceLegacy *felt.Felt `json:"gas_price"`
	GasPriceWEI    *felt.Felt `json:"eth_l1_gas_price"`
}

func (b *Block) GasPriceETH() *felt.Felt {
	if b.GasPriceWEI != nil {
		return b.GasPriceWEI
	}
	return b.GasPriceLegacy
}
