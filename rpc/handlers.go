package rpc

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

//go:generate mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc Gateway
type Gateway interface {
	AddTransaction(json.RawMessage) (json.RawMessage, error)
}

var (
	ErrPendingNotSupported = errors.New("pending block is not supported yet")

	ErrBlockNotFound            = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrContractNotFound         = &jsonrpc.Error{Code: 20, Message: "Contract not found"}
	ErrTxnHashNotFound          = &jsonrpc.Error{Code: 25, Message: "Transaction hash not found"}
	ErrNoBlock                  = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidTxIndex           = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
	ErrClassHashNotFound        = &jsonrpc.Error{Code: 28, Message: "Class hash not found"}
	ErrInvalidContinuationToken = &jsonrpc.Error{Code: 33, Message: "Invalid continuation token"}
	ErrPageSizeTooBig           = &jsonrpc.Error{Code: 31, Message: "Requested page size is too big"}
	ErrTooManyKeysInFilter      = &jsonrpc.Error{Code: 34, Message: "Too many keys provided in a filter"}
	ErrInvlaidContractClass     = &jsonrpc.Error{Code: 50, Message: "Invalid contract class"}
	ErrClassAlreadyDeclared     = &jsonrpc.Error{Code: 51, Message: "Class already declared"}
	ErrInternal                 = &jsonrpc.Error{Code: jsonrpc.InternalError, Message: "Internal error"}
)

const (
	maxEventChunkSize  = 10240
	maxEventFilterKeys = 1024
)

type Handler struct {
	bcReader      blockchain.Reader
	synchronizer  *sync.Synchronizer
	network       utils.Network
	gatewayClient Gateway
	log           utils.Logger
	version       string
}

func New(bcReader blockchain.Reader, synchronizer *sync.Synchronizer, n utils.Network,
	gatewayClient Gateway, version string, logger utils.Logger,
) *Handler {
	return &Handler{
		bcReader:      bcReader,
		synchronizer:  synchronizer,
		network:       n,
		log:           logger,
		gatewayClient: gatewayClient,
		version:       version,
	}
}

