package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
)

var (
	ErrBlockNotFound   = &jsonrpc.Error{Code: 24, Message: "Block not found"}
	ErrTxnHashNotFound = &jsonrpc.Error{Code: 25, Message: "Transaction hash not found"}
	ErrNoBlock         = &jsonrpc.Error{Code: 32, Message: "There are no blocks"}
)

type Handler struct {
	bcReader blockchain.Reader

	chainId *felt.Felt
}

func New(bcReader blockchain.Reader, chainId *felt.Felt) *Handler {
	return &Handler{
		bcReader: bcReader,
		chainId:  chainId,
	}
}

func (h *Handler) ChainId() (*felt.Felt, *jsonrpc.Error) {
	return h.chainId, nil
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

func (h *Handler) GetBlockWithTxHashes(id *BlockId) (*BlockWithTxHashes, *jsonrpc.Error) {
	block, err := h.getBlockById(id)
	if err != nil || block == nil {
		return nil, ErrBlockNotFound
	}

	txnHashes := make([]*felt.Felt, len(block.Transactions))
	for index, txn := range block.Transactions {
		txnHashes[index] = txn.Hash()
	}

	return &BlockWithTxHashes{
		Status:      BlockStatusAcceptedL2, // todo
		BlockHeader: adaptBlockHeader(&block.Header),
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

func (h *Handler) GetBlockWithTxs(id *BlockId) (*BlockWithTxs, *jsonrpc.Error) {
	block, err := h.getBlockById(id)
	if err != nil || block == nil {
		return nil, ErrBlockNotFound
	}

	txs := make([]*Transaction, len(block.Transactions))
	for index, txn := range block.Transactions {
		txs[index] = adaptTransaction(txn)
	}

	return &BlockWithTxs{
		Status:       BlockStatusAcceptedL2, // todo
		BlockHeader:  adaptBlockHeader(&block.Header),
		Transactions: txs,
	}, nil
}

func adaptTransaction(t core.Transaction) *Transaction {
	switch v := t.(type) {
	case *core.DeployTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1521
		return &Transaction{
			Type:                "DEPLOY",
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
			Type:                "DEPLOY_ACCOUNT",
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCalldata: v.ConstructorCallData,
			ClassHash:           v.ClassHash,
			ContractAddress:     v.ContractAddress,
		}
	case *core.L1HandlerTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1669
		return &Transaction{
			Type:               "L1_HANDLER",
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
		Type:      "INVOKE",
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
		Type:          "DECLARE",
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

func (h *Handler) getBlockById(id *BlockId) (*core.Block, error) {
	var block *core.Block
	var err error
	if id.Latest {
		block, err = h.bcReader.Head()
	} else if id.Pending {
		err = errors.New("pending block is not supported yet")
	} else if id.Hash != nil {
		block, err = h.bcReader.GetBlockByHash(id.Hash)
	} else {
		block, err = h.bcReader.GetBlockByNumber(id.Number)
	}

	return block, err
}

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L158
func (h *Handler) GetTransactionByHash(hash *felt.Felt) (*Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.GetTransactionByHash(hash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	return adaptTransaction(txn), nil
}
