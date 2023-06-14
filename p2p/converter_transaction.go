package p2p

import (
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/p2pproto"
	"github.com/NethermindEth/juno/utils"
	"github.com/pkg/errors"
	"reflect"
)

func protobufTransactionToCore(protoTx *p2pproto.Transaction, protoReceipt *p2pproto.Receipt, network utils.Network) (core.Transaction, *core.TransactionReceipt, *felt.Felt, core.Class, error) {
	switch tx := protoTx.GetTxn().(type) {
	case *p2pproto.Transaction_Deploy:
		txReceipt := protoReceipt.Receipt.(*p2pproto.Receipt_Deploy)

		classHash, class, err := protobufClassToCoreClass(tx.Deploy.ContractClass)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "error deserializing class")
		}

		coreTx := &core.DeployTransaction{
			ClassHash:           classHash,
			ContractAddress:     fieldElementToFelt(tx.Deploy.ContractAddress),
			TransactionHash:     fieldElementToFelt(tx.Deploy.GetHash()),
			ContractAddressSalt: fieldElementToFelt(tx.Deploy.GetContractAddressSalt()),
			ConstructorCallData: fieldElementsToFelts(tx.Deploy.GetConstructorCalldata()), // TODO: incomplete
			Version:             fieldElementToFelt(tx.Deploy.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Deploy.Common)
		return coreTx, receipt, classHash, class, nil

	case *p2pproto.Transaction_DeployAccount:
		txReceipt := protoReceipt.Receipt.(*p2pproto.Receipt_DeployAccount)

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
		return coreTx, receipt, nil, nil, nil

	case *p2pproto.Transaction_Declare:
		txReceipt := protoReceipt.Receipt.(*p2pproto.Receipt_Declare)

		classHash, class, err := protobufClassToCoreClass(tx.Declare.ContractClass)
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "error deserializing class")
		}

		coreTx := &core.DeclareTransaction{
			TransactionHash:      fieldElementToFelt(tx.Declare.Hash),
			SenderAddress:        fieldElementToFelt(tx.Declare.GetSenderAddress()),
			MaxFee:               fieldElementToFelt(tx.Declare.GetMaxFee()),
			TransactionSignature: fieldElementsToFelts(tx.Declare.GetSignature()),
			Nonce:                fieldElementToFelt(tx.Declare.GetNonce()),
			ClassHash:            classHash,
			CompiledClassHash:    fieldElementToFelt(tx.Declare.CompiledClassHash),
			Version:              fieldElementToFelt(tx.Declare.GetVersion()),
		}

		receipt := protobufCommonReceiptToCoreReceipt(txReceipt.Declare.Common)
		return coreTx, receipt, classHash, class, nil

	case *p2pproto.Transaction_Invoke:
		txReceipt := protoReceipt.Receipt.(*p2pproto.Receipt_Invoke)

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
		return coreTx, receipt, nil, nil, nil

	case *p2pproto.Transaction_L1Handler:
		txReceipt := protoReceipt.Receipt.(*p2pproto.Receipt_L1Handler)

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
		return coreTx, receipt, nil, nil, nil

	default:
		panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(protoTx.GetTxn())))
	}
}

