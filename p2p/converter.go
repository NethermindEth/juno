package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// Shoulve reflect these....

func coreBlockToProtobufHeader(block *core.Block) (*grpcclient.BlockHeader, error) {
	txCommitment, err := block.CalculateTransactionCommitment()
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate transaction commitment")
	}

	eventCommitment, err := block.CalculateEventCommitment()
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate event commitment")
	}

	return &grpcclient.BlockHeader{
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
		ProtocolVersion:       0, //TODO: What is the correct value here?
	}, nil
}

func coreBlockToProtobufBody(block *core.Block) *grpcclient.BlockBody {
	grpctransactions := make([]*grpcclient.Transaction, len(block.Transactions))
	grpcreceipts := make([]*grpcclient.Receipt, len(block.Receipts))
	for i, transaction := range block.Transactions {
		grpctransactions[i], grpcreceipts[i] = coreTxToProtobufTx(transaction, block.Receipts[i])
	}

	return &grpcclient.BlockBody{
		Transactions: grpctransactions,
		Receipts:     grpcreceipts,
	}
}

func coreEventToProtobuf(events []*core.Event) []*grpcclient.Event {
	grpcevents := make([]*grpcclient.Event, len(events))
	for i, event := range events {
		grpcevents[i] = &grpcclient.Event{
			FromAddress: feltToFieldElement(event.From),
			Keys:        feltsToFieldElements(event.Keys),
			Data:        feltsToFieldElements(event.Data),
		}
	}

	return grpcevents
}

func coreL2ToL1MessageToProtobuf(messages []*core.L2ToL1Message) []*grpcclient.MessageToL1 {
	grpcmessages := make([]*grpcclient.MessageToL1, len(messages))

	for i, message := range messages {
		grpcmessages[i] = &grpcclient.MessageToL1{
			FromAddress: feltToFieldElement(message.From),
			Payload:     feltsToFieldElements(message.Payload),
			ToAddress:   addressToProto(message.To),
		}
	}

	return grpcmessages
}

func addressToProto(to common.Address) *grpcclient.EthereumAddress {
	return &grpcclient.EthereumAddress{
		Elements: to.Bytes(),
	}
}

func protoToAddress(to *grpcclient.EthereumAddress) common.Address {
	addr := common.Address{}
	copy(addr[:], to.Elements)
	return addr
}

func coreStateUpdateToProtobufStateUpdate(corestateupdate *core.StateUpdate) *grpcclient.StateDiffs_BlockStateUpdateWithHash {

	contractdiffs := make([]*grpcclient.BlockStateUpdate_ContractDiff, len(corestateupdate.StateDiff.StorageDiffs))
	i := 0
	for key, diffs := range corestateupdate.StateDiff.StorageDiffs {

		storagediff := make([]*grpcclient.BlockStateUpdate_StorageDiff, len(diffs))
		for i2, diff := range diffs {
			storagediff[i2] = &grpcclient.BlockStateUpdate_StorageDiff{
				Key:   feltToFieldElement(diff.Key),
				Value: feltToFieldElement(diff.Value),
			}
		}

		contractdiffs[i] = &grpcclient.BlockStateUpdate_ContractDiff{
			ContractAddress: feltToFieldElement(&key),
			Nonce:           feltToFieldElement(corestateupdate.StateDiff.Nonces[key]),
			StorageDiffs:    nil,
		}

		i += 1
	}

	deployedcontracts := make([]*grpcclient.BlockStateUpdate_DeployedContract, len(corestateupdate.StateDiff.DeployedContracts))
	for i, contract := range corestateupdate.StateDiff.DeployedContracts {
		deployedcontracts[i] = &grpcclient.BlockStateUpdate_DeployedContract{
			ContractAddress:   feltToFieldElement(contract.Address),
			ContractClassHash: feltToFieldElement(contract.ClassHash),
		}
	}

	declaredv1classes := make([]*grpcclient.BlockStateUpdate_DeclaredV1Class, len(corestateupdate.StateDiff.DeclaredV1Classes))
	for i, contract := range corestateupdate.StateDiff.DeclaredV1Classes {
		declaredv1classes[i] = &grpcclient.BlockStateUpdate_DeclaredV1Class{
			ClassHash:         feltToFieldElement(contract.ClassHash),
			CompiledClassHash: feltToFieldElement(contract.CompiledClassHash),
		}
	}

	replacedclasses := make([]*grpcclient.BlockStateUpdate_ReplacedClasses, len(corestateupdate.StateDiff.ReplacedClasses))
	for i, contract := range corestateupdate.StateDiff.ReplacedClasses {
		replacedclasses[i] = &grpcclient.BlockStateUpdate_ReplacedClasses{
			ContractAddress:   feltToFieldElement(contract.Address),
			ContractClassHash: feltToFieldElement(contract.ClassHash),
		}
	}

	stateupdate := &grpcclient.BlockStateUpdate{
		ContractDiffs:               contractdiffs,
		DeployedContracts:           deployedcontracts,
		DeclaredContractClassHashes: feltsToFieldElements(corestateupdate.StateDiff.DeclaredV0Classes),
		DeclaredV1Classes:           declaredv1classes,
		ReplacedClasses:             replacedclasses,
	}

	return &grpcclient.StateDiffs_BlockStateUpdateWithHash{
		StateUpdate: stateupdate,
	}

}

