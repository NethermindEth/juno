package rpc

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	stdsync "sync"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common/lru"
	"github.com/hashicorp/go-set/v2"
	"github.com/sourcegraph/conc"
)

//go:generate mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc Gateway
type Gateway interface {
	AddTransaction(context.Context, json.RawMessage) (json.RawMessage, error)
}

var (
	ErrContractNotFound                = &jsonrpc.Error{Code: 20, Message: "Contract not found"}
	ErrBlockNotFound                   = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrInvalidTxHash                   = &jsonrpc.Error{Code: 25, Message: "Invalid transaction hash"}
	ErrInvalidBlockHash                = &jsonrpc.Error{Code: 26, Message: "Invalid block hash"}
	ErrInvalidTxIndex                  = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
	ErrClassHashNotFound               = &jsonrpc.Error{Code: 28, Message: "Class hash not found"}
	ErrTxnHashNotFound                 = &jsonrpc.Error{Code: 29, Message: "Transaction hash not found"}
	ErrPageSizeTooBig                  = &jsonrpc.Error{Code: 31, Message: "Requested page size is too big"}
	ErrNoBlock                         = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidContinuationToken        = &jsonrpc.Error{Code: 33, Message: "Invalid continuation token"}
	ErrTooManyKeysInFilter             = &jsonrpc.Error{Code: 34, Message: "Too many keys provided in a filter"}
	ErrContractError                   = &jsonrpc.Error{Code: 40, Message: "Contract error"}
	ErrTransactionExecutionError       = &jsonrpc.Error{Code: 41, Message: "Transaction execution error"}
	ErrInvalidContractClass            = &jsonrpc.Error{Code: 50, Message: "Invalid contract class"}
	ErrClassAlreadyDeclared            = &jsonrpc.Error{Code: 51, Message: "Class already declared"}
	ErrInternal                        = &jsonrpc.Error{Code: jsonrpc.InternalError, Message: "Internal error"}
	ErrInvalidTransactionNonce         = &jsonrpc.Error{Code: 52, Message: "Invalid transaction nonce"}
	ErrInsufficientMaxFee              = &jsonrpc.Error{Code: 53, Message: "Max fee is smaller than the minimal transaction cost (validation plus fee transfer)"} //nolint:lll
	ErrInsufficientAccountBalance      = &jsonrpc.Error{Code: 54, Message: "Account balance is smaller than the transaction's max_fee"}
	ErrValidationFailure               = &jsonrpc.Error{Code: 55, Message: "Account validation failed"}
	ErrCompilationFailed               = &jsonrpc.Error{Code: 56, Message: "Compilation failed"}
	ErrContractClassSizeTooLarge       = &jsonrpc.Error{Code: 57, Message: "Contract class size is too large"}
	ErrNonAccount                      = &jsonrpc.Error{Code: 58, Message: "Sender address is not an account contract"}
	ErrDuplicateTx                     = &jsonrpc.Error{Code: 59, Message: "A transaction with the same hash already exists in the mempool"}
	ErrCompiledClassHashMismatch       = &jsonrpc.Error{Code: 60, Message: "the compiled class hash did not match the one supplied in the transaction"} //nolint:lll
	ErrUnsupportedTxVersion            = &jsonrpc.Error{Code: 61, Message: "the transaction version is not supported"}
	ErrUnsupportedContractClassVersion = &jsonrpc.Error{Code: 62, Message: "the contract class version is not supported"}
	ErrUnexpectedError                 = &jsonrpc.Error{Code: 63, Message: "An unexpected error occurred"}

	// These errors can be only be returned by Juno-specific methods.
	ErrSubscriptionNotFound = &jsonrpc.Error{Code: 100, Message: "Subscription not found"}
)

const (
	maxEventChunkSize  = 10240
	maxEventFilterKeys = 1024
	traceCacheSize     = 128
	throttledVMErr     = "VM throughput limit reached"
)

type traceCacheKey struct {
	blockHash    felt.Felt
	v0_6Response bool
}

type Handler struct {
	bcReader      blockchain.Reader
	syncReader    sync.Reader
	gatewayClient Gateway
	feederClient  *feeder.Client
	vm            vm.VM
	log           utils.Logger

	version                    string
	forceFeederTracesForBlocks *set.Set[uint64]

	newHeads *feed.Feed[*core.Header]

	idgen         func() uint64
	mu            stdsync.Mutex // protects subscriptions.
	subscriptions map[uint64]*subscription

	blockTraceCache *lru.Cache[traceCacheKey, []TracedBlockTransaction]

	filterLimit  uint
	callMaxSteps uint64
}

type subscription struct {
	cancel func()
	wg     conc.WaitGroup
	conn   jsonrpc.Conn
}

func New(bcReader blockchain.Reader, syncReader sync.Reader, virtualMachine vm.VM, version string, network *utils.Network,
	logger utils.Logger,
) *Handler {
	return &Handler{
		bcReader:   bcReader,
		syncReader: syncReader,
		log:        logger,
		vm:         virtualMachine,
		idgen: func() uint64 {
			var n uint64
			for err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil; {
			}
			return n
		},
		version:                    version,
		forceFeederTracesForBlocks: set.From(network.BlockHashMetaInfo.ForceFetchingTracesForBlocks),
		newHeads:                   feed.New[*core.Header](),
		subscriptions:              make(map[uint64]*subscription),

		blockTraceCache: lru.NewCache[traceCacheKey, []TracedBlockTransaction](traceCacheSize),
		filterLimit:     math.MaxUint,
	}
}

// WithFilterLimit sets the maximum number of blocks to scan in a single call for event filtering.
func (h *Handler) WithFilterLimit(limit uint) *Handler {
	h.filterLimit = limit
	return h
}

func (h *Handler) WithCallMaxSteps(maxSteps uint64) *Handler {
	h.callMaxSteps = maxSteps
	return h
}

func (h *Handler) WithIDGen(idgen func() uint64) *Handler {
	h.idgen = idgen
	return h
}

func (h *Handler) WithFeeder(feederClient *feeder.Client) *Handler {
	h.feederClient = feederClient
	return h
}

func (h *Handler) WithGateway(gatewayClient Gateway) *Handler {
	h.gatewayClient = gatewayClient
	return h
}

func (h *Handler) Run(ctx context.Context) error {
	newHeadsSub := h.syncReader.SubscribeNewHeads().Subscription
	defer newHeadsSub.Unsubscribe()
	feed.Tee[*core.Header](newHeadsSub, h.newHeads)
	<-ctx.Done()
	for _, sub := range h.subscriptions {
		sub.wg.Wait()
	}
	return nil
}

// ChainID returns the chain ID of the currently configured network.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L542
func (h *Handler) ChainID() (*felt.Felt, *jsonrpc.Error) {
	return h.bcReader.Network().L2ChainIDFelt(), nil
}