// ChainID returns the chain ID of the currently configured network.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L542
func (h *Handler) ChainID() (*felt.Felt, *jsonrpc.Error) {
	return h.network.ChainID(), nil
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
	block, err := h.blockByID(&id)
	if block == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return nil, jsonErr
	}

	status := StatusAcceptedL2
	if id.Pending {
		status = StatusPending
	} else if isL1Verified(block.Number, l1H) {
		status = StatusAcceptedL1
	}

	return &BlockWithTxHashes{
		Status:      status,
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
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

	return BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           blockNumber,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: header.SequencerAddress,
	}
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
func (h *Handler) BlockWithTxs(id BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	block, err := h.blockByID(&id)
	if block == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = adaptTransaction(txn)
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return nil, jsonErr
	}

	status := StatusAcceptedL2
	if id.Pending {
		status = StatusPending
	} else if isL1Verified(block.Number, l1H) {
		status = StatusAcceptedL1
	}

	return &BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func adaptTransaction(t core.Transaction) *Transaction {
	switch v := t.(type) {
	case *core.DeployTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1521
		return &Transaction{
			Type:                TxnDeploy,
			Hash:                v.Hash(),
			ClassHash:           v.ClassHash,
			Version:             v.Version,
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCalldata: v.ConstructorCallData,
			ContractAddress:     v.ContractAddress,
		}
	case *core.InvokeTransaction:
		return adaptInvokeTransaction(v)
	case *core.DeclareTransaction:
		return adaptDeclareTransaction(v)
	case *core.DeployAccountTransaction:
		sig := v.Signature()
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1466
		return &Transaction{
			Hash:                v.Hash(),
			MaxFee:              v.MaxFee,
			Version:             v.Version,
			Signature:           &sig,
			Nonce:               v.Nonce,
			Type:                TxnDeployAccount,
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCalldata: v.ConstructorCallData,
			ClassHash:           v.ClassHash,
			ContractAddress:     v.ContractAddress,
		}
	case *core.L1HandlerTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1669
		return &Transaction{
			Type:               TxnL1Handler,
			Hash:               v.Hash(),
			Version:            v.Version,
			Nonce:              v.Nonce,
			ContractAddress:    v.ContractAddress,
			EntryPointSelector: v.EntryPointSelector,
			Calldata:           &v.CallData,
		}
	default:
		panic("not a transaction")
	}
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1605
func adaptInvokeTransaction(t *core.InvokeTransaction) *Transaction {
	sig := t.Signature()
	invTxn := &Transaction{
		Type:               TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version,
		Signature:          &sig,
		Nonce:              t.Nonce,
		Calldata:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	return invTxn
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1340
func adaptDeclareTransaction(t *core.DeclareTransaction) *Transaction {
	sig := t.Signature()
	txn := &Transaction{
		Type:              TxnDeclare,
		Hash:              t.Hash(),
		MaxFee:            t.MaxFee,
		Version:           t.Version,
		Signature:         &sig,
		Nonce:             t.Nonce,
		ClassHash:         t.ClassHash,
		SenderAddress:     t.SenderAddress,
		CompiledClassHash: t.CompiledClassHash,
	}

	return txn
}

func (h *Handler) blockByID(id *BlockID) (*core.Block, error) {
	switch {
	case id.Latest:
		return h.bcReader.Head()
	case id.Hash != nil:
		return h.bcReader.BlockByHash(id.Hash)
	case id.Pending:
		pending, err := h.bcReader.Pending()
		if err != nil {
			return nil, err
		}

		return pending.Block, nil
	default:
		return h.bcReader.BlockByNumber(id.Number)
	}
}

func (h *Handler) blockHeaderByID(id *BlockID) (*core.Header, error) {
	switch {
	case id.Latest:
		return h.bcReader.HeadsHeader()
	case id.Hash != nil:
		return h.bcReader.BlockHeaderByHash(id.Hash)
	case id.Pending:
		pending, err := h.bcReader.Pending()
		if err != nil {
			return nil, err
		}

		return pending.Block.Header, nil
	default:
		return h.bcReader.BlockHeaderByNumber(id.Number)
	}
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
	return adaptTransaction(txn), nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L373
func (h *Handler) BlockTransactionCount(id BlockID) (uint64, *jsonrpc.Error) {
	header, err := h.blockHeaderByID(&id)
	if header == nil || err != nil {
		return 0, ErrBlockNotFound
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

		return adaptTransaction(pending.Block.Transactions[txIndex]), nil
	}

	header, err := h.blockHeaderByID(&id)
	if header == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(header.Number, uint64(txIndex))
	if err != nil {
		return nil, ErrInvalidTxIndex
	}

	return adaptTransaction(txn), nil
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHash(hash felt.Felt) (*TransactionReceipt, *jsonrpc.Error) {
	txn, rpcErr := h.TransactionByHash(hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	receipt, blockHash, blockNumber, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	messages := make([]*MsgToL1, len(receipt.L2ToL1Message))
	for idx, msg := range receipt.L2ToL1Message {
		messages[idx] = &MsgToL1{
			To:      msg.To,
			Payload: msg.Payload,
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

	contractAddress := txn.ContractAddress
	if txn.Type != TxnDeploy && txn.Type != TxnDeployAccount {
		contractAddress = nil
	}

	var receiptBlockNumber *uint64
	status := StatusAcceptedL2

	if blockHash != nil {
		receiptBlockNumber = &blockNumber

		l1H, jsonErr := h.l1Head()
		if jsonErr != nil {
			return nil, jsonErr
		}

		if isL1Verified(blockNumber, l1H) {
			status = StatusAcceptedL1
		}
	} else {
		// Todo: Remove after starknet v0.12.0 is released. As Pending status will be removed from Transactions and only exist for blocks
		status = StatusPending
	}

	return &TransactionReceipt{
		Status:          status,
		Type:            txn.Type,
		Hash:            txn.Hash,
		ActualFee:       receipt.Fee,
		BlockHash:       blockHash,
		BlockNumber:     receiptBlockNumber,
		MessagesSent:    messages,
		Events:          events,
		ContractAddress: contractAddress,
	}, nil
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
		return nil, ErrBlockNotFound
	}

	nonces := make([]Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, Nonce{ContractAddress: new(felt.Felt).Set(&addr), Nonce: nonce})
	}

	storageDiffs := make([]StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]Entry, len(diffs))

		for index, diff := range diffs {
			entries[index] = Entry{Key: diff.Key, Value: diff.Value}
		}

		storageDiffs = append(storageDiffs, StorageDiff{Address: new(felt.Felt).Set(&addr), StorageEntries: entries})
	}

	deployedContracts := make([]DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for _, deployedContract := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, DeployedContract{
			Address:   deployedContract.Address,
			ClassHash: deployedContract.ClassHash,
		})
	}

	declaredClasses := make([]DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for _, declaredClass := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, DeclaredClass{
			ClassHash:         declaredClass.ClassHash,
			CompiledClassHash: declaredClass.CompiledClassHash,
		})
	}

	replacedClasses := make([]ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for _, replacedClass := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, ReplacedClass{
			ClassHash:       replacedClass.ClassHash,
			ContractAddress: replacedClass.Address,
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

	startingBlockNumber := h.synchronizer.StartingBlockNumber
	if startingBlockNumber == nil {
		return defaultSyncState, nil
	}
	startingBlockHeader, err := h.bcReader.BlockHeaderByNumber(*startingBlockNumber)
	if err != nil {
		return defaultSyncState, nil
	}
	head, err := h.bcReader.HeadsHeader()
	if err != nil {
		return defaultSyncState, nil
	}
	highestBlockHeader := h.synchronizer.HighestBlockHeader
	if highestBlockHeader == nil {
		return defaultSyncState, nil
	}
	if highestBlockHeader.Number <= head.Number {
		return defaultSyncState, nil
	}

	startingBlockNumberAsHex := NumAsHex(startingBlockHeader.Number)
	currentBlockNumberAsHex := NumAsHex(head.Number)
	highestBlockNumberAsHex := NumAsHex(highestBlockHeader.Number)
	return &Sync{
		StartingBlockHash:   startingBlockHeader.Hash,
		StartingBlockNumber: &startingBlockNumberAsHex,
		CurrentBlockHash:    head.Hash,
		CurrentBlockNumber:  &currentBlockNumberAsHex,
		HighestBlockHash:    highestBlockHeader.Hash,
		HighestBlockNumber:  &highestBlockNumberAsHex,
	}, nil
}

func (h *Handler) stateByBlockID(id *BlockID) (core.StateReader, blockchain.StateCloser, error) {
	switch {
	case id.Latest:
		return h.bcReader.HeadState()
	case id.Hash != nil:
		return h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		return h.bcReader.PendingState()
	default:
		return h.bcReader.StateAtBlockNumber(id.Number)
	}
}

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	nonce, err := stateReader.ContractNonce(&address)
	if closerErr := stateCloser(); closerErr != nil {
		h.log.Errorw("Error closing state reader in getNonce", "err", closerErr)
	}
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
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	value, err := stateReader.ContractStorage(&address, &key)
	if closerErr := stateCloser(); closerErr != nil {
		h.log.Errorw("Error closing state reader in getStorageAt", "err", closerErr)
	}
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
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	classHash, err := stateReader.ContractClassHash(&address)
	if closerErr := stateCloser(); closerErr != nil {
		h.log.Errorw("Error closing state reader in getClassHashAt", "err", closerErr)
	}
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
	state, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	declared, err := state.Class(&classHash)
	if closerErr := stateCloser(); closerErr != nil {
		h.log.Errorw("Error closing state reader in getClass", "err", closerErr)
	}

	if err != nil {
		return nil, ErrClassHashNotFound
	}

	var rpcClass *Class
	switch c := declared.Class.(type) {
	case *core.Cairo0Class:
		rpcClass = &Class{
			Abi:         c.Abi,
			Program:     c.Program,
			EntryPoints: EntryPoints{},
		}

		rpcClass.EntryPoints.Constructor = make([]EntryPoint, 0, len(c.Constructors))
		for _, entryPoint := range c.Constructors {
			rpcClass.EntryPoints.Constructor = append(rpcClass.EntryPoints.Constructor, EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.L1Handler = make([]EntryPoint, 0, len(c.L1Handlers))
		for _, entryPoint := range c.L1Handlers {
			rpcClass.EntryPoints.L1Handler = append(rpcClass.EntryPoints.L1Handler, EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.External = make([]EntryPoint, 0, len(c.Externals))
		for _, entryPoint := range c.Externals {
			rpcClass.EntryPoints.External = append(rpcClass.EntryPoints.External, EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

	case *core.Cairo1Class:
		rpcClass = &Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints:          EntryPoints{},
		}

		rpcClass.EntryPoints.Constructor = make([]EntryPoint, 0, len(c.EntryPoints.Constructor))
		for _, entryPoint := range c.EntryPoints.Constructor {
			index := entryPoint.Index
			rpcClass.EntryPoints.Constructor = append(rpcClass.EntryPoints.Constructor, EntryPoint{
				Index:    &index,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.L1Handler = make([]EntryPoint, 0, len(c.EntryPoints.L1Handler))
		for _, entryPoint := range c.EntryPoints.L1Handler {
			index := entryPoint.Index
			rpcClass.EntryPoints.L1Handler = append(rpcClass.EntryPoints.L1Handler, EntryPoint{
				Index:    &index,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.External = make([]EntryPoint, 0, len(c.EntryPoints.External))
		for _, entryPoint := range c.EntryPoints.External {
			index := entryPoint.Index
			rpcClass.EntryPoints.External = append(rpcClass.EntryPoints.External, EntryPoint{
				Index:    &index,
				Selector: entryPoint.Selector,
			})
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

	defer func() {
		if closerErr := filter.Close(); closerErr != nil {
			h.log.Errorw("Error closing event filter in events", "err", closerErr)
		}
	}()

	var cToken *blockchain.ContinuationToken
	if len(args.ContinuationToken) > 0 {
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
			blockNumber = &fEvent.BlockNumber
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

// AddInvokeTransaction relays an invoke transaction to the gateway.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_write_api.json#L11
// Note: No checks are performed on the incoming request since we rely on the gateway to perform sanity checks.
// As this handler is just as a proxy. Any error returned by the gateway is returned to the user as a jsonrpc error.
func (h *Handler) AddInvokeTransaction(invokeTx json.RawMessage) (*AddInvokeTxResponse, *jsonrpc.Error) {
	resp, err := h.gatewayClient.AddTransaction(invokeTx)
	if err != nil {
		return nil, jsonrpc.Err(getAddInvokeTxCode(err), err.Error())
	}

	invokeRes := new(AddInvokeTxResponse)
	err = json.Unmarshal(resp, invokeRes)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	return invokeRes, nil
}

// getAddInvokeTxCode returns the relevant Code for a given addInvokeTx error
func getAddInvokeTxCode(err error) int {
	switch {
	case strings.Contains(err.Error(), "contract address") && strings.Contains(err.Error(), "is out of range"):
		return jsonrpc.InvalidParams
	case strings.Contains(err.Error(), "Fee") && strings.Contains(err.Error(), "is out of range"):
		return jsonrpc.InvalidParams
	case strings.Contains(err.Error(), "Missing data for required field"):
		return jsonrpc.InvalidParams
	case strings.Contains(err.Error(), "not supported. Supported versions"):
		return jsonrpc.InvalidParams
	case strings.Contains(err.Error(), "max_fee must be bigger than 0"):
		return jsonrpc.InvalidParams
	default:
		return jsonrpc.InternalError
	}
}

// AddDeployAccountTransaction relays an deploy account transaction to the gateway.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_write_api.json#L74
// Note: This handler is a proxy. No checks are performed on the incoming request
// since we rely on the gateway to perform sanity checks. Any error returned by the
// gateway is returned to the user as a jsonrpc error.
func (h *Handler) AddDeployAccountTransaction(deployAcntTx json.RawMessage) (*DeployAccountTxResponse, *jsonrpc.Error) {
	resp, err := h.gatewayClient.AddTransaction(deployAcntTx)
	if err != nil {
		if strings.Contains(err.Error(), "Class hash not found") {
			ErrClassHashNotFound.Data = err.Error()
			return nil, ErrClassHashNotFound
		}
		return nil, jsonrpc.Err(getAddInvokeTxCode(err), err.Error())
	}

	deployResp := new(DeployAccountTxResponse)
	if err = json.Unmarshal(resp, deployResp); err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	return deployResp, nil
}

// PendingTransactions gets the transactions in the pending block
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/aaea417f193bbec87b59455128d4b09a06876c28/api/starknet_api_openrpc.json#L602-L616
func (h *Handler) PendingTransactions() ([]*Transaction, *jsonrpc.Error) {
	var pendingTxns []*Transaction

	pending, err := h.bcReader.Pending()
	if err == nil {
		pendingTxns = make([]*Transaction, 0, len(pending.Block.Transactions))
		for _, txn := range pending.Block.Transactions {
			pendingTxns = append(pendingTxns, adaptTransaction(txn))
		}
	}
	return pendingTxns, nil
}

// AddDeclareTransaction relays a declare transaction to the gateway.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_write_api.json#L39
// Note: The gateway expects the sierra_program to be gzip compressed and 64-base encoded. We perform this operation,
// and then relay the transaction to the gateway.
func (h *Handler) AddDeclareTransaction(declareTx json.RawMessage) (*DeclareTxResponse, *jsonrpc.Error) {
	var request map[string]any
	err := json.Unmarshal(declareTx, &request)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	if version, ok := request["version"]; ok && version == "0x2" {
		contractClass, ok := request["contract_class"].(map[string]interface{})
		if !ok {
			return nil, jsonrpc.Err(jsonrpc.InvalidParams, "{'contract_class': ['Missing data for required field.']}")
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

		updatedReq, errIn := json.Marshal(request)
		if errIn != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}
		declareTx = updatedReq
	}

	resp, err := h.gatewayClient.AddTransaction(declareTx)
	if err != nil {
		if strings.Contains(err.Error(), "Invalid contract class") {
			return nil, ErrInvlaidContractClass
		} else if strings.Contains(err.Error(), "Class already declared") {
			return nil, ErrClassAlreadyDeclared
		}

		return nil, jsonrpc.Err(getAddInvokeTxCode(err), err.Error())
	}

	declareRes := new(DeclareTxResponse)
	err = json.Unmarshal(resp, declareRes)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	return declareRes, nil
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}
