package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/NethermindEth/juno/utils"
	"reflect"
)

func protobufTransactionToCore(protoTx *grpcclient.Transaction, protoReceipt *grpcclient.Receipt, network utils.Network) (core.Transaction, *core.TransactionReceipt, error) {
	switch tx := protoTx.GetTxn().(type) {
	case *grpcclient.Transaction_Deploy:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Deploy)

		coreTx := &core.DeployTransaction{
			ClassHash:           fieldElementToFelt(tx.Deploy.ClassHash),
			ContractAddress:     fieldElementToFelt(tx.Deploy.ContractAddress),
			TransactionHash:     fieldElementToFelt(tx.Deploy.GetHash()),
			ContractAddressSalt: fieldElementToFelt(tx.Deploy.GetContractAddressSalt()),
			ConstructorCallData: fieldElementsToFelts(tx.Deploy.GetConstructorCalldata()), // TODO: incomplete
			Version:             fieldElementToFelt(tx.Deploy.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Deploy.Common)
		return coreTx, receipt, nil

	case *grpcclient.Transaction_DeployAccount:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_DeployAccount)

		coreTx := &core.DeployAccountTransaction{

			DeployTransaction: core.DeployTransaction{
				TransactionHash:     fieldElementToFelt(tx.DeployAccount.GetHash()),
				ContractAddressSalt: fieldElementToFelt(tx.DeployAccount.GetContractAddressSalt()),
				ConstructorCallData: fieldElementsToFelts(tx.DeployAccount.GetConstructorCalldata()),
				ClassHash:           fieldElementToFelt(tx.DeployAccount.GetClassHash()),
				Version:             fieldElementToFelt(tx.DeployAccount.GetVersion()),
				ContractAddress:     fieldElementToFelt(tx.DeployAccount.GetContractAddress()),
			},

			MaxFee:               fieldElementToFelt(tx.DeployAccount.GetMaxFee()),
			TransactionSignature: fieldElementsToFelts(tx.DeployAccount.GetSignature()),
			Nonce:                fieldElementToFelt(tx.DeployAccount.GetNonce()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.DeployAccount.Common)
		return coreTx, receipt, nil

	case *grpcclient.Transaction_Declare:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Declare)

		coreTx := &core.DeclareTransaction{
			TransactionHash:      fieldElementToFelt(tx.Declare.Hash),
			SenderAddress:        fieldElementToFelt(tx.Declare.GetSenderAddress()),
			MaxFee:               fieldElementToFelt(tx.Declare.GetMaxFee()),
			TransactionSignature: fieldElementsToFelts(tx.Declare.GetSignature()),
			Nonce:                fieldElementToFelt(tx.Declare.GetNonce()),
			ClassHash:            fieldElementToFelt(tx.Declare.ContractClass.Hash),
			CompiledClassHash:    fieldElementToFelt(tx.Declare.CompiledClassHash),
			Version:              fieldElementToFelt(tx.Declare.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Declare.Common)
		return coreTx, receipt, nil

	case *grpcclient.Transaction_Invoke:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_Invoke)

		coreTx := &core.InvokeTransaction{
			TransactionHash:      fieldElementToFelt(tx.Invoke.GetHash()),
			SenderAddress:        fieldElementToFelt(tx.Invoke.GetSenderAddress()),
			ContractAddress:      fieldElementToFelt(tx.Invoke.GetContractAddress()),
			EntryPointSelector:   fieldElementToFelt(tx.Invoke.GetEntryPointSelector()),
			CallData:             fieldElementsToFelts(tx.Invoke.GetCalldata()),
			TransactionSignature: fieldElementsToFelts(tx.Invoke.GetSignature()),
			MaxFee:               fieldElementToFelt(tx.Invoke.GetMaxFee()),
			Nonce:                fieldElementToFelt(tx.Invoke.GetNonce()),
			Version:              fieldElementToFelt(tx.Invoke.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Invoke.Common)
		return coreTx, receipt, nil

	case *grpcclient.Transaction_L1Handler:
		txReceipt := protoReceipt.Receipt.(*grpcclient.Receipt_L1Handler)

		coreTx := &core.L1HandlerTransaction{
			TransactionHash:    fieldElementToFelt(tx.L1Handler.GetHash()),
			ContractAddress:    fieldElementToFelt(tx.L1Handler.GetContractAddress()),
			EntryPointSelector: fieldElementToFelt(tx.L1Handler.GetEntryPointSelector()),
			CallData:           fieldElementsToFelts(tx.L1Handler.GetCalldata()),
			Nonce:              fieldElementToFelt(tx.L1Handler.GetNonce()),
			Version:            fieldElementToFelt(tx.L1Handler.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.L1Handler.Common)
		receipt.L1ToL2Message = MapValueViaReflect[*core.L1ToL2Message](txReceipt.L1Handler.L1ToL2Message)
		return coreTx, receipt, nil

	default:
		panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(protoTx.GetTxn())))
	}
}