// BlockNumber returns the latest synced block number.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L500
func (h *Handler) BlockNumber() (uint64, *jsonrpc.Error) {
	num, err := h.bcReader.Height()
	if err != nil {
		return 0, ErrNoBlock
	}

	return num, nil
}

// BlockHashAndNumber returns the block hash and number of the latest synced block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L517
func (h *Handler) BlockHashAndNumber() (*BlockHashAndNumber, *jsonrpc.Error) {
	block, err := h.bcReader.Head()
	if err != nil {
		return nil, ErrNoBlock
	}
	return &BlockHashAndNumber{Number: block.Number, Hash: block.Hash}, nil
}

// BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L11
func (h *Handler) BlockWithTxHashes(id BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	status, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
}

func (h *Handler) BlockWithTxHashesV0_6(id BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	resp, err := h.BlockWithTxHashes(id)
	if err != nil {
		return nil, err
	}

	resp.L1DAMode = nil
	resp.L1DataGasPrice = nil
	return resp, nil
}

func (h *Handler) blockStatus(id BlockID, block *core.Block) (BlockStatus, *jsonrpc.Error) {
	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return 0, jsonErr
	}

	status := BlockAcceptedL2
	if id.Pending {
		status = BlockPending
	} else if isL1Verified(block.Number, l1H) {
		status = BlockAcceptedL1
	}

	return status, nil
}

func (h *Handler) l1Head() (*core.L1Head, *jsonrpc.Error) {
	l1Head, err := h.bcReader.L1Head()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	// nil is returned if l1 head doesn't exist
	return l1Head, nil
}

func isL1Verified(n uint64, l1 *core.L1Head) bool {
	if l1 != nil && l1.BlockNumber >= n {
		return true
	}
	return false
}

func adaptBlockHeader(header *core.Header) BlockHeader {
	var blockNumber *uint64
	// if header.Hash == nil it's a pending block
	if header.Hash != nil {
		blockNumber = &header.Number
	}

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = &felt.Zero
	}

	var l1DAMode L1DAMode
	switch header.L1DAMode {
	case core.Blob:
		l1DAMode = Blob
	case core.Calldata:
		l1DAMode = Calldata
	}

	var l1DataGasPrice ResourcePrice
	if header.L1DataGasPrice != nil {
		l1DataGasPrice = ResourcePrice{
			InWei: nilToZero(header.L1DataGasPrice.PriceInWei),
			InFri: nilToZero(header.L1DataGasPrice.PriceInFri),
		}
	} else {
		l1DataGasPrice = ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	return BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           blockNumber,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
		L1GasPrice: &ResourcePrice{
			InWei: header.GasPrice,
			InFri: nilToZero(header.GasPriceSTRK), // Old block headers will be nil.
		},
		L1DataGasPrice:  &l1DataGasPrice,
		L1DAMode:        &l1DAMode,
		StarknetVersion: header.ProtocolVersion,
	}
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
func (h *Handler) BlockWithTxs(id BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = AdaptTransaction(txn)
	}

	status, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func (h *Handler) BlockWithTxsV0_6(id BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	resp, err := h.BlockWithTxs(id)
	if err != nil {
		return nil, err
	}

	resp.L1DAMode = nil
	resp.L1DataGasPrice = nil
	return resp, nil
}

func (h *Handler) BlockWithReceipts(id BlockID) (*BlockWithReceipts, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	blockStatus, rpcErr := h.blockStatus(id, block)
	if rpcErr != nil {
		return nil, rpcErr
	}

	finalityStatus := TxnAcceptedOnL2
	if blockStatus == BlockAcceptedL1 {
		finalityStatus = TxnAcceptedOnL1
	}

	txsWithReceipts := make([]TransactionWithReceipt, len(block.Transactions))
	for index, txn := range block.Transactions {
		r := block.Receipts[index]

		txsWithReceipts[index] = TransactionWithReceipt{
			Transaction: AdaptTransaction(txn),
			// block_hash, block_number are optional in BlockWithReceipts response
			Receipt: AdaptReceipt(r, txn, finalityStatus, nil, 0, false),
		}
	}

	return &BlockWithReceipts{
		Status:       blockStatus,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txsWithReceipts,
	}, nil
}

// todo(Kirill): try to replace core.Transaction with rpc.Transaction type
func AdaptReceipt(receipt *core.TransactionReceipt, txn core.Transaction,
	finalityStatus TxnFinalityStatus, blockHash *felt.Felt, blockNumber uint64,
	v0_6Response bool,
) *TransactionReceipt {
	messages := make([]*MsgToL1, len(receipt.L2ToL1Message))
	for idx, msg := range receipt.L2ToL1Message {
		messages[idx] = &MsgToL1{
			To:      msg.To,
			Payload: msg.Payload,
			From:    msg.From,
		}
	}

	events := make([]*Event, len(receipt.Events))
	for idx, event := range receipt.Events {
		events[idx] = &Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		}
	}

	var messageHash string
	var contractAddress *felt.Felt
	switch v := txn.(type) {
	case *core.DeployTransaction:
		contractAddress = v.ContractAddress
	case *core.DeployAccountTransaction:
		contractAddress = v.ContractAddress
	case *core.L1HandlerTransaction:
		messageHash = "0x" + hex.EncodeToString(v.MessageHash())
	}

	var receiptBlockNumber *uint64
	// case for pending blocks: they don't have blockHash and therefore no block number
	if blockHash != nil {
		receiptBlockNumber = &blockNumber
	}

	var es TxnExecutionStatus
	if receipt.Reverted {
		es = TxnFailure
	} else {
		es = TxnSuccess
	}

	return &TransactionReceipt{
		FinalityStatus:  finalityStatus,
		ExecutionStatus: es,
		Type:            AdaptTransaction(txn).Type,
		Hash:            txn.Hash(),
		ActualFee: &FeePayment{
			Amount: receipt.Fee,
			Unit:   feeUnit(txn),
		},
		BlockHash:          blockHash,
		BlockNumber:        receiptBlockNumber,
		MessagesSent:       messages,
		Events:             events,
		ContractAddress:    contractAddress,
		RevertReason:       receipt.RevertReason,
		ExecutionResources: adaptExecutionResources(receipt.ExecutionResources, v0_6Response),
		MessageHash:        messageHash,
	}
}