func (c *converter) coreTxToProtobufTx(transaction core.Transaction, receipt *core.TransactionReceipt) (*p2pproto.Transaction, *p2pproto.Receipt, error) {
	commonReceipt := &p2pproto.CommonTransactionReceiptProperties{
		TransactionHash:    feltToFieldElement(receipt.TransactionHash),
		ActualFee:          feltToFieldElement(receipt.Fee),
		MessagesSent:       coreL2ToL1MessageToProtobuf(receipt.L2ToL1Message),
		Events:             coreEventToProtobuf(receipt.Events),
		ExecutionResources: MapValueViaReflect[*p2pproto.CommonTransactionReceiptProperties_ExecutionResources](receipt.ExecutionResources),
	}

	if deployTx, ok := transaction.(*core.DeployTransaction); ok {
		stateReader, closer, err := c.blockchain.HeadState()
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to fetch head state")
		}
		defer closer()

		coreClass, err := stateReader.Class(deployTx.ClassHash)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to fetch class")
		}

		protobufClass, err := coreClassToProtobufClass(deployTx.ClassHash, coreClass)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to convert core class to protobuf")
		}

		return &p2pproto.Transaction{
				Txn: &p2pproto.Transaction_Deploy{
					Deploy: &p2pproto.DeployTransaction{
						ContractClass:       protobufClass,
						ContractAddress:     feltToFieldElement(deployTx.ContractAddress),
						Hash:                feltToFieldElement(deployTx.TransactionHash),
						ContractAddressSalt: feltToFieldElement(deployTx.ContractAddressSalt),
						ConstructorCalldata: feltsToFieldElements(deployTx.ConstructorCallData),
						Version:             feltToFieldElement(deployTx.Version),
					},
				},
			}, &p2pproto.Receipt{
				Receipt: &p2pproto.Receipt_Deploy{
					Deploy: &p2pproto.DeployTransactionReceipt{
						Common:          commonReceipt,
						ContractAddress: feltToFieldElement(deployTx.ContractAddress),
					},
				},
			}, nil
	}

	if deployTx, ok := transaction.(*core.DeployAccountTransaction); ok {
		return &p2pproto.Transaction{
				Txn: &p2pproto.Transaction_DeployAccount{
					DeployAccount: &p2pproto.DeployAccountTransaction{
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
			}, &p2pproto.Receipt{
				Receipt: &p2pproto.Receipt_DeployAccount{
					DeployAccount: &p2pproto.DeployAccountTransactionReceipt{
						Common:          commonReceipt,
						ContractAddress: feltToFieldElement(deployTx.ContractAddress),
					},
				},
			}, nil
	}

	if declareTx, ok := transaction.(*core.DeclareTransaction); ok {
		stateReader, closer, err := c.blockchain.HeadState()
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to fetch head state")
		}
		defer closer()

		coreClass, err := stateReader.Class(declareTx.ClassHash)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to fetch class")
		}

		protobufClass, err := coreClassToProtobufClass(declareTx.ClassHash, coreClass)
		if err != nil {
			return nil, nil, errors.Wrap(err, "unable to convert core class to protobuf")
		}

		return &p2pproto.Transaction{
				Txn: &p2pproto.Transaction_Declare{
					Declare: &p2pproto.DeclareTransaction{
						Hash:              feltToFieldElement(declareTx.TransactionHash),
						ContractClass:     protobufClass,
						SenderAddress:     feltToFieldElement(declareTx.SenderAddress),
						MaxFee:            feltToFieldElement(declareTx.MaxFee),
						Signature:         feltsToFieldElements(declareTx.TransactionSignature),
						Nonce:             feltToFieldElement(declareTx.Nonce),
						CompiledClassHash: feltToFieldElement(declareTx.CompiledClassHash),
						Version:           feltToFieldElement(declareTx.Version),
					},
				},
			}, &p2pproto.Receipt{
				Receipt: &p2pproto.Receipt_Declare{
					Declare: &p2pproto.DeclareTransactionReceipt{
						Common: commonReceipt,
					},
				},
			}, nil
	}

	if invokeTx, ok := transaction.(*core.InvokeTransaction); ok {
		return &p2pproto.Transaction{
				Txn: &p2pproto.Transaction_Invoke{
					Invoke: &p2pproto.InvokeTransaction{
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
			}, &p2pproto.Receipt{
				Receipt: &p2pproto.Receipt_Invoke{
					Invoke: &p2pproto.InvokeTransactionReceipt{
						Common: commonReceipt,
					},
				},
			}, nil
	}

	if l1HandlerTx, ok := transaction.(*core.L1HandlerTransaction); ok {
		return &p2pproto.Transaction{
				Txn: &p2pproto.Transaction_L1Handler{
					L1Handler: &p2pproto.L1HandlerTransaction{
						Hash:               feltToFieldElement(l1HandlerTx.TransactionHash),
						ContractAddress:    feltToFieldElement(l1HandlerTx.ContractAddress),
						EntryPointSelector: feltToFieldElement(l1HandlerTx.EntryPointSelector),
						Calldata:           feltsToFieldElements(l1HandlerTx.CallData),
						Nonce:              feltToFieldElement(l1HandlerTx.Nonce),
						Version:            feltToFieldElement(l1HandlerTx.Version),
					},
				},
			}, &p2pproto.Receipt{
				Receipt: &p2pproto.Receipt_L1Handler{
					L1Handler: &p2pproto.L1HandlerTransactionReceipt{
						Common:        commonReceipt,
						L1ToL2Message: MapValueViaReflect[*p2pproto.L1HandlerTransactionReceipt_L1ToL2Message](receipt.L1ToL2Message),
					},
				},
			}, nil
	}

	panic(fmt.Sprintf("Unknown transaction type %s", reflect.TypeOf(transaction)))
}

func coreClassToProtobufClass(hash *felt.Felt, theclass *core.DeclaredClass) (*p2pproto.ContractClass, error) {
	switch class := theclass.Class.(type) {
	case *core.Cairo0Class:
		abistr, err := class.Abi.MarshalJSON()
		if err != nil {
			return nil, err
		}

		constructors := MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Constructors)
		externals := MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.Externals)
		handlers := MapValueViaReflect[[]*p2pproto.Cairo0Class_EntryPoint](class.L1Handlers)

		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo0{
				Cairo0: &p2pproto.Cairo0Class{
					ConstructorEntryPoints: constructors,
					ExternalEntryPoints:    externals,
					L1HandlerEntryPoints:   handlers,
					Program:                class.Program,
					Abi:                    string(abistr),
					Hash:                   feltToFieldElement(hash),
				},
			},
		}, nil
	case *core.Cairo1Class:
		constructors := MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.Constructor)
		externals := MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.External)
		handlers := MapValueViaReflect[[]*p2pproto.Cairo1Class_EntryPoint](class.EntryPoints.L1Handler)
		program := feltsToFieldElements(class.Program)

		return &p2pproto.ContractClass{
			Class: &p2pproto.ContractClass_Cairo1{
				Cairo1: &p2pproto.Cairo1Class{
					ConstructorEntryPoints: constructors,
					ExternalEntryPoints:    externals,
					L1HandlerEntryPoints:   handlers,
					Program:                program,
					ProgramHash:            feltToFieldElement(class.ProgramHash),
					Abi:                    class.Abi,
					Hash:                   feltToFieldElement(hash),
				},
			},
		}, nil
	default:
		panic(fmt.Sprintf("Unsupported class type %s", reflect.TypeOf(theclass.Class)))
	}
}

