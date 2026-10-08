package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

const (
	headerHash = iota
	headerParentHash
	headerNumber
	headerGlobalStateRoot
	headerSequencerAddress
	headerTransactionCount
	headerEventCount
	headerTimestamp
	headerProtocolVersion
	headerEventsBloom
	headerL1GasPriceETH
	headerSignatures
	headerL1GasPriceSTRK
	headerL1DAMode
	headerL1DataGasPrice
	headerL2GasPrice
)

var headerFields = fieldsOf[core.Header]([]string{
	headerHash:             "Hash",
	headerParentHash:       "ParentHash",
	headerNumber:           "Number",
	headerGlobalStateRoot:  "GlobalStateRoot",
	headerSequencerAddress: "SequencerAddress",
	headerTransactionCount: "TransactionCount",
	headerEventCount:       "EventCount",
	headerTimestamp:        "Timestamp",
	headerProtocolVersion:  "ProtocolVersion",
	headerEventsBloom:      "EventsBloom",
	headerL1GasPriceETH:    "gasprice",
	headerSignatures:       "Signatures",
	headerL1GasPriceSTRK:   "gaspricestrk",
	headerL1DAMode:         "L1DAMode",
	headerL1DataGasPrice:   "L1DataGasPrice",
	headerL2GasPrice:       "L2GasPrice",
})

// ReadHeader reads a stored block header. The keys of the legacy fields GasPrice,
// GasPriceSTRK and ExtraData are skipped, as fxamacker skips them.
func ReadHeader(r *fast.Reader, dst *core.Header) error {
	var prices *[2]core.GasPrice
	s, err := r.OpenStruct(headerFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case headerHash:
			err = readFelt(r, &dst.Hash)
		case headerParentHash:
			err = readFelt(r, &dst.ParentHash)
		case headerNumber:
			err = readUint64(r, &dst.Number)
		case headerGlobalStateRoot:
			err = readFelt(r, &dst.GlobalStateRoot)
		case headerSequencerAddress:
			err = readFelt(r, &dst.SequencerAddress)
		case headerTransactionCount:
			err = readUint64(r, &dst.TransactionCount)
		case headerEventCount:
			err = readUint64(r, &dst.EventCount)
		case headerTimestamp:
			err = readUint64(r, &dst.Timestamp)
		case headerProtocolVersion:
			err = readString(r, &dst.ProtocolVersion)
		case headerEventsBloom:
			err = readBloom(r, &dst.EventsBloom)
		case headerL1GasPriceETH:
			err = readFelt(r, &dst.L1GasPriceETH)
		case headerSignatures:
			err = readSlice(r, &dst.Signatures, readFeltPtrSlice)
		case headerL1GasPriceSTRK:
			err = readFelt(r, &dst.L1GasPriceSTRK)
		case headerL1DAMode:
			var mode uint64
			err = readUint64(r, &mode)
			dst.L1DAMode = core.L1DAMode(mode)
		case headerL1DataGasPrice:
			err = readGasPricePtr(r, &dst.L1DataGasPrice, &prices, 0)
		case headerL2GasPrice:
			err = readGasPricePtr(r, &dst.L2GasPrice, &prices, 1)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	gasPriceInWei = iota
	gasPriceInFri
)

var gasPriceFields = fieldsOf[core.GasPrice]([]string{
	gasPriceInWei: "PriceInWei",
	gasPriceInFri: "PriceInFri",
})

// readGasPricePtr reads a gas price behind a pointer, into slot of prices. A header's two gas
// prices share one allocation. Null leaves the pointer nil.
func readGasPricePtr(r *fast.Reader, dst **core.GasPrice, prices **[2]core.GasPrice, slot int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	if *prices == nil {
		*prices = new([2]core.GasPrice)
	}
	value := &(*prices)[slot]
	if err := readGasPrice(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}

func readGasPrice(r *fast.Reader, dst *core.GasPrice) error {
	s, err := r.OpenStruct(gasPriceFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case gasPriceInWei:
			err = readFelt(r, &dst.PriceInWei)
		case gasPriceInFri:
			err = readFelt(r, &dst.PriceInFri)
		default:
			err = r.Skip()
		}
	}
	return err
}