func AdaptTransaction(t core.Transaction) *Transaction {
	var txn *Transaction
	switch v := t.(type) {
	case *core.DeployTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1521
		txn = &Transaction{
			Type:                TxnDeploy,
			Hash:                v.Hash(),
			ClassHash:           v.ClassHash,
			Version:             v.Version.AsFelt(),
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCallData: &v.ConstructorCallData,
		}
	case *core.InvokeTransaction:
		txn = adaptInvokeTransaction(v)
	case *core.DeclareTransaction:
		txn = adaptDeclareTransaction(v)
	case *core.DeployAccountTransaction:
		txn = adaptDeployAccountTrandaction(v)
	case *core.L1HandlerTransaction:
		nonce := v.Nonce
		if nonce == nil {
			nonce = &felt.Zero
		}
		txn = &Transaction{
			Type:               TxnL1Handler,
			Hash:               v.Hash(),
			Version:            v.Version.AsFelt(),
			Nonce:              nonce,
			ContractAddress:    v.ContractAddress,
			EntryPointSelector: v.EntryPointSelector,
			CallData:           &v.CallData,
		}
	default:
		panic("not a transaction")
	}

	if txn.Version.IsZero() && txn.Type != TxnL1Handler {
		txn.Nonce = nil
	}
	return txn
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1605
func adaptInvokeTransaction(t *core.InvokeTransaction) *Transaction {
	tx := &Transaction{
		Type:               TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version.AsFelt(),
		Signature:          utils.Ptr(t.Signature()),
		Nonce:              t.Nonce,
		CallData:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.Ptr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.Ptr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.Ptr(DataAvailabilityMode(t.FeeDAMode))
	}
	return tx
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1340
func adaptDeclareTransaction(t *core.DeclareTransaction) *Transaction {
	tx := &Transaction{
		Hash:              t.Hash(),
		Type:              TxnDeclare,
		MaxFee:            t.MaxFee,
		Version:           t.Version.AsFelt(),
		Signature:         utils.Ptr(t.Signature()),
		Nonce:             t.Nonce,
		ClassHash:         t.ClassHash,
		SenderAddress:     t.SenderAddress,
		CompiledClassHash: t.CompiledClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.Ptr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.Ptr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.Ptr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptDeployAccountTrandaction(t *core.DeployAccountTransaction) *Transaction {
	tx := &Transaction{
		Hash:                t.Hash(),
		MaxFee:              t.MaxFee,
		Version:             t.Version.AsFelt(),
		Signature:           utils.Ptr(t.Signature()),
		Nonce:               t.Nonce,
		Type:                TxnDeployAccount,
		ContractAddressSalt: t.ContractAddressSalt,
		ConstructorCallData: &t.ConstructorCallData,
		ClassHash:           t.ClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.Ptr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.NonceDAMode = utils.Ptr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.Ptr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func (h *Handler) blockByID(id *BlockID) (*core.Block, *jsonrpc.Error) {
	var block *core.Block
	var err error
	switch {
	case id.Latest:
		block, err = h.bcReader.Head()
	case id.Hash != nil:
		block, err = h.bcReader.BlockByHash(id.Hash)
	case id.Pending:
		var pending blockchain.Pending
		pending, err = h.bcReader.Pending()
		block = pending.Block
	default:
		block, err = h.bcReader.BlockByNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, ErrInternal.CloneWithData(err)
	}
	if block == nil {
		return nil, ErrInternal.CloneWithData("nil block with no error")
	}
	return block, nil
}

func (h *Handler) blockHeaderByID(id *BlockID) (*core.Header, *jsonrpc.Error) {
	var header *core.Header
	var err error
	switch {
	case id.Latest:
		header, err = h.bcReader.HeadsHeader()
	case id.Hash != nil:
		header, err = h.bcReader.BlockHeaderByHash(id.Hash)
	case id.Pending:
		var pending blockchain.Pending
		pending, err = h.bcReader.Pending()
		if pending.Block != nil {
			header = pending.Block.Header
		}
	default:
		header, err = h.bcReader.BlockHeaderByNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, ErrInternal.CloneWithData(err)
	}
	if header == nil {
		return nil, ErrInternal.CloneWithData("nil header with no error")
	}
	return header, nil
}

// TransactionByHash returns the details of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L158
func (h *Handler) TransactionByHash(hash felt.Felt) (*Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	return AdaptTransaction(txn), nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L373
func (h *Handler) BlockTransactionCount(id BlockID) (uint64, *jsonrpc.Error) {
	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return 0, rpcErr
	}
	return header.TransactionCount, nil
}

// TransactionByBlockIDAndIndex returns the details of a transaction identified by the given
// BlockID and index.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L184
func (h *Handler) TransactionByBlockIDAndIndex(id BlockID, txIndex int) (*Transaction, *jsonrpc.Error) {
	if txIndex < 0 {
		return nil, ErrInvalidTxIndex
	}

	if id.Pending {
		pending, err := h.bcReader.Pending()
		if err != nil {
			return nil, ErrBlockNotFound
		}

		if uint64(txIndex) > pending.Block.TransactionCount {
			return nil, ErrInvalidTxIndex
		}

		return AdaptTransaction(pending.Block.Transactions[txIndex]), nil
	}

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(header.Number, uint64(txIndex))
	if err != nil {
		return nil, ErrInvalidTxIndex
	}

	return AdaptTransaction(txn), nil
}

func feeUnit(txn core.Transaction) FeeUnit {
	feeUnit := WEI
	version := txn.TxVersion()
	if !version.Is(0) && !version.Is(1) && !version.Is(2) {
		feeUnit = FRI
	}

	return feeUnit
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHash(hash felt.Felt) (*TransactionReceipt, *jsonrpc.Error) { //nolint:dupl
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	receipt, blockHash, blockNumber, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	status := TxnAcceptedOnL2

	if blockHash != nil {
		l1H, jsonErr := h.l1Head()
		if jsonErr != nil {
			return nil, jsonErr
		}

		if isL1Verified(blockNumber, l1H) {
			status = TxnAcceptedOnL1
		}
	}

	return AdaptReceipt(receipt, txn, status, blockHash, blockNumber, false), nil
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHashV0_6(hash felt.Felt) (*TransactionReceipt, *jsonrpc.Error) { //nolint:dupl
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	receipt, blockHash, blockNumber, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	status := TxnAcceptedOnL2

	if blockHash != nil {
		l1H, jsonErr := h.l1Head()
		if jsonErr != nil {
			return nil, jsonErr
		}

		if isL1Verified(blockNumber, l1H) {
			status = TxnAcceptedOnL1
		}
	}

	return AdaptReceipt(receipt, txn, status, blockHash, blockNumber, true), nil
}

func adaptExecutionResources(resources *core.ExecutionResources, v0_6Response bool) *ExecutionResources {
	if resources == nil {
		return &ExecutionResources{}
	}

	res := &ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        resources.Steps,
			MemoryHoles:  resources.MemoryHoles,
			Pedersen:     resources.BuiltinInstanceCounter.Pedersen,
			RangeCheck:   resources.BuiltinInstanceCounter.RangeCheck,
			Bitwise:      resources.BuiltinInstanceCounter.Bitwise,
			Ecsda:        resources.BuiltinInstanceCounter.Ecsda,
			EcOp:         resources.BuiltinInstanceCounter.EcOp,
			Keccak:       resources.BuiltinInstanceCounter.Keccak,
			Poseidon:     resources.BuiltinInstanceCounter.Poseidon,
			SegmentArena: resources.BuiltinInstanceCounter.SegmentArena,
		},
	}
	if !v0_6Response && resources.DataAvailability != nil {
		res.DataAvailability = &DataAvailability{
			L1Gas:     resources.DataAvailability.L1Gas,
			L1DataGas: resources.DataAvailability.L1DataGas,
		}
	}

	return res
}

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L77
func (h *Handler) StateUpdate(id BlockID) (*StateUpdate, *jsonrpc.Error) {
	var update *core.StateUpdate
	var err error
	if id.Latest {
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			err = heightErr
		} else {
			update, err = h.bcReader.StateUpdateByNumber(height)
		}
	} else if id.Pending {
		var pending blockchain.Pending
		pending, err = h.bcReader.Pending()
		if err == nil {
			update = pending.StateUpdate
		}
	} else if id.Hash != nil {
		update, err = h.bcReader.StateUpdateByHash(id.Hash)
	} else {
		update, err = h.bcReader.StateUpdateByNumber(id.Number)
	}
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, ErrBlockNotFound
		}
		return nil, ErrInternal.CloneWithData(err)
	}

	nonces := make([]Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, Nonce{ContractAddress: addr, Nonce: *nonce})
	}

	storageDiffs := make([]StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]Entry, 0, len(diffs))
		for key, value := range diffs {
			entries = append(entries, Entry{
				Key:   key,
				Value: *value,
			})
		}

		storageDiffs = append(storageDiffs, StorageDiff{
			Address:        addr,
			StorageEntries: entries,
		})
	}

	deployedContracts := make([]DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for addr, classHash := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, DeployedContract{
			Address:   addr,
			ClassHash: *classHash,
		})
	}

	declaredClasses := make([]DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for classHash, compiledClassHash := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, DeclaredClass{
			ClassHash:         classHash,
			CompiledClassHash: *compiledClassHash,
		})
	}

	replacedClasses := make([]ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for addr, classHash := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, ReplacedClass{
			ClassHash:       *classHash,
			ContractAddress: addr,
		})
	}

	return &StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &StateDiff{
			DeprecatedDeclaredClasses: update.StateDiff.DeclaredV0Classes,
			DeclaredClasses:           declaredClasses,
			ReplacedClasses:           replacedClasses,
			Nonces:                    nonces,
			StorageDiffs:              storageDiffs,
			DeployedContracts:         deployedContracts,
		},
	}, nil
}

