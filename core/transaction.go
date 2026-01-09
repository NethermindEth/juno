package core

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/fxamacker/cbor/v2"
	"golang.org/x/crypto/sha3"
)

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
	ResourceL1DataGas
)

func (r Resource) String() string {
	switch r {
	case ResourceL1Gas:
		return "L1_GAS"
	case ResourceL2Gas:
		return "L2_GAS"
	case ResourceL1DataGas:
		return "L1_DATA"
	default:
		return ""
	}
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

type FeeUnit byte

// WEI is the default value since ETH is the first native currency.
// This allows us to avoid a costly database migration.
const (
	WEI FeeUnit = iota
	STRK
)

// From the RPC spec: The max amount and max price per unit of gas used in this transaction.
type ResourceBounds struct {
	MaxAmount uint64
	// MaxPricePerUnit is technically a uint128
	MaxPricePerUnit *felt.Felt
}

func (rb ResourceBounds) Bytes(resource Resource) []byte {
	maxAmountBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(maxAmountBytes, rb.MaxAmount)
	maxPriceBytes := rb.MaxPricePerUnit.Bytes()
	return slices.Concat(
		[]byte{0},
		[]byte(resource.String()),
		maxAmountBytes,
		maxPriceBytes[16:], // Last 128 bits.
	)
}

func (rb ResourceBounds) IsZero() bool {
	return rb.MaxAmount == 0 && (rb.MaxPricePerUnit == nil || rb.MaxPricePerUnit.IsZero())
}

type Event struct {
	Data []*felt.Felt
	From *felt.Felt
	Keys []*felt.Felt
}

type L1ToL2Message struct {
	// todo(rdr): Starknet from 0.14.1 has dropped the assumption that we use an EthAddress
	//            here. We should change this to felt.Address
	From     common.Address
	Nonce    *felt.Felt
	Payload  []*felt.Felt
	Selector *felt.Felt
	To       *felt.Felt
}

type L2ToL1Message struct {
	From    *felt.Felt
	Payload []*felt.Felt
	// todo(rdr): Starknet from 0.14.1 has dropped the assumption that we use an EthAddress
	//            here. We should change this to felt.Address
	To common.Address
}

type ExecutionResources struct {
	BuiltinInstanceCounter BuiltinInstanceCounter
	MemoryHoles            uint64
	Steps                  uint64
	DataAvailability       *DataAvailability
	TotalGasConsumed       *GasConsumed
}

type DataAvailability struct {
	L1Gas     uint64
	L1DataGas uint64
}

type BuiltinInstanceCounter struct {
	Pedersen     uint64
	RangeCheck   uint64
	Bitwise      uint64
	Output       uint64
	Ecsda        uint64
	EcOp         uint64
	Keccak       uint64
	Poseidon     uint64
	SegmentArena uint64
	AddMod       uint64
	MulMod       uint64
	RangeCheck96 uint64
}

type Transaction interface {
	// TODO: This should be TransactionHash instead of Felt.
	Hash() *felt.Felt
	Signature() []*felt.Felt
	TxVersion() *TransactionVersion
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
	EventsBloomLength    = 8192
	EventsBloomHashFuncs = 6
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

func (d *DeployTransaction) TxVersion() *TransactionVersion {
	return d.Version
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

	// Version 3 fields
	// See InvokeTransaction for descriptions of the fields.
	ResourceBounds map[Resource]ResourceBounds
	Tip            uint64
	PaymasterData  []*felt.Felt
	NonceDAMode    DataAvailabilityMode
	FeeDAMode      DataAvailabilityMode
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
	// Available in version 1 only
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

	// Version 3 fields (there was no version 2)
	ResourceBounds map[Resource]ResourceBounds
	Tip            uint64
	// From the RPC spec: data needed to allow the paymaster to pay for the transaction in native tokens
	PaymasterData []*felt.Felt
	// From RPC spec: data needed to deploy the account contract from which this tx will be initiated
	AccountDeploymentData []*felt.Felt
	// From RPC spec: The storage domain of the account's nonce (an account has a nonce per DA mode)
	NonceDAMode DataAvailabilityMode
	// From RPC spec: The storage domain of the account's balance from which fee will be charged
	FeeDAMode DataAvailabilityMode
}

func (i *InvokeTransaction) TxVersion() *TransactionVersion {
	return i.Version
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
	// Available in versions 1, 2
	MaxFee *felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	TransactionSignature []*felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The transaction’s version. Possible values are 0, 1, 2, or 3.
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of Starknet.
	Version *TransactionVersion

	// Version 2 fields
	CompiledClassHash *felt.Felt

	// Version 3 fields
	// See InvokeTransaction for descriptions of the fields.
	ResourceBounds        map[Resource]ResourceBounds
	Tip                   uint64
	PaymasterData         []*felt.Felt
	AccountDeploymentData []*felt.Felt
	NonceDAMode           DataAvailabilityMode
	FeeDAMode             DataAvailabilityMode
}

func (d *DeclareTransaction) TxVersion() *TransactionVersion {
	return d.Version
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

func (l *L1HandlerTransaction) TxVersion() *TransactionVersion {
	return l.Version
}

func (l *L1HandlerTransaction) Hash() *felt.Felt {
	return l.TransactionHash
}

func (l *L1HandlerTransaction) Signature() []*felt.Felt {
	return make([]*felt.Felt, 0)
}

func (l *L1HandlerTransaction) MessageHash() []byte {
	fromAddress := l.CallData[0].Bytes()
	toAddress := l.ContractAddress.Bytes()
	selectorBytes := l.EntryPointSelector.Bytes()

	digest := sha3.NewLegacyKeccak256()
	digest.Write(fromAddress[:])
	digest.Write(toAddress[:])
	if l.Nonce != nil {
		nonceBytes := l.Nonce.Bytes()
		lenPayload := new(felt.Felt).SetUint64(uint64(len(l.CallData) - 1)).Bytes()
		digest.Write(nonceBytes[:])
		digest.Write(selectorBytes[:])
		digest.Write(lenPayload[:])
	} else {
		lenCalldata := new(felt.Felt).SetUint64(uint64(len(l.CallData))).Bytes()
		digest.Write(lenCalldata[:])
		digest.Write(selectorBytes[:])
	}
	for idx := range l.CallData[1:] {
		data := l.CallData[idx+1].Bytes()
		digest.Write(data[:])
	}
	return digest.Sum(nil)
}

func TransactionHash(transaction Transaction, n *utils.Network) (felt.Felt, error) {
	switch t := transaction.(type) {
	case *DeclareTransaction:
		return declareTransactionHash(t, n)
	case *InvokeTransaction:
		return invokeTransactionHash(t, n)
	case *DeployTransaction:
		// it's not always correct assumption because p2p peers do not provide this field
		// so essentially we might return nil field for non-sepolia network and p2p sync
		// deploy transactions are deprecated after re-genesis therefore we don't verify
		// transaction hash
		if t.TransactionHash == nil {
			return felt.Felt{}, nil
		}
		return *t.TransactionHash, nil
	case *L1HandlerTransaction:
		return l1HandlerTransactionHash(t, n)
	case *DeployAccountTransaction:
		return deployAccountTransactionHash(t, n)
	default:
		return felt.Felt{}, errors.New("unknown transaction")
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

func invokeTransactionHash(i *InvokeTransaction, n *utils.Network) (felt.Felt, error) {
	switch {
	case i.Version.Is(0):
		calldataHash := crypto.PedersenArray(i.CallData...)
		return crypto.PedersenArray(
			invokeFelt,
			i.Version.AsFelt(),
			i.ContractAddress,
			i.EntryPointSelector,
			&calldataHash,
			i.MaxFee,
			n.L2ChainIDFelt(),
		), nil
	case i.Version.Is(1):
		calldataHash := crypto.PedersenArray(i.CallData...)
		return crypto.PedersenArray(
			invokeFelt,
			i.Version.AsFelt(),
			i.SenderAddress,
			new(felt.Felt),
			&calldataHash,
			i.MaxFee,
			n.L2ChainIDFelt(),
			i.Nonce,
		), nil
	case i.Version.Is(3):
		tipAndResourceBoundsHash := tipAndResourcesHash(i.Tip, i.ResourceBounds)
		paymasterDataHash := crypto.PoseidonArray(i.PaymasterData...)
		accountDeploymentDataHash := crypto.PoseidonArray(i.AccountDeploymentData...)
		calldataHash := crypto.PoseidonArray(i.CallData...)
		daMode := felt.FromUint64[felt.Felt](dataAvailabilityMode(i.FeeDAMode, i.NonceDAMode))
		return crypto.PoseidonArray(
			invokeFelt,
			i.Version.AsFelt(),
			i.SenderAddress,
			&tipAndResourceBoundsHash,
			&paymasterDataHash,
			n.L2ChainIDFelt(),
			i.Nonce,
			&daMode,
			&accountDeploymentDataHash,
			&calldataHash,
		), nil
	default:
		return felt.Felt{}, errInvalidTransactionVersion(i, i.Version)
	}
}

func tipAndResourcesHash(tip uint64, resourceBounds map[Resource]ResourceBounds) felt.Felt {
	l1Bounds := felt.FromBytes[felt.Felt](resourceBounds[ResourceL1Gas].Bytes(ResourceL1Gas))
	l2Bounds := felt.FromBytes[felt.Felt](resourceBounds[ResourceL2Gas].Bytes(ResourceL2Gas))
	tipFelt := felt.FromUint64[felt.Felt](tip)
	elems := []*felt.Felt{
		&tipFelt,
		&l1Bounds,
		&l2Bounds,
	}

	// l1_data_gas resource bounds were added in 0.13.4
	if bounds, ok := resourceBounds[ResourceL1DataGas]; ok && bounds.MaxPricePerUnit != nil {
		l1DataBounds := felt.FromBytes[felt.Felt](bounds.Bytes(ResourceL1DataGas))
		elems = append(elems, &l1DataBounds)
	}

	return crypto.PoseidonArray(elems...)
}

func dataAvailabilityMode(feeDAMode, nonceDAMode DataAvailabilityMode) uint64 {
	const dataAvailabilityModeBits = 32
	return uint64(feeDAMode) + uint64(nonceDAMode)<<dataAvailabilityModeBits
}

func declareTransactionHash(d *DeclareTransaction, n *utils.Network) (felt.Felt, error) {
	switch {
	case d.Version.Is(0):
		// Due to inconsistencies in version 0 hash calculation we don't verify the hash
		if d.TransactionHash == nil {
			// This is only going to happen when a transaction is received from p2p as no hash is passed along with a p2p transaction.
			// Therefore, we have to calculate the transaction hash.
			// This may become problematic if blockifier create a hash which is different from below.
			emptyHash := crypto.PedersenArray()
			h := crypto.PedersenArray(
				declareFelt,
				d.Version.AsFelt(),
				d.SenderAddress,
				&felt.Zero,
				&emptyHash,
				d.MaxFee,
				n.L2ChainIDFelt(),
				d.ClassHash,
			)
			return h, nil
		}

		return *d.TransactionHash, nil
	case d.Version.Is(1):
		classHash := crypto.PedersenArray(d.ClassHash)
		return crypto.PedersenArray(
			declareFelt,
			d.Version.AsFelt(),
			d.SenderAddress,
			new(felt.Felt),
			&classHash,
			d.MaxFee,
			n.L2ChainIDFelt(),
			d.Nonce,
		), nil
	case d.Version.Is(2):
		classHash := crypto.PedersenArray(d.ClassHash)
		return crypto.PedersenArray(
			declareFelt,
			d.Version.AsFelt(),
			d.SenderAddress,
			&felt.Zero,
			&classHash,
			d.MaxFee,
			n.L2ChainIDFelt(),
			d.Nonce,
			d.CompiledClassHash,
		), nil
	case d.Version.Is(3):
		resourceHash := tipAndResourcesHash(d.Tip, d.ResourceBounds)
		paymasterDataHash := crypto.PoseidonArray(d.PaymasterData...)
		accountDeploymentDataHash := crypto.PoseidonArray(d.AccountDeploymentData...)
		daMode := felt.FromUint64[felt.Felt](dataAvailabilityMode(d.FeeDAMode, d.NonceDAMode))
		return crypto.PoseidonArray(
			declareFelt,
			d.Version.AsFelt(),
			d.SenderAddress,
			&resourceHash,
			&paymasterDataHash,
			n.L2ChainIDFelt(),
			d.Nonce,
			&daMode,
			&accountDeploymentDataHash,
			d.ClassHash,
			d.CompiledClassHash,
		), nil
	default:
		return felt.Felt{}, errInvalidTransactionVersion(d, d.Version)
	}
}

func l1HandlerTransactionHash(l *L1HandlerTransaction, n *utils.Network) (felt.Felt, error) {
	switch {
	case l.Version.Is(0):
		// There are some l1 handler transaction which do not return a nonce and for some random
		// transaction the following hash fails.
		if l.Nonce == nil {
			return *l.TransactionHash, nil
		}
		calldataHash := crypto.PedersenArray(l.CallData...)
		return crypto.PedersenArray(
			l1HandlerFelt,
			l.Version.AsFelt(),
			l.ContractAddress,
			l.EntryPointSelector,
			&calldataHash,
			&felt.Zero,
			n.L2ChainIDFelt(),
			l.Nonce,
		), nil
	default:
		return felt.Felt{}, errInvalidTransactionVersion(l, l.Version)
	}
}

func deployAccountTransactionHash(
	d *DeployAccountTransaction,
	n *utils.Network,
) (felt.Felt, error) {
	// There is no version 0 for deploy account
	switch {
	case d.Version.Is(1):
		var digest crypto.PedersenDigest
		digest.Update(d.ClassHash)
		digest.Update(d.ContractAddressSalt)
		digest.Update(d.ConstructorCallData...)
		callDataHash := digest.Finish()

		return crypto.PedersenArray(
			deployAccountFelt,
			d.Version.AsFelt(),
			d.ContractAddress,
			&felt.Zero,
			&callDataHash,
			d.MaxFee,
			n.L2ChainIDFelt(),
			d.Nonce,
		), nil
	case d.Version.Is(3):
		resourcesHash := tipAndResourcesHash(d.Tip, d.ResourceBounds)
		paymasterDataHash := crypto.PoseidonArray(d.PaymasterData...)
		ctorCallDataHash := crypto.PoseidonArray(d.ConstructorCallData...)
		daMode := felt.FromUint64[felt.Felt](dataAvailabilityMode(d.FeeDAMode, d.NonceDAMode))
		return crypto.PoseidonArray(
			deployAccountFelt,
			d.Version.AsFelt(),
			d.ContractAddress,
			&resourcesHash,
			&paymasterDataHash,
			n.L2ChainIDFelt(),
			d.Nonce,
			&daMode,
			&ctorCallDataHash,
			d.ClassHash,
			d.ContractAddressSalt,
		), nil
	default:
		return felt.Felt{}, errInvalidTransactionVersion(d, d.Version)
	}
}

func VerifyTransactions(txs []Transaction, n *utils.Network, protocolVersion string) error {
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

// transactionCommitmentPedersen is the root of a height 64 binary Merkle Patricia tree of the
// transaction hashes and signatures in a block.
func transactionCommitmentPedersen(
	transactions []Transaction,
	protocolVersion string,
) (felt.Felt, error) {
	blockVersion, err := ParseBlockVersion(protocolVersion)
	if err != nil {
		return felt.Felt{}, err
	}

	v0_11_1 := semver.MustParse("0.11.1")
	var hashFunc processFunc[Transaction]
	if blockVersion.GreaterThanEqual(v0_11_1) {
		hashFunc = func(transaction Transaction) felt.Felt {
			signatureHash := crypto.PedersenArray(transaction.Signature()...)
			return crypto.Pedersen(transaction.Hash(), &signatureHash)
		}
	} else {
		hashFunc = func(transaction Transaction) felt.Felt {
			signatureHash := crypto.PedersenArray()
			if _, ok := transaction.(*InvokeTransaction); ok {
				signatureHash = crypto.PedersenArray(transaction.Signature()...)
			}
			return crypto.Pedersen(transaction.Hash(), &signatureHash)
		}
	}
	return calculateCommitment(transactions, trie.RunOnTempTriePedersen, hashFunc)
}

// transactionCommitmentPoseidon0134 handles empty signatures compared to
// transactionCommitmentPoseidon0132.
// Empty signatures are interpreted as [] instead of [0]
func transactionCommitmentPoseidon0134(transactions []Transaction) (felt.Felt, error) {
	return calculateCommitment(
		transactions,
		trie.RunOnTempTriePoseidon,
		func(transaction Transaction) felt.Felt {
			var digest crypto.PoseidonDigest
			digest.Update(transaction.Hash())

			if txSignature := transaction.Signature(); len(txSignature) > 0 {
				digest.Update(txSignature...)
			}

			return digest.Finish()
		})
}

// transactionCommitmentPoseidon0132 is used to calculate tx commitment for
// 0.13.2 <= block.version < 0.13.4
func transactionCommitmentPoseidon0132(transactions []Transaction) (felt.Felt, error) {
	return calculateCommitment(
		transactions,
		trie.RunOnTempTriePoseidon,
		func(transaction Transaction) felt.Felt {
			var digest crypto.PoseidonDigest
			digest.Update(transaction.Hash())

			if txSignature := transaction.Signature(); len(txSignature) > 0 {
				digest.Update(txSignature...)
			} else {
				digest.Update(&felt.Zero)
			}

			return digest.Finish()
		},
	)
}

type eventWithTxHash struct {
	Event  *Event
	TxHash *felt.Felt
}

// eventCommitmentPoseidon computes the event commitment for a block.
func eventCommitmentPoseidon(receipts []*TransactionReceipt) (felt.Felt, error) {
	eventCounter := 0
	for _, receipt := range receipts {
		eventCounter += len(receipt.Events)
	}
	items := make([]*eventWithTxHash, 0, eventCounter)
	for _, receipt := range receipts {
		for _, event := range receipt.Events {
			items = append(items, &eventWithTxHash{
				Event:  event,
				TxHash: receipt.TransactionHash,
			})
		}
	}
	return calculateCommitment(
		items,
		trie.RunOnTempTriePoseidon,
		func(item *eventWithTxHash) felt.Felt {
			return crypto.PoseidonArray(
				slices.Concat(
					[]*felt.Felt{
						item.Event.From,
						item.TxHash,
						felt.NewFromUint64[felt.Felt](uint64(len(item.Event.Keys))),
					},
					item.Event.Keys,
					[]*felt.Felt{
						felt.NewFromUint64[felt.Felt](uint64(len(item.Event.Data))),
					},
					item.Event.Data,
				)...,
			)
		},
	)
}

// eventCommitmentPedersen computes the event commitment for a block.
func eventCommitmentPedersen(receipts []*TransactionReceipt) (felt.Felt, error) {
	eventCounter := 0
	for _, receipt := range receipts {
		eventCounter += len(receipt.Events)
	}
	events := make([]*Event, 0, eventCounter)
	for _, receipt := range receipts {
		events = append(events, receipt.Events...)
	}
	return calculateCommitment(events, trie.RunOnTempTriePedersen, func(event *Event) felt.Felt {
		keysHash := crypto.PedersenArray(event.Keys...)
		dataHash := crypto.PedersenArray(event.Data...)
		return crypto.PedersenArray(
			event.From,
			&keysHash,
			&dataHash,
		)
	})
}

func EventsBloom(receipts []*TransactionReceipt) *bloom.BloomFilter {
	filter := bloom.New(EventsBloomLength, EventsBloomHashFuncs)

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
