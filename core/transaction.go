package core

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type Event struct {
	Data []*felt.Felt
	From *felt.Felt
	Keys []*felt.Felt
}

type L1ToL2Message struct {
	From     common.Address
	Nonce    *felt.Felt
	Payload  []*felt.Felt
	Selector *felt.Felt
	To       *felt.Felt
}

type L2ToL1Message struct {
	From    *felt.Felt
	Payload []*felt.Felt
	To      common.Address
}

type ExecutionResources struct {
	BuiltinInstanceCounter BuiltinInstanceCounter
	MemoryHoles            uint64
	Steps                  uint64
}

type BuiltinInstanceCounter struct {
	Bitwise    uint64
	EcOp       uint64
	Ecsda      uint64
	Output     uint64
	Pedersen   uint64
	RangeCheck uint64
}

// Todo: Having both TransactionType and DeployTransaction,
// InvokeTransaction and DeclareTransaction is redundant
type TransactionType int

const (
	Declare TransactionType = iota
	Deploy
	DeployAccount
	Invoke
	L1Handler
)

func init() {
	err := encoder.RegisterType(reflect.TypeOf(DeclareTransaction{}))
	if err != nil {
		panic(err)
	}
	err = encoder.RegisterType(reflect.TypeOf(DeployTransaction{}))
	if err != nil {
		panic(err)
	}
	err = encoder.RegisterType(reflect.TypeOf(InvokeTransaction{}))
	if err != nil {
		panic(err)
	}
}

type TransactionReceipt struct {
	ActualFee          *felt.Felt
	Events             []*Event
	ExecutionResources *ExecutionResources
	L1ToL2Message      *L1ToL2Message
	L2ToL1Message      []*L2ToL1Message
	Signatures         []*felt.Felt
	TransactionHash    *felt.Felt
	TransactionIndex   *big.Int
	// Todo: Do we need to have this?
	Type TransactionType
}

type Transaction interface {
	// Todo: Add Hash as a field to all the Transaction Objects
	Hash(utils.Network) (*felt.Felt, error)
}

type DeployTransaction struct {
	// A random number used to distinguish between different instances of the contract.
	ContractAddressSalt *felt.Felt
	// The address of the contract.
	ContractAddress *felt.Felt
	// The class that defines the contract’s functionality.
	ClassHash *felt.Felt
	// The arguments passed to the constructor during deployment.
	ConstructorCallData []*felt.Felt
	// Who invoked the deployment. Set to 0 (in future: the deploying account contract).
	CallerAddress *felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	//
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *felt.Felt
}

func (d *DeployTransaction) Hash(network utils.Network) (*felt.Felt, error) {
	snKeccakConstructor, err := crypto.StarkNetKeccak([]byte("constructor"))
	if err != nil {
		return nil, err
	}
	return crypto.PedersenArray(
		new(felt.Felt).SetBytes([]byte("deploy")),
		d.Version,
		d.ContractAddress,
		snKeccakConstructor,
		crypto.PedersenArray(d.ConstructorCallData...),
		new(felt.Felt),
		network.ChainId(),
	), nil
}

type InvokeTransaction struct {
	// Version 0 fields
	// The address of the contract invoked by this transaction.
	ContractAddress *felt.Felt
	// The encoding of the selector for the function invoked (the entry point in the contract)
	EntryPointSelector *felt.Felt

	// Version 1 fields
	// The address of the sender of this transaction.
	SenderAddress *felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The arguments that are passed to the validated and execute functions.
	CallData []*felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	Signature []*felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction
	MaxFee *felt.Felt
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	Version *felt.Felt
}

func (i *InvokeTransaction) Hash(network utils.Network) (*felt.Felt, error) {
	invokeFelt := new(felt.Felt).SetBytes([]byte("invoke"))
	if i.Version.IsZero() {
		return crypto.PedersenArray(
			invokeFelt,
			i.ContractAddress,
			i.EntryPointSelector,
			crypto.PedersenArray(i.CallData...),
			network.ChainId(),
		), nil
	} else if i.Version.IsOne() {
		return crypto.PedersenArray(
			invokeFelt,
			i.Version,
			i.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(i.CallData...),
			i.MaxFee,
			network.ChainId(),
			i.Nonce,
		), nil
	}
	return nil, errors.New("invalid transaction version")
}

type DeclareTransaction struct {
	// The class hash
	ClassHash *felt.Felt
	// The address of the account initiating the transaction.
	SenderAddress *felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction.
	MaxFee *felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	Signature []*felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *felt.Felt
}

func (d *DeclareTransaction) Hash(network utils.Network) (*felt.Felt, error) {
	declareFelt := new(felt.Felt).SetBytes([]byte("declare"))
	if d.Version.IsZero() {
		return crypto.PedersenArray(
			declareFelt,
			d.Version,
			d.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(make([]*felt.Felt, 0)...),
			d.MaxFee,
			network.ChainId(),
			d.ClassHash,
		), nil
	} else if d.Version.IsOne() {
		return crypto.PedersenArray(
			declareFelt,
			d.Version,
			d.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(d.ClassHash),
			d.MaxFee,
			network.ChainId(),
			d.Nonce,
		), nil
	}
	return nil, errors.New("invalid transaction version")
}

type L1HandlerTransaction struct {
	// todo
}

func (h *L1HandlerTransaction) Hash(network utils.Network) (*felt.Felt, error) {
	panic("not implemented")
}

type DeployAccountTransaction struct {
	// todo
}

func (d *DeployAccountTransaction) Hash(network utils.Network) (*felt.Felt, error) {
	panic("not implemented")
}

// TransactionCommitment is the root of a height 64 binary Merkle Patricia tree of the
// transaction hashes and signatures in a block.
func TransactionCommitment(receipts []*TransactionReceipt) (*felt.Felt, error) {
	var transactionCommitment *felt.Felt
	return transactionCommitment, trie.RunOnTempTrie(64, func(trie *trie.Trie) error {
		for i, receipt := range receipts {
			signaturesHash := crypto.PedersenArray()
			if receipt.Type == Invoke {
				signaturesHash = crypto.PedersenArray(receipt.Signatures...)
			}
			transactionAndSignatureHash := crypto.Pedersen(receipt.TransactionHash, signaturesHash)
			if _, err := trie.Put(new(felt.Felt).SetUint64(uint64(i)), transactionAndSignatureHash); err != nil {
				return err
			}
		}
		root, err := trie.Root()
		if err != nil {
			return err
		}
		transactionCommitment = root
		return nil
	})
}

// EventCommitmentAndCount computes the event commitment and event count for a block.
func EventCommitmentAndCount(receipts []*TransactionReceipt) (*felt.Felt, uint64, error) {
	var eventCommitment *felt.Felt // root of a height 64 binary Merkle Patricia tree of the events in a block.
	var eventCount uint64          // number of events in a block.
	return eventCommitment, eventCount, trie.RunOnTempTrie(64, func(trie *trie.Trie) error {
		for _, receipt := range receipts {
			for _, event := range receipt.Events {
				eventHash := crypto.PedersenArray(
					event.From,
					crypto.PedersenArray(event.Keys...),
					crypto.PedersenArray(event.Data...),
				)

				if _, err := trie.Put(new(felt.Felt).SetUint64(eventCount), eventHash); err != nil {
					return err
				}
				eventCount++
			}
		}
		root, err := trie.Root()
		if err != nil {
			return err
		}
		eventCommitment = root
		return nil
	})
}