// Syncing returns the syncing status of the node.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L569
func (h *Handler) Syncing() (*Sync, *jsonrpc.Error) {
	defaultSyncState := &Sync{Syncing: new(bool)}

	startingBlockNumber, err := h.syncReader.StartingBlockNumber()
	if err != nil {
		return defaultSyncState, nil
	}
	startingBlockHeader, err := h.bcReader.BlockHeaderByNumber(startingBlockNumber)
	if err != nil {
		return defaultSyncState, nil
	}
	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		return defaultSyncState, nil
	}
	highestBlockHeader := h.syncReader.HighestBlockHeader()
	if highestBlockHeader == nil {
		return defaultSyncState, nil
	}
	if highestBlockHeader.Number <= head.Number {
		return defaultSyncState, nil
	}

	return &Sync{
		StartingBlockHash:   startingBlockHeader.Hash,
		StartingBlockNumber: &startingBlockHeader.Number,
		CurrentBlockHash:    head.Hash,
		CurrentBlockNumber:  &head.Number,
		HighestBlockHash:    highestBlockHeader.Hash,
		HighestBlockNumber:  &highestBlockHeader.Number,
	}, nil
}

func (h *Handler) stateByBlockID(id *BlockID) (core.StateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.StateReader
	var closer blockchain.StateCloser
	var err error
	switch {
	case id.Latest:
		reader, closer, err = h.bcReader.HeadState()
	case id.Hash != nil:
		reader, closer, err = h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		reader, closer, err = h.bcReader.PendingState()
	default:
		reader, closer, err = h.bcReader.StateAtBlockNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, ErrBlockNotFound
		}
		return nil, nil, ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getNonce")

	nonce, err := stateReader.ContractNonce(&address)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return nonce, nil
}

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key felt.Felt, id BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	value, err := stateReader.ContractStorage(&address, &key)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return value, nil
}

// ClassHashAt gets the class hash for the contract deployed at the given address in the given block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L292
func (h *Handler) ClassHashAt(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClassHashAt")

	classHash, err := stateReader.ContractClassHash(&address)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return classHash, nil
}

// Class gets the contract class definition in the given block associated with the given hash
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L248
func (h *Handler) Class(id BlockID, classHash felt.Felt) (*Class, *jsonrpc.Error) {
	state, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClass")

	declared, err := state.Class(&classHash)
	if err != nil {
		return nil, ErrClassHashNotFound
	}

	var rpcClass *Class
	switch c := declared.Class.(type) {
	case *core.Cairo0Class:
		adaptEntryPoint := func(ep core.EntryPoint) EntryPoint {
			return EntryPoint{
				Offset:   ep.Offset,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:     c.Abi,
			Program: c.Program,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.Constructors, adaptEntryPoint),
				External:    utils.Map(c.Externals, adaptEntryPoint),
				L1Handler:   utils.Map(c.L1Handlers, adaptEntryPoint),
			},
		}
	case *core.Cairo1Class:
		adaptEntryPoint := func(ep core.SierraEntryPoint) EntryPoint {
			return EntryPoint{
				Index:    &ep.Index,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.EntryPoints.Constructor, adaptEntryPoint),
				External:    utils.Map(c.EntryPoints.External, adaptEntryPoint),
				L1Handler:   utils.Map(c.EntryPoints.L1Handler, adaptEntryPoint),
			},
		}
	default:
		return nil, ErrClassHashNotFound
	}

	return rpcClass, nil
}

// ClassAt gets the contract class definition in the given block instantiated by the given contract address
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L329
func (h *Handler) ClassAt(id BlockID, address felt.Felt) (*Class, *jsonrpc.Error) {
	classHash, err := h.ClassHashAt(id, address)
	if err != nil {
		return nil, err
	}
	return h.Class(id, *classHash)
}

