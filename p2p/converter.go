package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"reflect"
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

func coreTxToProtobufTx(transaction core.Transaction, receipt *core.TransactionReceipt) (*grpcclient.Transaction, *grpcclient.Receipt) {
	commonReceipt := &grpcclient.CommonTransactionReceiptProperties{
		TransactionHash: feltToFieldElement(receipt.TransactionHash),
		ActualFee:       feltToFieldElement(receipt.Fee),
		MessagesSent:    coreL2ToL1MessageToProtobuf(receipt.L2ToL1Message),
		Events:          coreEventToProtobuf(receipt.Events),
	}

	if deployTx, ok := transaction.(*core.DeployTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Deploy{
					Deploy: &grpcclient.DeployTransaction{
						// ContractClass:       feltToFieldElement(deployTx.ClassHash), // TODO: What is this?
						ContractAddressSalt: feltToFieldElement(deployTx.ContractAddressSalt),
						ConstructorCalldata: feltsToFieldElements(deployTx.ConstructorCallData),
						Version:             feltToFieldElement(deployTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_Deploy{
					Deploy: &grpcclient.DeployTransactionReceipt{
						Common:          commonReceipt,
						ContractAddress: feltToFieldElement(deployTx.ContractAddress),
					},
				},
			}
	}

	if deployTx, ok := transaction.(*core.DeployAccountTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_DeployAccount{
					DeployAccount: &grpcclient.DeployAccountTransaction{
						ContractAddressSalt: feltToFieldElement(deployTx.ContractAddressSalt),
						ConstructorCalldata: feltsToFieldElements(deployTx.ConstructorCallData),
						ClassHash:           feltToFieldElement(deployTx.ClassHash),
						MaxFee:              feltToFieldElement(deployTx.MaxFee),
						Signature:           feltsToFieldElements(deployTx.TransactionSignature),
						Nonce:               feltToFieldElement(deployTx.Nonce),
						Version:             feltToFieldElement(deployTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_DeployAccount{
					DeployAccount: &grpcclient.DeployAccountTransactionReceipt{
						Common:          commonReceipt,
						ContractAddress: feltToFieldElement(deployTx.ContractAddress),
					},
				},
			}
	}

	if declareTx, ok := transaction.(*core.DeclareTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Declare{
					Declare: &grpcclient.DeclareTransaction{
						// ContractClass: nil, // TODO: What is this?
						SenderAddress: feltToFieldElement(declareTx.SenderAddress),
						MaxFee:        feltToFieldElement(declareTx.MaxFee),
						Signature:     feltsToFieldElements(declareTx.TransactionSignature),
						Nonce:         feltToFieldElement(declareTx.Nonce),
						Version:       feltToFieldElement(declareTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_Declare{
					Declare: &grpcclient.DeclareTransactionReceipt{
						Common: commonReceipt,
					},
				},
			}
	}

	if invokeTx, ok := transaction.(*core.InvokeTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Invoke{
					Invoke: &grpcclient.InvokeTransaction{
						ContractAddress:    feltToFieldElement(invokeTx.ContractAddress),
						EntryPointSelector: feltToFieldElement(invokeTx.EntryPointSelector),
						Calldata:           feltsToFieldElements(invokeTx.CallData),
						Signature:          feltsToFieldElements(invokeTx.TransactionSignature),
						MaxFee:             feltToFieldElement(invokeTx.MaxFee),
						Nonce:              feltToFieldElement(invokeTx.Nonce),
						Version:            feltToFieldElement(invokeTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_Invoke{
					Invoke: &grpcclient.InvokeTransactionReceipt{
						Common: commonReceipt,
					},
				},
			}
	}

	if l1HandlerTx, ok := transaction.(*core.L1HandlerTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_L1Handler{
					L1Handler: &grpcclient.L1HandlerTransaction{
						ContractAddress:    feltToFieldElement(l1HandlerTx.ContractAddress),
						EntryPointSelector: feltToFieldElement(l1HandlerTx.EntryPointSelector),
						Calldata:           feltsToFieldElements(l1HandlerTx.CallData),
						Nonce:              feltToFieldElement(l1HandlerTx.Nonce),
						Version:            feltToFieldElement(l1HandlerTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_L1Handler{
					L1Handler: &grpcclient.L1HandlerTransactionReceipt{},
				},
			}
	}

	panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(transaction)))
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

func protobufHeaderToCoreBlock(header *grpcclient.BlockHeader, body *grpcclient.BlockBody) (*core.Block, error) {
	parentHash := fieldElementToFelt(header.ParentBlockHash)
	globalStateRoot := fieldElementToFelt(header.GlobalStateRoot)
	sequencerAddress := fieldElementToFelt(header.SequencerAddress)
	// TODO: these are validation
	// txCommitment := fieldElementToFelt(header.TransactionCommitment)
	// eventCommitment := fieldElementToFelt(header.EventCommitment)

	block := &core.Block{
		Header: &core.Header{
			Hash:             nil,
			ParentHash:       parentHash,
			Number:           header.BlockNumber,
			GlobalStateRoot:  globalStateRoot,
			SequencerAddress: sequencerAddress,
			TransactionCount: uint64(len(body.Transactions)),
			EventCount:       uint64(len(body.Receipts)), // many events per receipt
			Timestamp:        header.BlockTimestamp,
			ProtocolVersion:  "",
			ExtraData:        nil,
			EventsBloom:      nil,
		},
		Transactions: make([]core.Transaction, 0), // Assuming it's initialized as an empty slice
		Receipts:     make([]*core.TransactionReceipt, 0),
	}

	for i := uint32(0); i < header.TransactionCount; i++ {
		// Assuming you have a function to convert a transaction from protobuf to core
		transaction, receipt := protobufTransactionToCore(body.Transactions[i], body.Receipts[i])
		block.Transactions = append(block.Transactions, transaction)
		block.Receipts = append(block.Receipts, receipt)
	}

	return block, nil
}

func protobufCommonReceiptToCoreReceipt(commonReceipt *grpcclient.CommonTransactionReceiptProperties) *core.TransactionReceipt {
	receipt := &core.TransactionReceipt{
		Fee:             fieldElementToFelt(commonReceipt.GetActualFee()),
		Events:          coreEventFromProtobuf(commonReceipt.GetEvents()),
		L2ToL1Message:   coreL2ToL1MessageFromProtobuf(commonReceipt.GetMessagesSent()),
		TransactionHash: fieldElementToFelt(commonReceipt.GetTransactionHash()),
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

func protobufTransactionToCore(protoTx *grpcclient.Transaction, protoReceipt *grpcclient.Receipt) (core.Transaction, *core.TransactionReceipt) {
	switch tx := protoTx.GetTxn().(type) {
	case *grpcclient.Transaction_Deploy:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Deploy)

		deployTx := &core.DeployTransaction{
			ContractAddressSalt: fieldElementToFelt(tx.Deploy.GetContractAddressSalt()),
			ConstructorCallData: fieldElementsToFelts(tx.Deploy.GetConstructorCalldata()), // TODO: incomplete
			Version:             fieldElementToFelt(tx.Deploy.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Deploy.Common)
		return deployTx, receipt

	case *grpcclient.Transaction_DeployAccount:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_DeployAccount)

		deployAccountTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				ContractAddressSalt: fieldElementToFelt(tx.DeployAccount.GetContractAddressSalt()),
				ConstructorCallData: fieldElementsToFelts(tx.DeployAccount.GetConstructorCalldata()),
				ClassHash:           fieldElementToFelt(tx.DeployAccount.GetClassHash()),
				Version:             fieldElementToFelt(tx.DeployAccount.GetVersion()),
			},

			MaxFee:               fieldElementToFelt(tx.DeployAccount.GetMaxFee()),
			TransactionSignature: fieldElementsToFelts(tx.DeployAccount.GetSignature()),
			Nonce:                fieldElementToFelt(tx.DeployAccount.GetNonce()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.DeployAccount.Common)
		return deployAccountTx, receipt

	case *grpcclient.Transaction_Declare:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Declare)

		declareTx := &core.DeclareTransaction{
			SenderAddress:        fieldElementToFelt(tx.Declare.GetSenderAddress()),
			MaxFee:               fieldElementToFelt(tx.Declare.GetMaxFee()),
			TransactionSignature: fieldElementsToFelts(tx.Declare.GetSignature()),
			Nonce:                fieldElementToFelt(tx.Declare.GetNonce()),
			Version:              fieldElementToFelt(tx.Declare.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Declare.Common)
		return declareTx, receipt

	case *grpcclient.Transaction_Invoke:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Invoke)

		invokeTx := &core.InvokeTransaction{
			ContractAddress:      fieldElementToFelt(tx.Invoke.GetContractAddress()),
			EntryPointSelector:   fieldElementToFelt(tx.Invoke.GetEntryPointSelector()),
			CallData:             fieldElementsToFelts(tx.Invoke.GetCalldata()),
			TransactionSignature: fieldElementsToFelts(tx.Invoke.GetSignature()),
			MaxFee:               fieldElementToFelt(tx.Invoke.GetMaxFee()),
			Nonce:                fieldElementToFelt(tx.Invoke.GetNonce()),
			Version:              fieldElementToFelt(tx.Invoke.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Invoke.Common)
		return invokeTx, receipt

	case *grpcclient.Transaction_L1Handler:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_L1Handler)

		l1HandlerTx := &core.L1HandlerTransaction{
			ContractAddress:    fieldElementToFelt(tx.L1Handler.GetContractAddress()),
			EntryPointSelector: fieldElementToFelt(tx.L1Handler.GetEntryPointSelector()),
			CallData:           fieldElementsToFelts(tx.L1Handler.GetCalldata()),
			Nonce:              fieldElementToFelt(tx.L1Handler.GetNonce()),
			Version:            fieldElementToFelt(tx.L1Handler.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.L1Handler.Common)
		return l1HandlerTx, receipt

	default:
		panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(protoTx.GetTxn())))
	}
}
