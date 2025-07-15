package sync

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	common "github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/receipt"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	transaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

// These adapters are only used in this test. They allow us to use the starknet test data to
// build blocks that we query from peers over p2p.

// Todo: move to utils

func toHash2(felt *felt.Felt) *common.Hash {
	feltBytes := felt.Bytes()
	return &common.Hash{Elements: feltBytes[:]}
}

func toAddress2(felt *felt.Felt) *common.Address {
	feltBytes := felt.Bytes()
	return &common.Address{Elements: feltBytes[:]}
}

func toFelt252(felt *felt.Felt) *common.Felt252 {
	feltBytes := felt.Bytes()
	return &common.Felt252{Elements: feltBytes[:]}
}

func toFelt252Slice(felts []*felt.Felt) []*common.Felt252 {
	if felts == nil {
		return nil
	}
	out := make([]*common.Felt252, len(felts))
	for i, f := range felts {
		out[i] = toFelt252(f)
	}
	return out
}

func toUint1282(f *felt.Felt) *common.Uint128 {
	// bits represents value in little endian byte order
	// i.e. first is least significant byte
	bits := f.Bits()
	return &common.Uint128{
		Low:  bits[0],
		High: bits[1],
	}
}

// toAccountSignature converts a slice of *felt.Felt into a protobuf AccountSignature.
func toAccountSignature(parts *[]*felt.Felt) *transaction.AccountSignature {
	return &transaction.AccountSignature{
		Parts: toFelt252Slice(*parts),
	}
}

// adaptTransactionInBlock converts your domain Transaction into the protobuf TransactionInBlock.
func adaptTransactionInBlock(tx *starknet.Transaction) (*synctransaction.TransactionInBlock, error) {

	pb := &synctransaction.TransactionInBlock{
		TransactionHash: toHash2(tx.Hash),
	}

	// Tdoo: implement other txn types
	switch tx.Type {
	case starknet.TxnInvoke:
		pb.Txn = &synctransaction.TransactionInBlock_InvokeV1_{
			InvokeV1: &synctransaction.TransactionInBlock_InvokeV1{
				Sender:    toAddress2(tx.SenderAddress),
				MaxFee:    toFelt252(tx.MaxFee),
				Signature: toAccountSignature(tx.Signature),
				Calldata:  toFelt252Slice(*tx.CallData),
				Nonce:     toFelt252(tx.Nonce),
			},
		}

	case starknet.TxnDeclare:
		pb.Txn = &synctransaction.TransactionInBlock_DeclareV0{
			DeclareV0: &synctransaction.TransactionInBlock_DeclareV0WithoutClass{
				Sender:    toAddress2(tx.SenderAddress),
				MaxFee:    toFelt252(tx.MaxFee),
				Signature: toAccountSignature(tx.Signature),
				ClassHash: toHash2(tx.ClassHash),
			},
		}

	case starknet.TxnDeployAccount:
		pb.Txn = &synctransaction.TransactionInBlock_Deploy_{
			Deploy: &synctransaction.TransactionInBlock_Deploy{
				ClassHash:   toHash2(tx.ClassHash),
				AddressSalt: toFelt252(tx.ClassHash),
				Calldata:    toFelt252Slice(*tx.CallData),
				Version:     uint32(tx.Version.Uint64()),
			},
		}
	default:
		return nil, errors.New("unknown txn type")
	}

	return pb, nil
}

func adaptReceiptCommon(r *starknet.TransactionReceipt) *receipt.Receipt_Common {

	// map execution resources
	execRes := &receipt.Receipt_ExecutionResources{
		Steps:       r.ExecutionResources.Steps,
		MemoryHoles: r.ExecutionResources.MemoryHoles,
	}

	return &receipt.Receipt_Common{
		ActualFee:          toFelt252(r.ActualFee),
		PriceUnit:          receipt.PriceUnit(r.PriceUnit),
		ExecutionResources: execRes,
		RevertReason:       &r.RevertError,
	}
}

// adaptReceipt converts a domain Receipt into its protobuf Receipt.
func adaptReceipt(r *starknet.TransactionReceipt) (*receipt.Receipt, error) {
	common := adaptReceiptCommon(r)

	pb := &receipt.Receipt{}

	switch r.Type {
	case starknet.ReceiptType_Invoke:
		pb.Type = &receipt.Receipt_Invoke_{
			Invoke: &receipt.Receipt_Invoke{
				Common: common,
			},
		}
	case starknet.ReceiptType_Declare:
		pb.Type = &receipt.Receipt_Declare_{
			Declare: &receipt.Receipt_Declare{
				Common: common,
			},
		}
	case starknet.ReceiptType_DeployAccount:
		pb.Type = &receipt.Receipt_DeployAccount_{
			DeployAccount: &receipt.Receipt_DeployAccount{
				Common: common,
			},
		}
	default:
		return nil, fmt.Errorf("adaptReceipt: unknown receipt type %q", r.Type)
	}

	return pb, nil
}