// Events gets the events matching a filter
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/94a969751b31f5d3e25a0c6850c723ddadeeb679/api/starknet_api_openrpc.json#L642
func (h *Handler) Events(args EventsArg) (*EventsChunk, *jsonrpc.Error) {
	if args.ChunkSize > maxEventChunkSize {
		return nil, ErrPageSizeTooBig
	} else {
		lenKeys := len(args.Keys)
		for _, keys := range args.Keys {
			lenKeys += len(keys)
		}
		if lenKeys > maxEventFilterKeys {
			return nil, ErrTooManyKeysInFilter
		}
	}

	height, err := h.bcReader.Height()
	if err != nil {
		return nil, ErrInternal
	}

	filter, err := h.bcReader.EventFilter(args.EventFilter.Address, args.EventFilter.Keys)
	if err != nil {
		return nil, ErrInternal
	}
	filter = filter.WithLimit(h.filterLimit)
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

	var cToken *blockchain.ContinuationToken
	if args.ContinuationToken != "" {
		cToken = new(blockchain.ContinuationToken)
		if err = cToken.FromString(args.ContinuationToken); err != nil {
			return nil, ErrInvalidContinuationToken
		}
	}

	if err = setEventFilterRange(filter, args.EventFilter.FromBlock, args.EventFilter.ToBlock, height); err != nil {
		return nil, ErrBlockNotFound
	}

	filteredEvents, cToken, err := filter.Events(cToken, args.ChunkSize)
	if err != nil {
		return nil, ErrInternal
	}

	emittedEvents := make([]*EmittedEvent, 0, len(filteredEvents))
	for _, fEvent := range filteredEvents {
		var blockNumber *uint64
		if fEvent.BlockHash != nil {
			blockNumber = &(fEvent.BlockNumber)
		}
		emittedEvents = append(emittedEvents, &EmittedEvent{
			BlockNumber:     blockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &Event{
				From: fEvent.From,
				Keys: fEvent.Keys,
				Data: fEvent.Data,
			},
		})
	}

	cTokenStr := ""
	if cToken != nil {
		cTokenStr = cToken.String()
	}
	return &EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}

func setEventFilterRange(filter *blockchain.EventFilter, fromID, toID *BlockID, latestHeight uint64) error {
	//nolint:gocritic
	set := func(filterRange blockchain.EventFilterRange, id *BlockID) error {
		if id == nil {
			return nil
		}

		switch {
		case id.Latest:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight)
		case id.Hash != nil:
			return filter.SetRangeEndBlockByHash(filterRange, id.Hash)
		case id.Pending:
			return filter.SetRangeEndBlockByNumber(filterRange, latestHeight+1)
		default:
			return filter.SetRangeEndBlockByNumber(filterRange, id.Number)
		}
	}
	if err := set(blockchain.EventFilterFrom, fromID); err != nil {
		return err
	}
	return set(blockchain.EventFilterTo, toID)
}

// AddTransaction relays a transaction to the gateway.
func (h *Handler) AddTransaction(ctx context.Context, tx BroadcastedTransaction) (*AddTxResponse, *jsonrpc.Error) { //nolint:gocritic
	if tx.Type == TxnDeclare && tx.Version.Cmp(new(felt.Felt).SetUint64(2)) != -1 {
		contractClass := make(map[string]any)
		if err := json.Unmarshal(tx.ContractClass, &contractClass); err != nil {
			return nil, ErrInternal.CloneWithData(fmt.Sprintf("unmarshal contract class: %v", err))
		}
		sierraProg, ok := contractClass["sierra_program"]
		if !ok {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, "{'sierra_program': ['Missing data for required field.']}")
		}

		sierraProgBytes, errIn := json.Marshal(sierraProg)
		if errIn != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		gwSierraProg, errIn := utils.Gzip64Encode(sierraProgBytes)
		if errIn != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		contractClass["sierra_program"] = gwSierraProg
		newContractClass, err := json.Marshal(contractClass)
		if err != nil {
			return nil, ErrInternal.CloneWithData(fmt.Sprintf("marshal revised contract class: %v", err))
		}
		tx.ContractClass = newContractClass
	}

	txJSON, err := json.Marshal(&struct {
		*starknet.Transaction
		ContractClass json.RawMessage `json:"contract_class,omitempty"`
	}{
		Transaction:   adaptRPCTxToFeederTx(&tx.Transaction),
		ContractClass: tx.ContractClass,
	})
	if err != nil {
		return nil, ErrInternal.CloneWithData(fmt.Sprintf("marshal transaction: %v", err))
	}

	if h.gatewayClient == nil {
		return nil, ErrInternal.CloneWithData("no gateway client configured")
	}

	respJSON, err := h.gatewayClient.AddTransaction(ctx, txJSON)
	if err != nil {
		return nil, makeJSONErrorFromGatewayError(err)
	}

	var gatewayResponse struct {
		TransactionHash *felt.Felt `json:"transaction_hash"`
		ContractAddress *felt.Felt `json:"address"`
		ClassHash       *felt.Felt `json:"class_hash"`
	}
	if err = json.Unmarshal(respJSON, &gatewayResponse); err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, fmt.Sprintf("unmarshal gateway response: %v", err))
	}

	return &AddTxResponse{
		TransactionHash: gatewayResponse.TransactionHash,
		ContractAddress: gatewayResponse.ContractAddress,
		ClassHash:       gatewayResponse.ClassHash,
	}, nil
}

func makeJSONErrorFromGatewayError(err error) *jsonrpc.Error {
	gatewayErr, ok := err.(*gateway.Error)
	if !ok {
		return jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch gatewayErr.Code {
	case gateway.InvalidContractClass:
		return ErrInvalidContractClass
	case gateway.UndeclaredClass:
		return ErrClassHashNotFound
	case gateway.ClassAlreadyDeclared:
		return ErrClassAlreadyDeclared
	case gateway.InsufficientMaxFee:
		return ErrInsufficientMaxFee
	case gateway.InsufficientAccountBalance:
		return ErrInsufficientAccountBalance
	case gateway.ValidateFailure:
		return ErrValidationFailure.CloneWithData(gatewayErr.Message)
	case gateway.ContractBytecodeSizeTooLarge, gateway.ContractClassObjectSizeTooLarge:
		return ErrContractClassSizeTooLarge
	case gateway.DuplicatedTransaction:
		return ErrDuplicateTx
	case gateway.InvalidTransactionNonce:
		return ErrInvalidTransactionNonce
	case gateway.CompilationFailed:
		return ErrCompilationFailed
	case gateway.InvalidCompiledClassHash:
		return ErrCompiledClassHashMismatch
	case gateway.InvalidTransactionVersion:
		return ErrUnsupportedTxVersion
	case gateway.InvalidContractClassVersion:
		return ErrUnsupportedContractClassVersion
	default:
		return ErrUnexpectedError.CloneWithData(gatewayErr.Message)
	}
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L401-L445
func (h *Handler) Call(funcCall FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	return h.call(funcCall, id, true)
}

func (h *Handler) CallV0_6(call FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	return h.call(call, id, false)
}

func (h *Handler) call(funcCall FunctionCall, id BlockID, useBlobData bool) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_call")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	classHash, err := state.ContractClassHash(&funcCall.ContractAddress)
	if err != nil {
		return nil, ErrContractNotFound
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	res, err := h.vm.Call(&vm.CallInfo{
		ContractAddress: &funcCall.ContractAddress,
		Selector:        &funcCall.EntryPointSelector,
		Calldata:        funcCall.Calldata,
		ClassHash:       classHash,
	}, &vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}, state, h.bcReader.Network(), h.callMaxSteps, useBlobData)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, ErrInternal.CloneWithData(throttledVMErr)
		}
		return nil, makeContractError(err)
	}
	return res, nil
}

type ContractErrorData struct {
	RevertError string `json:"revert_error"`
}