func (c *converter) coreTxToProtobufTx(transaction core.Transaction, receipt *core.TransactionReceipt) (*grpcclient.Transaction, *grpcclient.Receipt, error) {
	commonReceipt := &grpcclient.CommonTransactionReceiptProperties{
		TransactionHash:    feltToFieldElement(receipt.TransactionHash),
		ActualFee:          feltToFieldElement(receipt.Fee),
		MessagesSent:       coreL2ToL1MessageToProtobuf(receipt.L2ToL1Message),
		Events:             coreEventToProtobuf(receipt.Events),
		ExecutionResources: MapValueViaReflect[*grpcclient.CommonTransactionReceiptProperties_ExecutionResources](receipt.ExecutionResources),
	}

	if deployTx, ok := transaction.(*core.DeployTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Deploy{
					Deploy: &grpcclient.DeployTransaction{
						ClassHash:           feltToFieldElement(deployTx.ClassHash),
						ContractAddress:     feltToFieldElement(deployTx.ContractAddress),
						Hash:                feltToFieldElement(deployTx.TransactionHash),
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
			}, nil
	}

	if deployTx, ok := transaction.(*core.DeployAccountTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_DeployAccount{
					DeployAccount: &grpcclient.DeployAccountTransaction{
						Hash:                feltToFieldElement(deployTx.TransactionHash),
						ContractAddress:     feltToFieldElement(deployTx.ContractAddress),
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
			}, nil
	}

	if declareTx, ok := transaction.(*core.DeclareTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Declare{
					Declare: &grpcclient.DeclareTransaction{
						Hash: feltToFieldElement(declareTx.TransactionHash),
						ContractClass: &grpcclient.ContractClass{
							Hash: feltToFieldElement(declareTx.ClassHash),
						},
						SenderAddress:     feltToFieldElement(declareTx.SenderAddress),
						MaxFee:            feltToFieldElement(declareTx.MaxFee),
						Signature:         feltsToFieldElements(declareTx.TransactionSignature),
						Nonce:             feltToFieldElement(declareTx.Nonce),
						CompiledClassHash: feltToFieldElement(declareTx.CompiledClassHash),
						Version:           feltToFieldElement(declareTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_Declare{
					Declare: &grpcclient.DeclareTransactionReceipt{
						Common: commonReceipt,
					},
				},
			}, nil
	}

	if invokeTx, ok := transaction.(*core.InvokeTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_Invoke{
					Invoke: &grpcclient.InvokeTransaction{
						Hash:               feltToFieldElement(invokeTx.TransactionHash),
						SenderAddress:      feltToFieldElement(invokeTx.SenderAddress),
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
			}, nil
	}

	if l1HandlerTx, ok := transaction.(*core.L1HandlerTransaction); ok {
		return &grpcclient.Transaction{
				Txn: &grpcclient.Transaction_L1Handler{
					L1Handler: &grpcclient.L1HandlerTransaction{
						Hash:               feltToFieldElement(l1HandlerTx.TransactionHash),
						ContractAddress:    feltToFieldElement(l1HandlerTx.ContractAddress),
						EntryPointSelector: feltToFieldElement(l1HandlerTx.EntryPointSelector),
						Calldata:           feltsToFieldElements(l1HandlerTx.CallData),
						Nonce:              feltToFieldElement(l1HandlerTx.Nonce),
						Version:            feltToFieldElement(l1HandlerTx.Version),
					},
				},
			}, &grpcclient.Receipt{
				Receipt: &grpcclient.Receipt_L1Handler{
					L1Handler: &grpcclient.L1HandlerTransactionReceipt{
						Common:        commonReceipt,
						L1ToL2Message: MapValueViaReflect[*grpcclient.L1HandlerTransactionReceipt_L1ToL2Message](receipt.L1ToL2Message),
					},
				},
			}, nil
	}

	panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(transaction)))
}

func coreClassToGrpcClass(theclass *core.DeclaredClass) *grpcclient.ContractClass {
	switch class := theclass.Class.(type) {
	case *core.Cairo0Class:
		abistr, err := class.Abi.MarshalJSON()
		if err != nil {
			panic(err)
		}

		externals := make([]*grpcclient.ContractClass_EntryPoint, len(class.Externals))
		for i, external := range class.Externals {
			externals[i] = &grpcclient.ContractClass_EntryPoint{
				Selector: feltToFieldElement(external.Selector),
				Offset:   feltToFieldElement(external.Offset),
			}
		}

		handlers := make([]*grpcclient.ContractClass_EntryPoint, len(class.L1Handlers))
		for i, external := range class.L1Handlers {
			handlers[i] = &grpcclient.ContractClass_EntryPoint{
				Selector: feltToFieldElement(external.Selector),
				Offset:   feltToFieldElement(external.Offset),
			}
		}

		// TODO: What about constructor?
		return &grpcclient.ContractClass{
			ExternalEntryPoints:  externals,
			L1HandlerEntryPoints: handlers,
			Program:              class.Program,
			Abi:                  string(abistr),
		}

	// TODO: case *core.Cairo1Class:
	default:
		panic(fmt.Sprintf("Unsupported class type %s", reflect.TypeOf(theclass.Class)))
	}
}
