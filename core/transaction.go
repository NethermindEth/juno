package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fxamacker/cbor/v2"
	"github.com/sourcegraph/conc/pool"
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
	Pedersen   uint64
	RangeCheck uint64
	Bitwise    uint64
	Output     uint64
	Ecsda      uint64
	EcOp       uint64
	Keccak     uint64
	Poseidon   uint64
}

type TransactionReceipt struct {
	Fee                *felt.Felt
	Events             []*Event
	ExecutionResources *ExecutionResources
	L1ToL2Message      *L1ToL2Message
	L2ToL1Message      []*L2ToL1Message
	TransactionHash    *felt.Felt
	Reverted           bool
	RevertReason       string
}

type Transaction interface {
	Hash() *felt.Felt
	Signature() []*felt.Felt
}

var (
	_            Transaction = (*DeployTransaction)(nil)
	_            Transaction = (*DeployAccountTransaction)(nil)
	_            Transaction = (*DeclareTransaction)(nil)
	_            Transaction = (*InvokeTransaction)(nil)
	_            Transaction = (*L1HandlerTransaction)(nil)
	queryVersion             = new(felt.Felt).Exp(new(felt.Felt).SetUint64(2), new(big.Int).SetUint64(queryBit))
)

const (
	// Calculated at https://hur.st/bloomfilter/?n=1000&p=&m=8192&k=
	// provides 1 in 51 possibility of false positives for approximately 1000 elements
	eventsBloomLength    = 8192
	eventsBloomHashFuncs = 6
	queryBit             = 128
)

// Keep in mind that this is used as a storage type, make sure you migrate
// the DB if you change the underlying type
type TransactionVersion felt.Felt

func (v *TransactionVersion) SetUint64(u64 uint64) *TransactionVersion {
	v.AsFelt().SetUint64(u64)
	return v
}

func (v *TransactionVersion) HasQueryBit() bool {
	// if versionWithoutQueryBit >= queryBit
	return v.AsFelt().Cmp(queryVersion) != -1
}

// Is compares the version (without query bit) with the given value
func (v *TransactionVersion) Is(u64 uint64) bool {
	var tmpV TransactionVersion
	tmpV.SetUint64(u64)
	return tmpV == v.WithoutQueryBit()
}

func (v *TransactionVersion) WithoutQueryBit() TransactionVersion {
	vFelt := felt.Felt(*v)
	if v.HasQueryBit() {
		vFelt.Sub(&vFelt, queryVersion)
	}
	return TransactionVersion(vFelt)
}

func (v *TransactionVersion) String() string {
	return v.AsFelt().String()
}

func (v *TransactionVersion) AsFelt() *felt.Felt {
	return (*felt.Felt)(v)
}

func (v *TransactionVersion) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(v.AsFelt())
}

func (v *TransactionVersion) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, v.AsFelt())
}

type DeployTransaction struct {
	TransactionHash *felt.Felt
	// A random number used to distinguish between different instances of the contract.
	ContractAddressSalt *felt.Felt
	// The address of the contract.
	ContractAddress *felt.Felt
	// The hash of the class which defines the contract’s functionality.
	ClassHash *felt.Felt
	// The arguments passed to the constructor during deployment.
	ConstructorCallData []*felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	//
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of Starknet.
	Version *TransactionVersion
}

func (d *DeployTransaction) Hash() *felt.Felt {
	return d.TransactionHash
}

func (d *DeployTransaction) Signature() []*felt.Felt {
	return make([]*felt.Felt, 0)
}

type DeployAccountTransaction struct {
	DeployTransaction
	// The maximum fee that the sender is willing to pay for the transaction.
	MaxFee *felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	TransactionSignature []*felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
}

func (d *DeployAccountTransaction) Hash() *felt.Felt {
	return d.TransactionHash
}

func (d *DeployAccountTransaction) Signature() []*felt.Felt {
	return d.TransactionSignature
}

type InvokeTransaction struct {
	TransactionHash *felt.Felt
	// The arguments that are passed to the validated and execute functions.
	CallData []*felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	TransactionSignature []*felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction
	MaxFee *felt.Felt
	// The address of the contract invoked by this transaction.
	ContractAddress *felt.Felt
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	Version *TransactionVersion

	// Version 0 fields
	// The encoding of the selector for the function invoked (the entry point in the contract)
	EntryPointSelector *felt.Felt

	// Version 1 fields
	// The transaction nonce.
	Nonce *felt.Felt
	// The address of the sender of this transaction
	SenderAddress *felt.Felt
}

func (i *InvokeTransaction) Hash() *felt.Felt {
	return i.TransactionHash
}

func (i *InvokeTransaction) Signature() []*felt.Felt {
	return i.TransactionSignature
}

type DeclareTransaction struct {
	TransactionHash *felt.Felt
	// The class hash
	ClassHash *felt.Felt
	// The address of the account initiating the transaction.
	SenderAddress *felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction.
	MaxFee *felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	TransactionSignature []*felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of Starknet.
	Version *TransactionVersion

	// Version 2 fields
	CompiledClassHash *felt.Felt
}