func makeContractError(err error) *jsonrpc.Error {
	return ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err.Error(),
	})
}

type TransactionExecutionErrorData struct {
	TransactionIndex uint64 `json:"transaction_index"`
	ExecutionError   string `json:"execution_error"`
}

func makeTransactionExecutionError(err *vm.TransactionExecutionError) *jsonrpc.Error {
	return ErrTransactionExecutionError.CloneWithData(TransactionExecutionErrorData{
		TransactionIndex: err.Index,
		ExecutionError:   err.Cause.Error(),
	})
}

func (h *Handler) TransactionStatus(ctx context.Context, hash felt.Felt) (*TransactionStatus, *jsonrpc.Error) {
	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		return &TransactionStatus{
			Finality:  TxnStatus(receipt.FinalityStatus),
			Execution: receipt.ExecutionStatus,
		}, nil
	case ErrTxnHashNotFound:
		if h.feederClient == nil {
			break
		}

		txStatus, err := h.feederClient.Transaction(ctx, &hash)
		if err != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}

		var status TransactionStatus
		switch txStatus.FinalityStatus {
		case starknet.AcceptedOnL1:
			status.Finality = TxnStatusAcceptedOnL1
		case starknet.AcceptedOnL2:
			status.Finality = TxnStatusAcceptedOnL2
		case starknet.Received:
			status.Finality = TxnStatusReceived
		default:
			return nil, ErrTxnHashNotFound
		}

		switch txStatus.ExecutionStatus {
		case starknet.Succeeded:
			status.Execution = TxnSuccess
		case starknet.Reverted:
			status.Execution = TxnFailure
		case starknet.Rejected:
			status.Finality = TxnStatusRejected
		default: // Omit the field on error. It's optional in the spec.
		}
		return &status, nil
	}
	return nil, txErr
}

func (h *Handler) EstimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), false, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateFeeV0_6(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateMessageFee(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	return h.estimateMessageFee(msg, id, h.EstimateFee)
}

func (h *Handler) EstimateMessageFeeV0_6(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	feeEstimate, rpcErr := h.estimateMessageFee(msg, id, h.EstimateFeeV0_6)
	if rpcErr != nil {
		return nil, rpcErr
	}

	feeEstimate.v0_6Response = true
	feeEstimate.DataGasPrice = nil
	feeEstimate.DataGasConsumed = nil

	return feeEstimate, nil
}

type estimateFeeHandler func(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error)

func (h *Handler) estimateMessageFee(msg MsgFromL1, id BlockID, f estimateFeeHandler) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	calldata := make([]*felt.Felt, 0, len(msg.Payload)+1)
	// The order of the calldata parameters matters. msg.From must be prepended.
	calldata = append(calldata, new(felt.Felt).SetBytes(msg.From.Bytes()))
	for payloadIdx := range msg.Payload {
		calldata = append(calldata, &msg.Payload[payloadIdx])
	}
	tx := BroadcastedTransaction{
		Transaction: Transaction{
			Type:               TxnL1Handler,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: new(felt.Felt).SetUint64(1),
	}
	estimates, rpcErr := f([]BroadcastedTransaction{tx}, nil, id)
	if rpcErr != nil {
		if rpcErr.Code == ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, makeContractError(errors.New(data.ExecutionError))
		}
		return nil, rpcErr
	}
	return &estimates[0], nil
}

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(ctx context.Context, hash felt.Felt) (*vm.TransactionTrace, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash, false)
}

func (h *Handler) TraceTransactionV0_6(ctx context.Context, hash felt.Felt) (*vm.TransactionTrace, *jsonrpc.Error) {
	return h.traceTransaction(ctx, &hash, true)
}

func (h *Handler) traceTransaction(ctx context.Context, hash *felt.Felt, v0_6Response bool) (*vm.TransactionTrace, *jsonrpc.Error) {
	_, _, blockNumber, err := h.bcReader.Receipt(hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	block, err := h.bcReader.BlockByNumber(blockNumber)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	txIndex := slices.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(hash)
	})
	if txIndex == -1 {
		return nil, ErrTxnHashNotFound
	}

	traceResults, traceBlockErr := h.traceBlockTransactions(ctx, block, v0_6Response)
	if traceBlockErr != nil {
		return nil, traceBlockErr
	}

	return traceResults[txIndex].TraceRoot, nil
}

func (h *Handler) SimulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	return h.simulateTransactions(id, transactions, simulationFlags, false, false)
}

// pre 13.1
func (h *Handler) SimulateTransactionsV0_6(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	return h.simulateTransactions(id, transactions, simulationFlags, true, true)
}

//nolint:funlen,gocyclo
func (h *Handler) simulateTransactions(id BlockID, transactions []BroadcastedTransaction,
	simulationFlags []SimulationFlag, v0_6Response, errOnRevert bool,
) ([]SimulatedTransaction, *jsonrpc.Error) {
	skipFeeCharge := slices.Contains(simulationFlags, SkipFeeChargeFlag)
	skipValidate := slices.Contains(simulationFlags, SkipValidateFlag)

	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	var txns []core.Transaction
	var classes []core.Class

	paidFeesOnL1 := make([]*felt.Felt, 0)
	for idx := range transactions {
		txn, declaredClass, paidFeeOnL1, aErr := adaptBroadcastedTransaction(&transactions[idx], h.bcReader.Network())
		if aErr != nil {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, aErr.Error())
		}

		if paidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, paidFeeOnL1)
		}

		txns = append(txns, txn)
		if declaredClass != nil {
			classes = append(classes, declaredClass)
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}
	useBlobData := !v0_6Response
	overallFees, dataGasConsumed, traces, err := h.vm.Execute(txns, classes, paidFeesOnL1, &blockInfo,
		state, h.bcReader.Network(), skipFeeCharge, skipValidate, errOnRevert, useBlobData)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, ErrInternal.CloneWithData(throttledVMErr)
		}
		var txnExecutionError vm.TransactionExecutionError
		if errors.As(err, &txnExecutionError) {
			return nil, makeTransactionExecutionError(&txnExecutionError)
		}
		return nil, ErrUnexpectedError.CloneWithData(err.Error())
	}

	var result []SimulatedTransaction
	for i, overallFee := range overallFees {
		feeUnit := feeUnit(txns[i])

		gasPrice := header.GasPrice
		if feeUnit == FRI {
			if gasPrice = header.GasPriceSTRK; gasPrice == nil {
				gasPrice = &felt.Zero
			}
		}

		dataGasPrice := &felt.Zero
		if header.L1DataGasPrice != nil {
			switch feeUnit {
			case FRI:
				dataGasPrice = header.L1DataGasPrice.PriceInFri
			case WEI:
				dataGasPrice = header.L1DataGasPrice.PriceInWei
			}
		}

		var gasConsumed *felt.Felt
		if !v0_6Response {
			dataGasFee := new(felt.Felt).Mul(dataGasConsumed[i], dataGasPrice)
			gasConsumed = new(felt.Felt).Sub(overallFee, dataGasFee)
		} else {
			gasConsumed = overallFee.Clone()
		}
		gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

		estimate := FeeEstimate{
			GasConsumed:     gasConsumed,
			GasPrice:        gasPrice,
			DataGasConsumed: dataGasConsumed[i],
			DataGasPrice:    dataGasPrice,
			OverallFee:      overallFee,
			Unit:            utils.Ptr(feeUnit),
			v0_6Response:    v0_6Response,
		}

		if !v0_6Response {
			trace := traces[i]
			executionResources := trace.TotalExecutionResources()
			executionResources.DataAvailability = vm.NewDataAvailability(gasConsumed, dataGasConsumed[i], header.L1DAMode)
			traces[i].ExecutionResources = executionResources
		}

		result = append(result, SimulatedTransaction{
			TransactionTrace: &traces[i],
			FeeEstimation:    estimate,
		})
	}

	return result, nil
}

