package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	client "github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var (
	ErrNoTraceAvailable                = &jsonrpc.Error{Code: 10, Message: "No trace available for transaction"}
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
)

const (
	maxEventChunkSize  = 10240
	maxEventFilterKeys = 1024
)

type Handler struct {
	bcReader      blockchain.Reader
	synchronizer  *sync.Synchronizer
	network       core.Network
	gatewayClient client.GatewayInterface
	feederClient  client.FeederInterface
	vm            vm.VM
	log           utils.Logger
	version       string
}

func New(bcReader blockchain.Reader, synchronizer *sync.Synchronizer, n core.Network,
	gatewayClient client.GatewayInterface, feederClient client.FeederInterface, virtualMachine vm.VM, version string, logger utils.Logger,
) *Handler {
	return &Handler{
		bcReader:      bcReader,
		synchronizer:  synchronizer,
		network:       n,
		log:           logger,
		feederClient:  feederClient,
		gatewayClient: gatewayClient,
		vm:            virtualMachine,
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

// BlockNumber returns the latest utils.Synced block number.
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

// utils.BlockHashAndNumber returns the block hash and number of the latest utils.Synced block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L517
func (h *Handler) BlockHashAndNumber() (*utils.BlockHashAndNumber, *jsonrpc.Error) {
	block, err := h.bcReader.Head()
	if err != nil {
		return nil, ErrNoBlock
	}
	return &utils.BlockHashAndNumber{Number: block.Number, Hash: block.Hash}, nil
}

// utils.BlockWithTxHashes returns the block information with transaction hashes given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L11
func (h *Handler) BlockWithTxHashes(id utils.BlockID) (*utils.BlockWithTxHashes, *jsonrpc.Error) {
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

	status := utils.BlockAcceptedL2
	if id.Pending {
		status = utils.BlockPending
	} else if isL1Verified(block.Number, l1H) {
		status = utils.BlockAcceptedL1
	}

	return &utils.BlockWithTxHashes{
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

func adaptBlockHeader(header *core.Header) utils.BlockHeader {
	var blockNumber *uint64
	// if header.Hash == nil it's a pending block
	if header.Hash != nil {
		blockNumber = &header.Number
	}

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = &felt.Zero
	}

	return utils.BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           blockNumber,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: sequencerAddress,
	}
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
func (h *Handler) BlockWithTxs(id utils.BlockID) (*utils.BlockWithTxs, *jsonrpc.Error) {
	block, err := h.blockByID(&id)
	if block == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txs := make([]*utils.Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = adaptTransaction(txn)
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return nil, jsonErr
	}

	status := utils.BlockAcceptedL2
	if id.Pending {
		status = utils.BlockPending
	} else if isL1Verified(block.Number, l1H) {
		status = utils.BlockAcceptedL1
	}

	return &utils.BlockWithTxs{
		Status:       status,
		BlockHeader:  adaptBlockHeader(block.Header),
		Transactions: txs,
	}, nil
}

func adaptTransaction(t core.Transaction) *utils.Transaction {
	var txn *utils.Transaction
	switch v := t.(type) {
	case *core.DeployTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1521
		txn = &utils.Transaction{
			Type:                utils.TxnDeploy,
			Hash:                v.Hash(),
			ClassHash:           v.ClassHash,
			Version:             v.Version,
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCallData: &v.ConstructorCallData,
		}
	case *core.InvokeTransaction:
		txn = adaptInvokeTransaction(v)
	case *core.DeclareTransaction:
		txn = adaptDeclareTransaction(v)
	case *core.DeployAccountTransaction:
		sig := v.Signature()
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1466
		txn = &utils.Transaction{
			Hash:                v.Hash(),
			MaxFee:              v.MaxFee,
			Version:             v.Version,
			Signature:           &sig,
			Nonce:               v.Nonce,
			Type:                utils.TxnDeployAccount,
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCallData: &v.ConstructorCallData,
			ClassHash:           v.ClassHash,
		}
	case *core.L1HandlerTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1669
		txn = &utils.Transaction{
			Type:               utils.TxnL1Handler,
			Hash:               v.Hash(),
			Version:            v.Version,
			Nonce:              v.Nonce,
			ContractAddress:    v.ContractAddress,
			EntryPointSelector: v.EntryPointSelector,
			CallData:           &v.CallData,
		}
	default:
		panic("not a transaction")
	}

	if txn.Version.IsZero() && txn.Type != utils.TxnL1Handler {
		txn.Nonce = nil
	}
	return txn
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1605
func adaptInvokeTransaction(t *core.InvokeTransaction) *utils.Transaction {
	sig := t.Signature()
	invTxn := &utils.Transaction{
		Type:               utils.TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version,
		Signature:          &sig,
		Nonce:              t.Nonce,
		CallData:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	return invTxn
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1340
func adaptDeclareTransaction(t *core.DeclareTransaction) *utils.Transaction {
	sig := t.Signature()
	txn := &utils.Transaction{
		Type:              utils.TxnDeclare,
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

func (h *Handler) blockByID(id *utils.BlockID) (*core.Block, error) {
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

func (h *Handler) blockHeaderByID(id *utils.BlockID) (*core.Header, error) {
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
func (h *Handler) TransactionByHash(hash felt.Felt) (*utils.Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	return adaptTransaction(txn), nil
}

// BlockTransactionCount returns the number of transactions in a block
// identified by the given utils.BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L373
func (h *Handler) BlockTransactionCount(id utils.BlockID) (uint64, *jsonrpc.Error) {
	header, err := h.blockHeaderByID(&id)
	if header == nil || err != nil {
		return 0, ErrBlockNotFound
	}
	return header.TransactionCount, nil
}

// TransactionByBlockIDAndIndex returns the details of a transaction identified by the given
// utils.BlockID and index.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L184
func (h *Handler) TransactionByBlockIDAndIndex(id utils.BlockID, txIndex int) (*utils.Transaction, *jsonrpc.Error) {
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
func (h *Handler) TransactionReceiptByHash(hash felt.Felt) (*utils.TransactionReceipt, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	receipt, blockHash, blockNumber, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}

	messages := make([]*utils.MsgToL1, len(receipt.L2ToL1Message))
	for idx, msg := range receipt.L2ToL1Message {
		messages[idx] = &utils.MsgToL1{
			To:      msg.To,
			Payload: msg.Payload,
			From:    msg.From,
		}
	}

	events := make([]*utils.Event, len(receipt.Events))
	for idx, event := range receipt.Events {
		events[idx] = &utils.Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		}
	}

	var contractAddress *felt.Felt
	switch v := txn.(type) {
	case *core.DeployTransaction:
		contractAddress = v.ContractAddress
	case *core.DeployAccountTransaction:
		contractAddress = v.ContractAddress
	}

	var receiptBlockNumber *uint64
	status := utils.AcceptedOnL2

	if blockHash != nil {
		receiptBlockNumber = &blockNumber

		l1H, jsonErr := h.l1Head()
		if jsonErr != nil {
			return nil, jsonErr
		}

		if isL1Verified(blockNumber, l1H) {
			status = utils.AcceptedOnL1
		}
	}

	var es utils.ExecutionStatus
	if receipt.Reverted {
		es = utils.Reverted
	} else {
		es = utils.Succeeded
	}

	return &utils.TransactionReceipt{
		FinalityStatus:  status,
		ExecutionStatus: es,
		Type:            adaptTransaction(txn).Type,
		Hash:            txn.Hash(),
		ActualFee:       receipt.Fee,
		BlockHash:       blockHash,
		BlockNumber:     receiptBlockNumber,
		MessagesSent:    messages,
		Events:          events,
		ContractAddress: contractAddress,
		RevertReason:    receipt.RevertReason,
	}, nil
}

// StateUpdate returns the state update identified by the given utils.BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L77
func (h *Handler) StateUpdate(id utils.BlockID) (*utils.StateUpdate, *jsonrpc.Error) {
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

	nonces := make([]utils.Nonce, 0, len(update.StateDiff.Nonces))
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, utils.Nonce{ContractAddress: new(felt.Felt).Set(&addr), Nonce: nonce})
	}

	storageDiffs := make([]utils.StorageDiff, 0, len(update.StateDiff.StorageDiffs))
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]utils.Entry, len(diffs))

		for index, diff := range diffs {
			entries[index] = utils.Entry{Key: diff.Key, Value: diff.Value}
		}

		storageDiffs = append(storageDiffs, utils.StorageDiff{Address: new(felt.Felt).Set(&addr), StorageEntries: entries})
	}

	deployedContracts := make([]utils.DeployedContract, 0, len(update.StateDiff.DeployedContracts))
	for _, deployedContract := range update.StateDiff.DeployedContracts {
		deployedContracts = append(deployedContracts, utils.DeployedContract{
			Address:   deployedContract.Address,
			ClassHash: deployedContract.ClassHash,
		})
	}

	declaredClasses := make([]utils.DeclaredClass, 0, len(update.StateDiff.DeclaredV1Classes))
	for _, declaredClass := range update.StateDiff.DeclaredV1Classes {
		declaredClasses = append(declaredClasses, utils.DeclaredClass{
			ClassHash:         declaredClass.ClassHash,
			CompiledClassHash: declaredClass.CompiledClassHash,
		})
	}

	replacedClasses := make([]utils.ReplacedClass, 0, len(update.StateDiff.ReplacedClasses))
	for _, replacedClass := range update.StateDiff.ReplacedClasses {
		replacedClasses = append(replacedClasses, utils.ReplacedClass{
			ClassHash:       replacedClass.ClassHash,
			ContractAddress: replacedClass.Address,
		})
	}

	return &utils.StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &utils.StateDiff{
			DeprecatedDeclaredClasses: update.StateDiff.DeclaredV0Classes,
			DeclaredClasses:           declaredClasses,
			ReplacedClasses:           replacedClasses,
			Nonces:                    nonces,
			StorageDiffs:              storageDiffs,
			DeployedContracts:         deployedContracts,
		},
	}, nil
}

// utils.Syncing returns the utils.Syncing status of the node.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L569
func (h *Handler) Syncing() (*utils.Sync, *jsonrpc.Error) {
	defaultSyncState := &utils.Sync{Syncing: new(bool)}

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
	highestBlockHeader := h.synchronizer.HighestBlockHeader.Load()
	if highestBlockHeader == nil {
		return defaultSyncState, nil
	}
	if highestBlockHeader.Number <= head.Number {
		return defaultSyncState, nil
	}

	return &utils.Sync{
		StartingBlockHash:   startingBlockHeader.Hash,
		StartingBlockNumber: &startingBlockHeader.Number,
		CurrentBlockHash:    head.Hash,
		CurrentBlockNumber:  &head.Number,
		HighestBlockHash:    highestBlockHeader.Hash,
		HighestBlockNumber:  &highestBlockHeader.Number,
	}, nil
}

func (h *Handler) stateByBlockID(id *utils.BlockID) (core.StateReader, blockchain.StateCloser, error) {
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
func (h *Handler) Nonce(id utils.BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
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
func (h *Handler) StorageAt(address, key felt.Felt, id utils.BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
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
func (h *Handler) ClassHashAt(id utils.BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
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
func (h *Handler) Class(id utils.BlockID, classHash felt.Felt) (*utils.Class, *jsonrpc.Error) {
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

	var rpcClass *utils.Class
	switch c := declared.Class.(type) {
	case *core.Cairo0Class:
		rpcClass = &utils.Class{
			Abi:         c.Abi,
			Program:     c.Program,
			EntryPoints: utils.EntryPoints{},
		}

		rpcClass.EntryPoints.Constructor = make([]utils.EntryPoint, 0, len(c.Constructors))
		for _, entryPoint := range c.Constructors {
			rpcClass.EntryPoints.Constructor = append(rpcClass.EntryPoints.Constructor, utils.EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.L1Handler = make([]utils.EntryPoint, 0, len(c.L1Handlers))
		for _, entryPoint := range c.L1Handlers {
			rpcClass.EntryPoints.L1Handler = append(rpcClass.EntryPoints.L1Handler, utils.EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.External = make([]utils.EntryPoint, 0, len(c.Externals))
		for _, entryPoint := range c.Externals {
			rpcClass.EntryPoints.External = append(rpcClass.EntryPoints.External, utils.EntryPoint{
				Offset:   entryPoint.Offset,
				Selector: entryPoint.Selector,
			})
		}

	case *core.Cairo1Class:
		rpcClass = &utils.Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints:          utils.EntryPoints{},
		}

		rpcClass.EntryPoints.Constructor = make([]utils.EntryPoint, 0, len(c.EntryPoints.Constructor))
		for _, entryPoint := range c.EntryPoints.Constructor {
			index := entryPoint.Index
			rpcClass.EntryPoints.Constructor = append(rpcClass.EntryPoints.Constructor, utils.EntryPoint{
				Index:    &index,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.L1Handler = make([]utils.EntryPoint, 0, len(c.EntryPoints.L1Handler))
		for _, entryPoint := range c.EntryPoints.L1Handler {
			index := entryPoint.Index
			rpcClass.EntryPoints.L1Handler = append(rpcClass.EntryPoints.L1Handler, utils.EntryPoint{
				Index:    &index,
				Selector: entryPoint.Selector,
			})
		}

		rpcClass.EntryPoints.External = make([]utils.EntryPoint, 0, len(c.EntryPoints.External))
		for _, entryPoint := range c.EntryPoints.External {
			index := entryPoint.Index
			rpcClass.EntryPoints.External = append(rpcClass.EntryPoints.External, utils.EntryPoint{
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
func (h *Handler) ClassAt(id utils.BlockID, address felt.Felt) (*utils.Class, *jsonrpc.Error) {
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
func (h *Handler) Events(args utils.EventsArg) (*utils.EventsChunk, *jsonrpc.Error) {
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
	defer h.callAndLogErr(filter.Close, "Error closing event filter in events")

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

	emittedEvents := make([]*utils.EmittedEvent, 0, len(filteredEvents))
	for _, fEvent := range filteredEvents {
		var blockNumber *uint64
		if fEvent.BlockHash != nil {
			blockNumber = &fEvent.BlockNumber
		}
		emittedEvents = append(emittedEvents, &utils.EmittedEvent{
			BlockNumber:     blockNumber,
			BlockHash:       fEvent.BlockHash,
			TransactionHash: fEvent.TransactionHash,
			Event: &utils.Event{
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
	return &utils.EventsChunk{Events: emittedEvents, ContinuationToken: cTokenStr}, nil
}

func setEventFilterRange(filter *blockchain.EventFilter, fromID, toID *utils.BlockID, latestHeight uint64) error {
	set := func(filterRange blockchain.EventFilterRange, id *utils.BlockID) error {
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
func (h *Handler) AddTransaction(txnJSON json.RawMessage) (*utils.AddTxResponse, *jsonrpc.Error) {
	var request map[string]any
	err := json.Unmarshal(txnJSON, &request)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InvalidJSON, err.Error())
	}

	if txnType, typeFound := request["type"]; typeFound && txnType == utils.TxnInvoke.String() {
		request["type"] = utils.TxnInvoke.String()

		updatedReq, errIn := json.Marshal(request)
		if errIn != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}
		txnJSON = updatedReq
	} else if version, ok := request["version"]; ok && version == "0x2" {
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
		txnJSON = updatedReq
	}

	resp, err := h.gatewayClient.AddTransaction(txnJSON)
	if err != nil {
		return nil, makeJSONErrorFromGatewayError(err)
	}

	var response utils.AddTxResponse
	err = json.Unmarshal(resp, &response)
	if err != nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	return &response, nil
}

func makeJSONErrorFromGatewayError(err error) *jsonrpc.Error {
	gatewayErr, ok := err.(*utils.Error)
	if !ok {
		return jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch gatewayErr.Code {
	case utils.InvalidContractClass:
		return ErrInvalidContractClass
	case utils.UndeclaredClass:
		return ErrClassHashNotFound
	case utils.ClassAlreadyDeclared:
		return ErrClassAlreadyDeclared
	case utils.InsufficientMaxFee:
		return ErrInsufficientMaxFee
	case utils.InsufficientAccountBalance:
		return ErrInsufficientAccountBalance
	case utils.ValidateFailure:
		return ErrValidationFailure
	case utils.ContractBytecodeSizeTooLarge, utils.ContractClassObjectSizeTooLarge:
		return ErrContractClassSizeTooLarge
	case utils.DuplicatedTransaction:
		return ErrDuplicateTx
	case utils.InvalidTransactionNonce:
		return ErrInvalidTransactionNonce
	case utils.CompilationFailed:
		return ErrCompilationFailed
	case utils.InvalidCompiledClassHash:
		return ErrCompiledClassHashMismatch
	case utils.InvalidTransactionVersion:
		return ErrUnsupportedTxVersion
	case utils.InvalidContractClassVersion:
		return ErrUnsupportedContractClassVersion
	default:
		unexpectedErr := ErrUnexpectedError
		unexpectedErr.Data = gatewayErr.Message
		return unexpectedErr
	}
}

// PendingTransactions gets the transactions in the pending block
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/aaea417f193bbec87b59455128d4b09a06876c28/api/starknet_api_openrpc.json#L602-L616
func (h *Handler) PendingTransactions() ([]*utils.Transaction, *jsonrpc.Error) {
	var pendingTxns []*utils.Transaction

	pending, err := h.bcReader.Pending()
	if err == nil {
		pendingTxns = make([]*utils.Transaction, 0, len(pending.Block.Transactions))
		for _, txn := range pending.Block.Transactions {
			pendingTxns = append(pendingTxns, adaptTransaction(txn))
		}
	}
	return pendingTxns, nil
}

func (h *Handler) Version() (string, *jsonrpc.Error) {
	return h.version, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L401-L445
func (h *Handler) Call(call utils.FunctionCall, id utils.BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	state, closer, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_call")

	header, err := h.blockHeaderByID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	_, err = state.ContractClassHash(&call.ContractAddress)
	if err != nil {
		return nil, ErrContractNotFound
	}

	blockNumber := header.Number
	if id.Pending {
		height, hErr := h.bcReader.Height()
		if hErr != nil {
			return nil, ErrBlockNotFound
		}
		blockNumber = height + 1
	}

	res, err := h.vm.Call(&call.ContractAddress, &call.EntryPointSelector, call.Calldata, blockNumber, header.Timestamp, state, h.network)
	if err != nil {
		contractErr := *ErrContractError
		contractErr.Data = err.Error()
		return nil, &contractErr
	}
	return res, nil
}

func (h *Handler) TransactionStatus(ctx context.Context, hash felt.Felt) (*utils.TransactionStatus, *jsonrpc.Error) {
	var status *utils.TransactionStatus

	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		status = &utils.TransactionStatus{
			Finality:  receipt.FinalityStatus,
			Execution: receipt.ExecutionStatus,
		}
	case ErrTxnHashNotFound:
		txStatus, err := h.feederClient.Transaction(ctx, &hash)
		if err != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		// Check if the error is due to a transaction not being found
		if txStatus.Status == "NOT_RECEIVED" || txStatus.Finality == utils.NotReceived {
			return nil, ErrTxnHashNotFound
		}

		status = new(utils.TransactionStatus)

		switch txStatus.Finality {
		case utils.AcceptedOnL1:
			status.Finality = utils.AcceptedOnL1
		case utils.AcceptedOnL2:
			status.Finality = utils.AcceptedOnL2
		default:
			// pre-0.12.1
			if txStatus.Status == "ACCEPTED_ON_L1" {
				status.Finality = utils.AcceptedOnL1
			} else {
				status.Finality = utils.AcceptedOnL2
			}
		}

		switch txStatus.Execution {
		case utils.Succeeded:
			status.Execution = utils.Succeeded
		case utils.Reverted:
			status.Execution = utils.Reverted
		default:
			// pre-0.12.1
			status.Execution = utils.Succeeded
		}
	default:
		return nil, txErr
	}

	return status, nil
}

func (h *Handler) EstimateFee(broadcastedTxns []utils.BroadcastedTransaction, id utils.BlockID) ([]utils.FeeEstimate, *jsonrpc.Error) {
	result, err := h.SimulateTransactions(id, broadcastedTxns, []utils.SimulationFlag{utils.SkipFeeChargeFlag})
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx utils.SimulatedTransaction) utils.FeeEstimate {
		return tx.FeeEstimate
	}), nil
}

func (h *Handler) EstimateMessageFee(msg utils.MsgFromL1, id utils.BlockID) (*utils.FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	calldata := make([]*felt.Felt, 0, len(msg.Payload)+1)
	// The order of the calldata parameters matters. msg.From must be prepended.
	calldata = append(calldata, new(felt.Felt).SetBytes(msg.From.Bytes()))
	for payloadIdx := range msg.Payload {
		calldata = append(calldata, &msg.Payload[payloadIdx])
	}
	tx := utils.BroadcastedTransaction{
		Transaction: utils.Transaction{
			Type:               utils.TxnL1Handler,
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
	estimates, rpcErr := h.EstimateFee([]utils.BroadcastedTransaction{tx}, id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return &estimates[0], nil
}

// TraceTransaction returns the trace for a given executed transaction, including internal calls
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/1ae810e0137cc5d175ace4554892a4f43052be56/api/starknet_trace_api_openrpc.json#L11
func (h *Handler) TraceTransaction(hash felt.Felt) (json.RawMessage, *jsonrpc.Error) {
	_, _, blockNumber, err := h.bcReader.Receipt(&hash)
	if err != nil {
		return nil, ErrInvalidTxHash
	}

	block, err := h.bcReader.BlockByNumber(blockNumber)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	txIndex := utils.IndexFunc(block.Transactions, func(tx core.Transaction) bool {
		return tx.Hash().Equal(&hash)
	})
	if txIndex == -1 {
		return nil, ErrTxnHashNotFound
	}

	traceResults, traceBlockErr := h.traceBlockTransactions(block, txIndex+1)
	if traceBlockErr != nil {
		return nil, traceBlockErr
	}

	return traceResults[txIndex].TraceRoot, nil
}

func (h *Handler) SimulateTransactions(id utils.BlockID, transactions []utils.BroadcastedTransaction,
	simulationFlags []utils.SimulationFlag,
) ([]utils.SimulatedTransaction, *jsonrpc.Error) {
	skipValidate := utils.Any(simulationFlags, func(f utils.SimulationFlag) bool {
		return f == utils.SkipValidateFlag
	})
	if skipValidate {
		return nil, jsonrpc.Err(jsonrpc.InvalidParams, "Skip validate is not supported")
	}
	skipFeeCharge := utils.Any(simulationFlags, func(f utils.SimulationFlag) bool {
		return f == utils.SkipFeeChargeFlag
	})

	state, closer, err := h.stateByBlockID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_estimateFee")

	header, err := h.blockHeaderByID(&id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	var txns []core.Transaction
	var classes []core.Class

	paidFeesOnL1 := make([]*felt.Felt, 0)
	for idx := range transactions {
		txn, declaredClass, paidFeeOnL1, aErr := adaptBroadcastedTransaction(&transactions[idx], h.network)
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

	blockNumber := header.Number
	if id.Pending {
		height, hErr := h.bcReader.Height()
		if hErr != nil {
			return nil, ErrBlockNotFound
		}
		blockNumber = height + 1
	}

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = core.NetworkBlockHashMetaInfo(h.network).FallBackSequencerAddress
	}
	overallFees, traces, err := h.vm.Execute(txns, classes, blockNumber, header.Timestamp, sequencerAddress,
		state, h.network, paidFeesOnL1, skipFeeCharge, header.GasPrice)
	if err != nil {
		rpcErr := *ErrContractError
		rpcErr.Data = err.Error()
		return nil, &rpcErr
	}

	var result []utils.SimulatedTransaction
	for i, overallFee := range overallFees {
		estimate := utils.FeeEstimate{
			GasConsumed: new(felt.Felt).Div(overallFee, header.GasPrice),
			GasPrice:    header.GasPrice,
			OverallFee:  overallFee,
		}
		result = append(result, utils.SimulatedTransaction{
			TransactionTrace: traces[i],
			FeeEstimate:      estimate,
		})
	}

	return result, nil
}

func (h *Handler) TraceBlockTransactions(blockHash felt.Felt) ([]utils.TracedBlockTransaction, *jsonrpc.Error) {
	block, err := h.bcReader.BlockByHash(&blockHash)
	if err != nil {
		return nil, ErrInvalidBlockHash
	}

	return h.traceBlockTransactions(block, len(block.Transactions))
}

func (h *Handler) traceBlockTransactions(block *core.Block, numTxns int) ([]utils.TracedBlockTransaction, *jsonrpc.Error) {
	isPending := block.Hash == nil

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

	transactions := block.Transactions[:numTxns]
	for _, transaction := range transactions {
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

	blockNumber := block.Number
	if isPending {
		height, hErr := h.bcReader.Height()
		if hErr != nil {
			return nil, ErrBlockNotFound
		}
		blockNumber = height + 1
	}

	header := block.Header

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = core.NetworkBlockHashMetaInfo(h.network).FallBackSequencerAddress
	}

	_, traces, err := h.vm.Execute(transactions, classes, blockNumber, header.Timestamp,
		sequencerAddress, state, h.network, paidFeesOnL1, false, header.GasPrice)
	if err != nil {
		rpcErr := *ErrContractError
		rpcErr.Data = err.Error()
		return nil, &rpcErr
	}

	var result []utils.TracedBlockTransaction
	for i, trace := range traces {
		result = append(result, utils.TracedBlockTransaction{
			TraceRoot:       trace,
			TransactionHash: transactions[i].Hash(),
		})
	}

	return result, nil
}

func (h *Handler) callAndLogErr(f func() error, msg string) {
	if err := f(); err != nil {
		h.log.Errorw(msg, "err", err)
	}
}
