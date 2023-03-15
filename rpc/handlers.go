package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
)

var (
	ErrPendingNotSupported = errors.New("pending block is not supported yet")

	ErrBlockNotFound   = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrTxnHashNotFound = &jsonrpc.Error{Code: 25, Message: "Transaction hash not found"}
	ErrNoBlock         = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
	ErrInvalidTxIndex  = &jsonrpc.Error{Code: 27, Message: "Invalid transaction index in a block"}
)

type Handler struct {
	bcReader blockchain.Reader
	network  utils.Network
}

func New(bcReader blockchain.Reader, n utils.Network) *Handler {
	return &Handler{
		bcReader: bcReader,
		network:  n,
	}
}

func (h *Handler) ChainId() (*felt.Felt, *jsonrpc.Error) {
	return h.network.ChainId(), nil
}

func (h *Handler) BlockNumber() (uint64, *jsonrpc.Error) {
	num, err := h.bcReader.Height()
	if err != nil {
		return 0, ErrNoBlock
	}

	return num, nil
}

func (h *Handler) BlockNumberAndHash() (*BlockNumberAndHash, *jsonrpc.Error) {
	if block, err := h.bcReader.Head(); err != nil {
		return nil, ErrNoBlock
	} else {
		return &BlockNumberAndHash{Number: block.Number, Hash: block.Hash}, nil
	}
}

func (h *Handler) BlockWithTxHashes(id *BlockId) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, err := h.blockById(id)
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

func (h *Handler) BlockWithTxs(id *BlockId) (*BlockWithTxs, *jsonrpc.Error) {
	block, err := h.blockById(id)
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
		Type:      TxnInvoke,
		Hash:      t.Hash(),
		MaxFee:    t.MaxFee,
		Version:   t.Version,
		Signature: &sig,
		Nonce:     t.Nonce,
		Calldata:  &t.CallData,
	}

	if t.Version.IsZero() {
		invTxn.ContractAddress = t.ContractAddress
		invTxn.EntryPointSelector = t.EntryPointSelector
	} else if t.Version.IsOne() {
		invTxn.SenderAddress = t.ContractAddress
	} else {
		panic("invalid invoke txn version")
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
	} else if !t.Version.IsZero() && !t.Version.IsOne() {
		panic("invalid invoke txn version")
	}
	return txn
}

func (h *Handler) blockById(id *BlockId) (*core.Block, error) {
	if id.Latest {
		return h.bcReader.Head()
	} else if id.Hash != nil {
		return h.bcReader.BlockByHash(id.Hash)
	} else if id.Pending {
		return nil, ErrPendingNotSupported
	} else {
		return h.bcReader.BlockByNumber(id.Number)
	}
}

func (h *Handler) blockHeaderById(id *BlockId) (*core.Header, error) {
	if id.Latest {
		return h.bcReader.HeadsHeader()
	} else if id.Hash != nil {
		return h.bcReader.BlockHeaderByHash(id.Hash)
	} else if id.Pending {
		return nil, ErrPendingNotSupported
	} else {
		return h.bcReader.BlockHeaderByNumber(id.Number)
	}
}

// TransactionByHash https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L158
func (h *Handler) TransactionByHash(hash *felt.Felt) (*Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	return adaptTransaction(txn), nil
}

// BlockTransactionCount https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L373
func (h *Handler) BlockTransactionCount(id *BlockId) (uint64, *jsonrpc.Error) {
	header, err := h.blockHeaderById(id)
	if header == nil || err != nil {
		return 0, ErrBlockNotFound
	}
	return header.TransactionCount, nil
}

// TransactionByBlockIdAndIndex https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L184
func (h *Handler) TransactionByBlockIdAndIndex(id *BlockId, txIndex int) (*Transaction, *jsonrpc.Error) {
	header, err := h.blockHeaderById(id)
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

// TransactionReceiptByHash https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
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

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L77
func (h *Handler) StateUpdate(id *BlockId) (*StateUpdate, *jsonrpc.Error) {
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

	nonces := []Nonce{}
	for addr, nonce := range update.StateDiff.Nonces {
		nonces = append(nonces, Nonce{ContractAddress: new(felt.Felt).Set(&addr), Nonce: nonce})
	}

	storageDiffs := []StorageDiff{}
	for addr, diffs := range update.StateDiff.StorageDiffs {
		entries := make([]Entry, len(diffs))

		for index, diff := range diffs {
			entries[index] = Entry{Key: diff.Key, Value: diff.Value}
		}

		storageDiffs = append(storageDiffs, StorageDiff{Address: new(felt.Felt).Set(&addr), StorageEntries: entries})
	}

	deployedContracts := make([]DeployedContract, len(update.StateDiff.DeployedContracts))
	for index, deployedContract := range update.StateDiff.DeployedContracts {
		deployedContracts[index] = DeployedContract{Address: deployedContract.Address, ClassHash: deployedContract.ClassHash}
	}

	return &StateUpdate{
		BlockHash: update.BlockHash,
		OldRoot:   update.OldRoot,
		NewRoot:   update.NewRoot,
		StateDiff: &StateDiff{
			DeclaredClasses:   update.StateDiff.DeclaredClasses,
			Nonces:            nonces,
			StorageDiffs:      storageDiffs,
			DeployedContracts: deployedContracts,
		},
	}, nil
}