func (h *Handler) TraceBlockTransactions(ctx context.Context, id BlockID) ([]TracedBlockTransaction, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return h.traceBlockTransactions(ctx, block, false)
}

func (h *Handler) TraceBlockTransactionsV0_6(ctx context.Context, id BlockID) ([]TracedBlockTransaction, *jsonrpc.Error) {
	block, rpcErr := h.blockByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return h.traceBlockTransactions(ctx, block, true)
}

var traceFallbackVersion = semver.MustParse("0.12.3")

func (h *Handler) getRevealedBlockHash(blockNumber uint64) (*felt.Felt, error) {
	const blockHashLag = 10
	if blockNumber < blockHashLag {
		return nil, nil
	}

	header, err := h.bcReader.BlockHeaderByNumber(blockNumber - blockHashLag)
	if err != nil {
		return nil, err
	}
	return header.Hash, nil
}

func (h *Handler) traceBlockTransactions(ctx context.Context, block *core.Block, v0_6Response bool, //nolint: gocyclo, funlen
) ([]TracedBlockTransaction, *jsonrpc.Error) {
	isPending := block.Hash == nil
	if !isPending {
		if blockVer, err := core.ParseBlockVersion(block.ProtocolVersion); err != nil {
			return nil, ErrUnexpectedError.CloneWithData(err.Error())
		} else if blockVer.Compare(traceFallbackVersion) != 1 || h.forceFeederTracesForBlocks.Contains(block.Number) {
			// version <= 0.12.3 or forcing fetch some blocks from feeder gateway
			return h.fetchTraces(ctx, block.Hash)
		}

		if trace, hit := h.blockTraceCache.Get(traceCacheKey{
			blockHash:    *block.Hash,
			v0_6Response: v0_6Response,
		}); hit {
			return trace, nil
		}
	}

	state, closer, err := h.bcReader.StateAtBlockHash(block.ParentHash)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in traceBlockTransactions")

	var (
		headState       core.StateReader
		headStateCloser blockchain.StateCloser
	)
	if isPending {
		headState, headStateCloser, err = h.bcReader.PendingState()
	} else {
		headState, headStateCloser, err = h.bcReader.HeadState()
	}
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headStateCloser, "Failed to close head state in traceBlockTransactions")

	var classes []core.Class
	paidFeesOnL1 := []*felt.Felt{}

	for _, transaction := range block.Transactions {
		switch tx := transaction.(type) {
		case *core.DeclareTransaction:
			class, stateErr := headState.Class(tx.ClassHash)
			if stateErr != nil {
				return nil, jsonrpc.Err(jsonrpc.InternalError, stateErr.Error())
			}
			classes = append(classes, class.Class)
		case *core.L1HandlerTransaction:
			var fee felt.Felt
			paidFeesOnL1 = append(paidFeesOnL1, fee.SetUint64(1))
		}
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(block.Number)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}
	network := h.bcReader.Network()
	header := block.Header
	blockInfo := vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}

	useBlobData := !v0_6Response
	overallFees, dataGasConsumed, traces, err := h.vm.Execute(block.Transactions, classes, paidFeesOnL1, &blockInfo, state, network, false,
		false, false, useBlobData)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, ErrInternal.CloneWithData(throttledVMErr)
		}
		// Since we are tracing an existing block, we know that there should be no errors during execution. If we encounter any,
		// report them as unexpected errors
		return nil, ErrUnexpectedError.CloneWithData(err.Error())
	}

	var result []TracedBlockTransaction
	for index, trace := range traces {
		if !v0_6Response {
			feeUnit := feeUnit(block.Transactions[index])

			gasPrice := header.GasPrice
			if feeUnit == FRI {
				if gasPrice = header.GasPriceSTRK; gasPrice == nil {
					gasPrice = &felt.Zero
				}
			}

			dataGasPrice := &felt.Zero
			if header.L1DataGasPrice != nil {
				switch feeUnit {
				case FRI:
					dataGasPrice = header.L1DataGasPrice.PriceInFri
				case WEI:
					dataGasPrice = header.L1DataGasPrice.PriceInWei
				}
			}

			dataGasFee := new(felt.Felt).Mul(dataGasConsumed[index], dataGasPrice)
			gasConsumed := new(felt.Felt).Sub(overallFees[index], dataGasFee)
			gasConsumed = gasConsumed.Div(gasConsumed, gasPrice) // division by zero felt is zero felt

			executionResources := trace.TotalExecutionResources()
			executionResources.DataAvailability = vm.NewDataAvailability(gasConsumed, dataGasConsumed[index],
				header.L1DAMode)
			traces[index].ExecutionResources = executionResources
		}
		result = append(result, TracedBlockTransaction{
			TraceRoot:       &traces[index],
			TransactionHash: block.Transactions[index].Hash(),
		})
	}

	if !isPending {
		h.blockTraceCache.Add(traceCacheKey{
			blockHash:    *block.Hash,
			v0_6Response: v0_6Response,
		}, result)
	}

	return result, nil
}

func (h *Handler) fetchTraces(ctx context.Context, blockHash *felt.Felt) ([]TracedBlockTransaction, *jsonrpc.Error) {
	rpcBlock, err := h.BlockWithTxs(BlockID{
		Hash: blockHash, // known non-nil
	})
	if err != nil {
		return nil, err
	}

	if h.feederClient == nil {
		return nil, ErrInternal.CloneWithData("no feeder client configured")
	}

	blockTrace, fErr := h.feederClient.BlockTrace(ctx, blockHash.String())
	if fErr != nil {
		return nil, ErrUnexpectedError.CloneWithData(fErr.Error())
	}

	traces, aErr := adaptBlockTrace(rpcBlock, blockTrace)
	if aErr != nil {
		return nil, ErrUnexpectedError.CloneWithData(aErr.Error())
	}

	return traces, nil
}

func (h *Handler) callAndLogErr(f func() error, msg string) {
	if err := f(); err != nil {
		h.log.Errorw(msg, "err", err)
	}
}