func protobufClassToCoreClass(class *p2pproto.ContractClass) (*felt.Felt, core.Class, error) {
	switch v := class.Class.(type) {
	case *p2pproto.ContractClass_Cairo0:
		hash := fieldElementToFelt(v.Cairo0.Hash)

		abiraw := json.RawMessage{}
		err := json.Unmarshal([]byte(v.Cairo0.Abi), &abiraw)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error unmarshalling cairo0 abi")
		}

		return hash, &core.Cairo0Class{
			Abi:          abiraw,
			Externals:    MapValueViaReflect[[]core.EntryPoint](v.Cairo0.ExternalEntryPoints),
			L1Handlers:   MapValueViaReflect[[]core.EntryPoint](v.Cairo0.L1HandlerEntryPoints),
			Constructors: MapValueViaReflect[[]core.EntryPoint](v.Cairo0.ConstructorEntryPoints),
			Program:      v.Cairo0.Program,
		}, nil
	case *p2pproto.ContractClass_Cairo1:
		coreClass := &core.Cairo1Class{
			Abi:     v.Cairo1.Abi,
			Program: fieldElementsToFelts(v.Cairo1.Program),
		}
		coreClass.EntryPoints.Constructor = MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.ConstructorEntryPoints)
		coreClass.EntryPoints.External = MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.ExternalEntryPoints)
		coreClass.EntryPoints.L1Handler = MapValueViaReflect[[]core.SierraEntryPoint](v.Cairo1.L1HandlerEntryPoints)

		hash := coreClass.Hash()
		return hash, coreClass, nil
	}

	return nil, nil, errors.Errorf("unknown class type %T", class.Class)
}