func protobufHeaderToCoreBlock(header *grpcclient.BlockHeader, body *grpcclient.BlockBody, network utils.Network) (*core.Block, error) {
	parentHash := fieldElementToFelt(header.ParentBlockHash)
	globalStateRoot := fieldElementToFelt(header.GlobalStateRoot)
	sequencerAddress := fieldElementToFelt(header.SequencerAddress)
	// TODO: these are validation
	// txCommitment := fieldElementToFelt(header.TransactionCommitment)
	// eventCommitment := fieldElementToFelt(header.EventCommitment)

	block := &core.Block{
		Header: &core.Header{
			Hash:             fieldElementToFelt(header.Hash),
			ParentHash:       parentHash,
			Number:           header.BlockNumber,
			GlobalStateRoot:  globalStateRoot,
			SequencerAddress: sequencerAddress,
			TransactionCount: uint64(len(body.Transactions)),
			EventCount:       0, // many events per receipt
			Timestamp:        header.BlockTimestamp,
			ProtocolVersion:  "",
			ExtraData:        nil,
			EventsBloom:      bloom.New(8192, 6),
		},
		Transactions: make([]core.Transaction, 0), // Assuming it's initialized as an empty slice
		Receipts:     make([]*core.TransactionReceipt, 0),
	}

	for i := uint32(0); i < header.TransactionCount; i++ {
		// Assuming you have a function to convert a transaction from protobuf to core
		transaction, receipt, err := protobufTransactionToCore(body.Transactions[i], body.Receipts[i], network)
		if err != nil {
			return nil, err
		}
		block.Transactions = append(block.Transactions, transaction)
		block.Receipts = append(block.Receipts, receipt)
	}

	return block, nil
}

func protobufCommonReceiptToCoreReceipt(commonReceipt *grpcclient.CommonTransactionReceiptProperties) *core.TransactionReceipt {
	receipt := &core.TransactionReceipt{
		Fee:                fieldElementToFelt(commonReceipt.GetActualFee()),
		Events:             coreEventFromProtobuf(commonReceipt.GetEvents()),
		L2ToL1Message:      coreL2ToL1MessageFromProtobuf(commonReceipt.GetMessagesSent()),
		TransactionHash:    fieldElementToFelt(commonReceipt.GetTransactionHash()),
		ExecutionResources: protobufToCoreExecutionResources(commonReceipt.GetExecutionResources()),
	}

	return receipt
}

func coreL2ToL1MessageFromProtobuf(sent []*grpcclient.MessageToL1) []*core.L2ToL1Message {
	messages := make([]*core.L2ToL1Message, len(sent))
	for i, grpcMsg := range sent {
		msg := &core.L2ToL1Message{
			From:    fieldElementToFelt(grpcMsg.FromAddress),
			Payload: fieldElementsToFelts(grpcMsg.Payload),
			To:      protoToAddress(grpcMsg.ToAddress),
		}
		messages[i] = msg
	}
	return messages
}

func coreEventFromProtobuf(events []*grpcclient.Event) []*core.Event {
	coreevents := make([]*core.Event, len(events))
	for i, event := range events {
		coreevents[i] = &core.Event{
			Data: fieldElementsToFelts(event.Data),
			From: fieldElementToFelt(event.FromAddress),
			Keys: fieldElementsToFelts(event.Keys),
		}
	}

	return coreevents
}