func (d *DeclareTransaction) Hash() *felt.Felt {
	return d.TransactionHash
}

func (d *DeclareTransaction) Signature() []*felt.Felt {
	return d.TransactionSignature
}

type L1HandlerTransaction struct {
	TransactionHash *felt.Felt
	// The address of the contract.
	ContractAddress *felt.Felt
	// The encoding of the selector for the function invoked (the entry point in the contract)
	EntryPointSelector *felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The arguments that are passed to the validated and execute functions.
	CallData []*felt.Felt
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	Version *TransactionVersion
}

func (l *L1HandlerTransaction) Hash() *felt.Felt {
	return l.TransactionHash
}

func (l *L1HandlerTransaction) Signature() []*felt.Felt {
	return make([]*felt.Felt, 0)
}

func TransactionHash(transaction Transaction, n utils.Network) (*felt.Felt, error) {
	switch t := transaction.(type) {
	case *DeclareTransaction:
		return declareTransactionHash(t, n)
	case *InvokeTransaction:
		return invokeTransactionHash(t, n)
	case *DeployTransaction:
		// deploy transactions are deprecated after re-genesis therefore we don't verify
		// transaction hash
		return t.TransactionHash, nil
	case *L1HandlerTransaction:
		return l1HandlerTransactionHash(t, n)
	case *DeployAccountTransaction:
		return deployAccountTransactionHash(t, n)
	default:
		return nil, errors.New("unknown transaction")
	}
}

var (
	invokeFelt        = new(felt.Felt).SetBytes([]byte("invoke"))
	declareFelt       = new(felt.Felt).SetBytes([]byte("declare"))
	l1HandlerFelt     = new(felt.Felt).SetBytes([]byte("l1_handler"))
	deployAccountFelt = new(felt.Felt).SetBytes([]byte("deploy_account"))
)

func errInvalidTransactionVersion(t Transaction, version *TransactionVersion) error {
	return fmt.Errorf("invalid Transaction (type: %T) version: %s", t, version)
}

func invokeTransactionHash(i *InvokeTransaction, n utils.Network) (*felt.Felt, error) {
	switch {
	case i.Version.Is(0):
		return crypto.PedersenArray(
			invokeFelt,
			i.Version.AsFelt(),
			i.ContractAddress,
			i.EntryPointSelector,
			crypto.PedersenArray(i.CallData...),
			i.MaxFee,
			n.ChainID(),
		), nil
	case i.Version.Is(1):
		return crypto.PedersenArray(
			invokeFelt,
			i.Version.AsFelt(),
			i.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(i.CallData...),
			i.MaxFee,
			n.ChainID(),
			i.Nonce,
		), nil
	default:
		return nil, errInvalidTransactionVersion(i, i.Version)
	}
}

func declareTransactionHash(d *DeclareTransaction, n utils.Network) (*felt.Felt, error) {
	switch {
	case d.Version.Is(0):
		// Due to inconsistencies in version 0 hash calculation we don't verify the hash
		return d.TransactionHash, nil
	case d.Version.Is(1):
		return crypto.PedersenArray(
			declareFelt,
			d.Version.AsFelt(),
			d.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(d.ClassHash),
			d.MaxFee,
			n.ChainID(),
			d.Nonce,
		), nil
	case d.Version.Is(2):
		return crypto.PedersenArray(
			declareFelt,
			d.Version.AsFelt(),
			d.SenderAddress,
			&felt.Zero,
			crypto.PedersenArray(d.ClassHash),
			d.MaxFee,
			n.ChainID(),
			d.Nonce,
			d.CompiledClassHash,
		), nil

	default:
		return nil, errInvalidTransactionVersion(d, d.Version)
	}
}

func l1HandlerTransactionHash(l *L1HandlerTransaction, n utils.Network) (*felt.Felt, error) {
	switch {
	case l.Version.Is(0):
		// There are some l1 handler transaction which do not return a nonce and for some random
		// transaction the following hash fails.
		if l.Nonce == nil {
			return l.TransactionHash, nil
		}
		return crypto.PedersenArray(
			l1HandlerFelt,
			l.Version.AsFelt(),
			l.ContractAddress,
			l.EntryPointSelector,
			crypto.PedersenArray(l.CallData...),
			&felt.Zero,
			n.ChainID(),
			l.Nonce,
		), nil
	default:
		return nil, errInvalidTransactionVersion(l, l.Version)
	}
}

func deployAccountTransactionHash(d *DeployAccountTransaction, n utils.Network) (*felt.Felt, error) {
	callData := []*felt.Felt{d.ClassHash, d.ContractAddressSalt}
	callData = append(callData, d.ConstructorCallData...)
	// There is no version 0 for deploy account
	if d.Version.Is(1) {
		return crypto.PedersenArray(
			deployAccountFelt,
			d.Version.AsFelt(),
			d.ContractAddress,
			&felt.Zero,
			crypto.PedersenArray(callData...),
			d.MaxFee,
			n.ChainID(),
			d.Nonce,
		), nil
	}
	return nil, errInvalidTransactionVersion(d, d.Version)
}

