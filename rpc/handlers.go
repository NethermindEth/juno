package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
)

var (
	ErrPendingNotSupported = errors.New("pending block is not supported yet")

	ErrBlockNotFound     = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrContractNotFound  = &jsonrpc.Error{Code: 20, Message: "Contract not found"}
	ErrTxnHashNotFound   = &jsonrpc.Error{Code: 25, Message: "Transaction hash not found"}
	ErrNoBlock           = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidTxIndex    = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
	ErrClassHashNotFound = &jsonrpc.Error{Code: 28, Message: "Class hash not found"}
)

type Handler struct {
	bcReader     blockchain.Reader
	synchronizer *sync.Synchronizer
	network      utils.Network
	log          utils.Logger
}

func New(bcReader blockchain.Reader, synchronizer *sync.Synchronizer, n utils.Network, logger utils.Logger) *Handler {
	return &Handler{
		bcReader:     bcReader,
		synchronizer: synchronizer,
		network:      n,
		log:          logger,
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
func (h *Handler) BlockWithTxHashes(id *BlockID) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, err := h.blockByID(id)
	if block == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	return &BlockWithTxHashes{
		Status:      StatusAcceptedL2, // todo
		BlockHeader: adaptBlockHeader(block.Header),
		TxnHashes:   txnHashes,
	}, nil
}

func adaptBlockHeader(header *core.Header) BlockHeader {
	return BlockHeader{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           header.Number,
		NewRoot:          header.GlobalStateRoot,
		Timestamp:        header.Timestamp,
		SequencerAddress: header.SequencerAddress,
	}
}

// BlockWithTxs returns the block information with full transactions given a block ID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L44
func (h *Handler) BlockWithTxs(id *BlockID) (*BlockWithTxs, *jsonrpc.Error) {
	block, err := h.blockByID(id)
	if block == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = adaptTransaction(txn)
	}

	return &BlockWithTxs{
		Status:       StatusAcceptedL2, // todo
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
		Type:          TxnDeclare,
		Hash:          t.Hash(),
		MaxFee:        t.MaxFee,
		Version:       t.Version,
		Signature:     &sig,
		Nonce:         t.Nonce,
		ClassHash:     t.ClassHash,
		SenderAddress: t.SenderAddress,
	}

	if t.Version.Equal(new(felt.Felt).SetUint64(2)) {
		txn.CompiledClassHash = nil // todo: add when we have support for Declare V2
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
		return nil, ErrPendingNotSupported
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
		return nil, ErrPendingNotSupported
	default:
		return h.bcReader.BlockHeaderByNumber(id.Number)
	}
}

// TransactionByHash returns the details of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L158
func (h *Handler) TransactionByHash(hash *felt.Felt) (*Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(hash)
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
func (h *Handler) BlockTransactionCount(id *BlockID) (uint64, *jsonrpc.Error) {
	header, err := h.blockHeaderByID(id)
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
func (h *Handler) TransactionByBlockIDAndIndex(id *BlockID, txIndex int) (*Transaction, *jsonrpc.Error) {
	header, err := h.blockHeaderByID(id)
	if header == nil || err != nil {
		return nil, ErrBlockNotFound
	}

	if txIndex < 0 {
		return nil, ErrInvalidTxIndex
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
func (h *Handler) TransactionReceiptByHash(hash *felt.Felt) (*TransactionReceipt, *jsonrpc.Error) {
	txn, rpcErr := h.TransactionByHash(hash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	receipt, blockHash, blockNumber, err := h.bcReader.Receipt(hash)
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

	return &TransactionReceipt{
		Status:          StatusAcceptedL2, // todo
		Type:            txn.Type,
		Hash:            txn.Hash,
		ActualFee:       receipt.Fee,
		BlockHash:       blockHash,
		BlockNumber:     blockNumber,
		MessagesSent:    messages,
		Events:          events,
		ContractAddress: contractAddress,
	}, nil
}

// StateUpdate returns the state update identified by the given BlockID.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L77
func (h *Handler) StateUpdate(id *BlockID) (*StateUpdate, *jsonrpc.Error) {
	var update *core.StateUpdate
	var err error
	if id.Latest {
		if height, heightErr := h.bcReader.Height(); heightErr != nil {
			err = heightErr
		} else {
			update, err = h.bcReader.StateUpdateByNumber(height)
		}
	} else if id.Pending {
		err = ErrPendingNotSupported
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
	if highestBlockHeader.Number < head.Number {
		return defaultSyncState, nil
	}
	return &Sync{
		StartingBlockHash:   startingBlockHeader.Hash,
		StartingBlockNumber: NumAsHex(startingBlockHeader.Number),
		CurrentBlockHash:    head.Hash,
		CurrentBlockNumber:  NumAsHex(head.Number),
		HighestBlockHash:    highestBlockHeader.Hash,
		HighestBlockNumber:  NumAsHex(highestBlockHeader.Number),
	}, nil
}

func (h *Handler) stateByBlockID(id *BlockID) (core.StateReader, blockchain.StateCloser, error) {
	switch {
	case id.Latest:
		return h.bcReader.HeadState()
	case id.Hash != nil:
		return h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		return nil, nil, ErrPendingNotSupported
	default:
		return h.bcReader.StateAtBlockNumber(id.Number)
	}
}

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id *BlockID, address *felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	nonce, err := stateReader.ContractNonce(address)
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
func (h *Handler) StorageAt(address, key *felt.Felt, id *BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	value, err := stateReader.ContractStorage(address, key)
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
func (h *Handler) ClassHashAt(id *BlockID, address *felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, err := h.stateByBlockID(id)
	if err != nil {
		return nil, ErrBlockNotFound
	}

	classHash, err := stateReader.ContractClassHash(address)
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
func (h *Handler) Class(id *BlockID, classHash *felt.Felt) (*Class, *jsonrpc.Error) {
	state, stateCloser, err := h.stateByBlockID(id)
	if err != nil {
		return nil, ErrBlockNotFound
	}
	declared, err := state.Class(classHash)
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
func (h *Handler) ClassAt(id *BlockID, address *felt.Felt) (*Class, *jsonrpc.Error) {
	classHash, err := h.ClassHashAt(id, address)
	if err != nil {
		return nil, err
	}
	return h.Class(id, classHash)
}