func (h *Handler) SpecVersion() (string, *jsonrpc.Error) {
	return "0.7.1", nil
}

func (h *Handler) SpecVersionV0_6() (string, *jsonrpc.Error) {
	return "0.6.0", nil
}

func (h *Handler) SubscribeNewHeads(ctx context.Context) (uint64, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return 0, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}

	id := h.idgen()
	subscriptionCtx, subscriptionCtxCancel := context.WithCancel(ctx)
	sub := &subscription{
		cancel: subscriptionCtxCancel,
		conn:   w,
	}
	h.mu.Lock()
	h.subscriptions[id] = sub
	h.mu.Unlock()
	headerSub := h.newHeads.Subscribe()
	sub.wg.Go(func() {
		defer func() {
			headerSub.Unsubscribe()
			h.unsubscribe(sub, id)
		}()
		for {
			select {
			case <-subscriptionCtx.Done():
				return
			case header := <-headerSub.Recv():
				resp, err := json.Marshal(jsonrpc.Request{
					Version: "2.0",
					Method:  "juno_subscribeNewHeads",
					Params: map[string]any{
						"result":       adaptBlockHeader(header),
						"subscription": id,
					},
				})
				if err != nil {
					h.log.Warnw("Error marshalling a subscription reply", "err", err)
					return
				}
				if _, err = w.Write(resp); err != nil {
					h.log.Warnw("Error writing a subscription reply", "err", err)
					return
				}
			}
		}
	})
	return id, nil
}

func (h *Handler) Unsubscribe(ctx context.Context, id uint64) (bool, *jsonrpc.Error) {
	w, ok := jsonrpc.ConnFromContext(ctx)
	if !ok {
		return false, jsonrpc.Err(jsonrpc.MethodNotFound, nil)
	}
	h.mu.Lock()
	sub, ok := h.subscriptions[id]
	h.mu.Unlock() // Don't defer since h.unsubscribe acquires the lock.
	if !ok || !sub.conn.Equal(w) {
		return false, ErrSubscriptionNotFound
	}
	sub.cancel()
	sub.wg.Wait() // Let the subscription finish before responding.
	return true, nil
}

// unsubscribe assumes h.mu is unlocked. It releases all subscription resources.
func (h *Handler) unsubscribe(sub *subscription, id uint64) {
	sub.cancel()
	h.mu.Lock()
	delete(h.subscriptions, id)
	h.mu.Unlock()
}

func (h *Handler) Methods() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxHashes,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxs,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionReceiptByHash,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.Call,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.EstimateFee,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.EstimateMessageFee,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TraceTransaction,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.SimulateTransactions,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.TraceBlockTransactions,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.SpecVersion,
		},
		{
			Name:    "juno_subscribeNewHeads",
			Handler: h.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.Unsubscribe,
		},
		{
			Name:    "starknet_getBlockWithReceipts",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithReceipts,
		},
	}, "/v0_7"
}

func (h *Handler) MethodsV0_6() ([]jsonrpc.Method, string) { //nolint: funlen
	return []jsonrpc.Method{
		{
			Name:    "starknet_chainId",
			Handler: h.ChainID,
		},
		{
			Name:    "starknet_blockNumber",
			Handler: h.BlockNumber,
		},
		{
			Name:    "starknet_blockHashAndNumber",
			Handler: h.BlockHashAndNumber,
		},
		{
			Name:    "starknet_getBlockWithTxHashes",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxHashesV0_6,
		},
		{
			Name:    "starknet_getBlockWithTxs",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockWithTxsV0_6,
		},
		{
			Name:    "starknet_getTransactionByHash",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionByHash,
		},
		{
			Name:    "starknet_getTransactionReceipt",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionReceiptByHashV0_6,
		},
		{
			Name:    "starknet_getBlockTransactionCount",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.BlockTransactionCount,
		},
		{
			Name:    "starknet_getTransactionByBlockIdAndIndex",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "index"}},
			Handler: h.TransactionByBlockIDAndIndex,
		},
		{
			Name:    "starknet_getStateUpdate",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.StateUpdate,
		},
		{
			Name:    "starknet_syncing",
			Handler: h.Syncing,
		},
		{
			Name:    "starknet_getNonce",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.Nonce,
		},
		{
			Name:    "starknet_getStorageAt",
			Params:  []jsonrpc.Parameter{{Name: "contract_address"}, {Name: "key"}, {Name: "block_id"}},
			Handler: h.StorageAt,
		},
		{
			Name:    "starknet_getClassHashAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassHashAt,
		},
		{
			Name:    "starknet_getClass",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "class_hash"}},
			Handler: h.Class,
		},
		{
			Name:    "starknet_getClassAt",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "contract_address"}},
			Handler: h.ClassAt,
		},
		{
			Name:    "starknet_addInvokeTransaction",
			Params:  []jsonrpc.Parameter{{Name: "invoke_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeployAccountTransaction",
			Params:  []jsonrpc.Parameter{{Name: "deploy_account_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_addDeclareTransaction",
			Params:  []jsonrpc.Parameter{{Name: "declare_transaction"}},
			Handler: h.AddTransaction,
		},
		{
			Name:    "starknet_getEvents",
			Params:  []jsonrpc.Parameter{{Name: "filter"}},
			Handler: h.Events,
		},
		{
			Name:    "juno_version",
			Handler: h.Version,
		},
		{
			Name:    "starknet_getTransactionStatus",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TransactionStatus,
		},
		{
			Name:    "starknet_call",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "block_id"}},
			Handler: h.CallV0_6,
		},
		{
			Name:    "starknet_estimateFee",
			Params:  []jsonrpc.Parameter{{Name: "request"}, {Name: "simulation_flags"}, {Name: "block_id"}},
			Handler: h.EstimateFeeV0_6,
		},
		{
			Name:    "starknet_estimateMessageFee",
			Params:  []jsonrpc.Parameter{{Name: "message"}, {Name: "block_id"}},
			Handler: h.EstimateMessageFeeV0_6,
		},
		{
			Name:    "starknet_traceTransaction",
			Params:  []jsonrpc.Parameter{{Name: "transaction_hash"}},
			Handler: h.TraceTransactionV0_6,
		},
		{
			Name:    "starknet_simulateTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}, {Name: "transactions"}, {Name: "simulation_flags"}},
			Handler: h.SimulateTransactionsV0_6,
		},
		{
			Name:    "starknet_traceBlockTransactions",
			Params:  []jsonrpc.Parameter{{Name: "block_id"}},
			Handler: h.TraceBlockTransactionsV0_6,
		},
		{
			Name:    "starknet_specVersion",
			Handler: h.SpecVersionV0_6,
		},
		{
			Name:    "juno_subscribeNewHeads",
			Handler: h.SubscribeNewHeads,
		},
		{
			Name:    "juno_unsubscribe",
			Params:  []jsonrpc.Parameter{{Name: "id"}},
			Handler: h.Unsubscribe,
		},
	}, "/v0_6"
}
