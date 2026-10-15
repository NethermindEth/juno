package rpc2core

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
)

func AdaptBlock(
	response *rpcv10.BlockWithReceipts,
	network *networks.Network,
) (core.Block, error) {
	if response == nil {
		return core.Block{}, errors.New("nil block")
	}
	if response.Number == nil {
		return core.Block{}, errors.New("block header is missing block_number")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	receipts := make([]*core.TransactionReceipt, len(response.Transactions))
	for i := range response.Transactions {
		receipt := &response.Transactions[i].Receipt
		if receipt.Hash == nil {
			return core.Block{}, fmt.Errorf(
				"block %d transaction %d is missing transaction_hash", *response.Number, i,
			)
		}

		adaptedReceipt, err := adaptReceipt(receipt)
		if err != nil {
			return core.Block{}, fmt.Errorf(
				"block %d transaction %s: %w", *response.Number, receipt.Hash, err,
			)
		}
		receipts[i] = &adaptedReceipt

		txns[i], err = adaptTransaction(&response.Transactions[i].Transaction, receipt, network)
		if err != nil {
			return core.Block{}, fmt.Errorf(
				"block %d transaction %s: %w", *response.Number, receipt.Hash, err,
			)
		}
	}

	header, err := adaptHeader(&response.BlockHeader, receipts)
	if err != nil {
		return core.Block{}, fmt.Errorf("block %d: %w", *response.Number, err)
	}

	return core.Block{
		Header:       &header,
		Transactions: txns,
		Receipts:     receipts,
	}, nil
}

func adaptHeader(
	header *rpcv10.BlockHeader,
	receipts []*core.TransactionReceipt,
) (core.Header, error) {
	missingField := cmp.Or(
		missing("block_hash", header.Hash),
		missing("parent_hash", header.ParentHash),
		missing("new_root", header.NewRoot),
		missing("sequencer_address", header.SequencerAddress),
		missingGasPrice("l1_gas_price", &header.L1GasPrice),
		missingGasPrice("l1_data_gas_price", &header.L1DataGasPrice),
		missingGasPrice("l2_gas_price", &header.L2GasPrice),
	)
	if missingField != "" {
		return core.Header{}, fmt.Errorf("header is missing %s", missingField)
	}

	var l1DAMode core.L1DAMode
	switch header.L1DAMode {
	case rpcv10.Blob:
		l1DAMode = core.Blob
	case rpcv10.Calldata:
		l1DAMode = core.Calldata
	default:
		return core.Header{}, fmt.Errorf("header has unknown l1_da_mode %d", header.L1DAMode)
	}

	eventCount := uint64(0)
	for _, receipt := range receipts {
		eventCount += uint64(len(receipt.Events))
	}

	return core.Header{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           *header.Number,
		GlobalStateRoot:  header.NewRoot,
		SequencerAddress: header.SequencerAddress,
		TransactionCount: uint64(len(receipts)),
		EventCount:       eventCount,
		Timestamp:        header.Timestamp,
		ProtocolVersion:  header.StarknetVersion,
		EventsBloom:      core.EventsBloom(receipts),
		L1GasPriceETH:    header.L1GasPrice.InWei,
		L1GasPriceSTRK:   header.L1GasPrice.InFri,
		L1DAMode:         l1DAMode,
		L1DataGasPrice:   new(adaptGasPrice(&header.L1DataGasPrice)),
		L2GasPrice:       new(adaptGasPrice(&header.L2GasPrice)),
		Signatures:       [][]*felt.Felt{},
	}, nil
}

func adaptGasPrice(price *rpcv10.ResourcePrice) core.GasPrice {
	return core.GasPrice{
		PriceInWei: price.InWei,
		PriceInFri: price.InFri,
	}
}

func missingGasPrice(name string, price *rpcv10.ResourcePrice) string {
	return cmp.Or(
		missing(name+".price_in_wei", price.InWei),
		missing(name+".price_in_fri", price.InFri),
	)
}

// missing returns the field's name when its value is absent, so cmp.Or can pick the first
// absent one.
func missing[T any](name string, value *T) string {
	if value == nil {
		return name
	}
	return ""
}