func VerifyTransactions(txs []Transaction, n utils.Network, protocolVersion string) error {
	blockVersion, err := ParseBlockVersion(protocolVersion)
	if err != nil {
		return err
	}

	// blockVersion < 0.11.0
	// only start verifying transaction hashes after 0.11.0
	if blockVersion.LessThan(semver.MustParse("0.11.0")) {
		return nil
	}

	for _, t := range txs {
		calculatedTxHash, hErr := TransactionHash(t, n)
		if hErr != nil {
			return fmt.Errorf("cannot calculate transaction hash of Transaction %s, reason: %w", t.Hash(), hErr)
		}
		if !calculatedTxHash.Equal(t.Hash()) {
			return fmt.Errorf("cannot verify transaction hash of Transaction %s", t.Hash())
		}
	}
	return nil
}

const commitmentTrieHeight = 64

// transactionCommitment is the root of a height 64 binary Merkle Patricia tree of the
// transaction hashes and signatures in a block.
func transactionCommitment(transactions []Transaction, protocolVersion string) (*felt.Felt, error) {
	var commitment *felt.Felt
	v0_11_1 := semver.MustParse("0.11.1")
	return commitment, trie.RunOnTempTrie(commitmentTrieHeight, func(trie *trie.Trie) error {
		blockVersion, err := ParseBlockVersion(protocolVersion)
		if err != nil {
			return err
		}

		for i, transaction := range transactions {
			signatureHash := crypto.PedersenArray()

			// blockVersion >= 0.11.1
			if blockVersion.Compare(v0_11_1) != -1 {
				signatureHash = crypto.PedersenArray(transaction.Signature()...)
			} else if _, ok := transaction.(*InvokeTransaction); ok {
				signatureHash = crypto.PedersenArray(transaction.Signature()...)
			}

			if _, err = trie.Put(new(felt.Felt).SetUint64(uint64(i)),
				crypto.Pedersen(transaction.Hash(), signatureHash)); err != nil {
				return err
			}
		}
		root, err := trie.Root()
		if err != nil {
			return err
		}
		commitment = root
		return nil
	})
}

// ParseBlockVersion computes the block version, defaulting to "0.0.0" for empty strings
func ParseBlockVersion(protocolVersion string) (*semver.Version, error) {
	if protocolVersion == "" {
		return semver.NewVersion("0.0.0")
	}

	sep := "."
	digits := strings.Split(protocolVersion, sep)
	// pad with 3 zeros in case version has less than 3 digits
	digits = append(digits, []string{"0", "0", "0"}...)

	// get first 3 digits only
	return semver.NewVersion(strings.Join(digits[:3], sep))
}

// eventCommitment computes the event commitment for a block.
func eventCommitment(receipts []*TransactionReceipt) (*felt.Felt, error) {
	var commitment *felt.Felt
	return commitment, trie.RunOnTempTrie(commitmentTrieHeight, func(trie *trie.Trie) error {
		eventCount := uint64(0)
		numWorkers := runtime.GOMAXPROCS(0)
		receiptPerWorker := len(receipts) / numWorkers
		if receiptPerWorker == 0 {
			receiptPerWorker = 1
		}
		workerPool := pool.New().WithErrors().WithMaxGoroutines(numWorkers)
		var trieMutex sync.Mutex

		for receiptIdx := range receipts {
			if receiptIdx%receiptPerWorker == 0 {
				curReceiptIdx := receiptIdx
				curEventIdx := eventCount

				workerPool.Go(func() error {
					maxIndex := curReceiptIdx + receiptPerWorker
					if maxIndex > len(receipts) {
						maxIndex = len(receipts)
					}
					receiptsSliced := receipts[curReceiptIdx:maxIndex]

					for _, receipt := range receiptsSliced {
						for _, event := range receipt.Events {
							eventHash := crypto.PedersenArray(
								event.From,
								crypto.PedersenArray(event.Keys...),
								crypto.PedersenArray(event.Data...),
							)

							eventTrieKey := new(felt.Felt).SetUint64(curEventIdx)
							trieMutex.Lock()
							_, err := trie.Put(eventTrieKey, eventHash)
							trieMutex.Unlock()
							if err != nil {
								return err
							}
							curEventIdx++
						}
					}
					return nil
				})
			}
			eventCount += uint64(len(receipts[receiptIdx].Events))
		}
		if err := workerPool.Wait(); err != nil {
			return err
		}
		root, err := trie.Root()
		if err != nil {
			return err
		}
		commitment = root
		return nil
	})
}

func EventsBloom(receipts []*TransactionReceipt) *bloom.BloomFilter {
	filter := bloom.New(eventsBloomLength, eventsBloomHashFuncs)

	for _, receipt := range receipts {
		for _, event := range receipt.Events {
			fromBytes := event.From.Bytes()
			filter.TestOrAdd(fromBytes[:])
			for index, key := range event.Keys {
				keyBytes := key.Bytes()
				keyAndIndexBytes := binary.AppendVarint(keyBytes[:], int64(index))
				filter.TestOrAdd(keyAndIndexBytes)
			}
		}
	}
	return filter
}
