package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

type converter struct {
	classprovider ClassProvider
}

func NewConverter(classProvider ClassProvider) *converter {
	return &converter{
		classprovider: classProvider,
	}
}

func (c *converter) coreBlockToProtobufHeader(block *core.Block) (*p2pproto.BlockHeader, error) {
	txCommitment, err := block.CalculateTransactionCommitment()
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate transaction commitment")
	}

	eventCommitment, err := block.CalculateEventCommitment()
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate event commitment")
	}

	return &p2pproto.BlockHeader{
		Hash:                  feltToFieldElement(block.Hash),
		ParentBlockHash:       feltToFieldElement(block.ParentHash),
		BlockNumber:           block.Number,
		GlobalStateRoot:       feltToFieldElement(block.GlobalStateRoot),
		SequencerAddress:      feltToFieldElement(block.SequencerAddress),
		BlockTimestamp:        block.Timestamp,
		TransactionCount:      uint32(len(block.Transactions)),
		TransactionCommitment: feltToFieldElement(txCommitment),
		EventCount:            uint32(block.EventCount),
		EventCommitment:       feltToFieldElement(eventCommitment),
		ProtocolVersion:       0, // TODO: What is the correct value here?
	}, nil
}

func (c *converter) coreBlockToProtobufBody(block *core.Block) (*p2pproto.BlockBody, error) {
	prototransactions := make([]*p2pproto.Transaction, len(block.Transactions))
	protoreceipts := make([]*p2pproto.Receipt, len(block.Receipts))
	for i, transaction := range block.Transactions {
		tx, receipt, err := c.coreTxToProtobufTx(transaction, block.Receipts[i])
		if err != nil {
			return nil, errors.Wrap(err, "unable convert core block to protobuff")
		}

		prototransactions[i] = tx
		protoreceipts[i] = receipt
	}

	return &p2pproto.BlockBody{
		Transactions: prototransactions,
		Receipts:     protoreceipts,
	}, nil
}

func coreEventToProtobuf(events []*core.Event) []*p2pproto.Event {
	return protoMapArray(events, func(event *core.Event) *p2pproto.Event {
		return &p2pproto.Event{
			FromAddress: feltToFieldElement(event.From),
			Keys:        feltsToFieldElements(event.Keys),
			Data:        feltsToFieldElements(event.Data),
		}
	})
}

func coreL2ToL1MessageToProtobuf(messages []*core.L2ToL1Message) []*p2pproto.MessageToL1 {
	return protoMapArray(messages, func(message *core.L2ToL1Message) *p2pproto.MessageToL1 {
		return &p2pproto.MessageToL1{
			FromAddress: feltToFieldElement(message.From),
			Payload:     feltsToFieldElements(message.Payload),
			ToAddress:   addressToProto(message.To),
		}
	})
}

func addressToProto(to common.Address) *p2pproto.EthereumAddress {
	return &p2pproto.EthereumAddress{
		Elements: to.Bytes(),
	}
}

func protoToAddress(to *p2pproto.EthereumAddress) common.Address {
	addr := common.Address{}
	if to != nil {
		copy(addr[:], to.Elements)
	}
	return addr
}

func (c *converter) protobufHeaderAndBodyToCoreBlock(
	header *p2pproto.BlockHeader,
	body *p2pproto.BlockBody,
) (*core.Block, map[felt.Felt]core.Class, error) {
	// TODO: TX and Event commitment validation

	block := &core.Block{
		Header: &core.Header{
			Hash:             fieldElementToFelt(header.Hash),
			ParentHash:       fieldElementToFelt(header.ParentBlockHash),
			Number:           header.BlockNumber,
			GlobalStateRoot:  fieldElementToFelt(header.GlobalStateRoot),
			SequencerAddress: fieldElementToFelt(header.SequencerAddress),
			TransactionCount: uint64(len(body.Transactions)),
			EventCount:       0, // many events per receipt
			Timestamp:        header.BlockTimestamp,
			ProtocolVersion:  "",
			ExtraData:        nil,
			//nolint:all
			EventsBloom: bloom.New(8192, 6),
		},
		Transactions: make([]core.Transaction, 0), // Assuming it's initialised as an empty slice
		Receipts:     make([]*core.TransactionReceipt, 0),
	}

	eventcount := 0
	declaredClasses := map[felt.Felt]core.Class{}

	for i := uint32(0); i < header.TransactionCount; i++ {
		transaction, receipt, classHash, class, err := c.protobufTransactionToCore(body.Transactions[i], body.Receipts[i])
		if err != nil {
			return nil, nil, err
		}
		block.Transactions = append(block.Transactions, transaction)
		block.Receipts = append(block.Receipts, receipt)

		if classHash != nil {
			declaredClasses[*classHash] = class
		}

		eventcount += len(receipt.Events)
	}

	block.EventCount = uint64(eventcount)
	block.EventsBloom = core.EventsBloom(block.Receipts)

	return block, declaredClasses, nil
}

func protobufCommonReceiptToCoreReceipt(commonReceipt *p2pproto.CommonTransactionReceiptProperties) *core.TransactionReceipt {
	return &core.TransactionReceipt{
		Fee:                fieldElementToFelt(commonReceipt.GetActualFee()),
		Events:             coreEventFromProtobuf(commonReceipt.GetEvents()),
		L2ToL1Message:      coreL2ToL1MessageFromProtobuf(commonReceipt.GetMessagesSent()),
		TransactionHash:    fieldElementToFelt(commonReceipt.GetTransactionHash()),
		ExecutionResources: MapValueWithReflection[*core.ExecutionResources](commonReceipt.GetExecutionResources()),
	}
}

func coreL2ToL1MessageFromProtobuf(messages []*p2pproto.MessageToL1) []*core.L2ToL1Message {
	return protoMapArray(messages, func(msg *p2pproto.MessageToL1) *core.L2ToL1Message {
		return &core.L2ToL1Message{
			From:    fieldElementToFelt(msg.FromAddress),
			Payload: fieldElementsToFelts(msg.Payload),
			To:      protoToAddress(msg.ToAddress),
		}
	})
}

func coreEventFromProtobuf(events []*p2pproto.Event) []*core.Event {
	return protoMapArray(events, func(event *p2pproto.Event) *core.Event {
		return &core.Event{
			Data: fieldElementsToFelts(event.Data),
			From: fieldElementToFelt(event.FromAddress),
			Keys: fieldElementsToFelts(event.Keys),
		}
	})
}
