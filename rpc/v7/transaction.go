package rpcv7

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type TransactionType uint8

const (
	Invalid TransactionType = iota
	TxnDeclare
	TxnDeploy
	TxnDeployAccount
	TxnInvoke
	TxnL1Handler
)

func (t TransactionType) String() string {
	switch t {
	case TxnDeclare:
		return "DECLARE"
	case TxnDeploy:
		return "DEPLOY"
	case TxnDeployAccount:
		return "DEPLOY_ACCOUNT"
	case TxnInvoke:
		return "INVOKE"
	case TxnL1Handler:
		return "L1_HANDLER"
	default:
		return "<unknown>"
	}
}

func (t TransactionType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *TransactionType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"DECLARE"`:
		*t = TxnDeclare
	case `"DEPLOY"`:
		*t = TxnDeploy
	case `"DEPLOY_ACCOUNT"`:
		*t = TxnDeployAccount
	case `"INVOKE"`, `"INVOKE_FUNCTION"`:
		*t = TxnInvoke
	case `"L1_HANDLER"`:
		*t = TxnL1Handler
	default:
		return errors.New("unknown TransactionType")
	}
	return nil
}

type TxnStatus uint8

const (
	TxnStatusReceived TxnStatus = iota + 1
	TxnStatusRejected
	TxnStatusAcceptedOnL2
	TxnStatusAcceptedOnL1
)

func (s TxnStatus) MarshalText() ([]byte, error) {
	switch s {
	case TxnStatusReceived:
		return []byte("RECEIVED"), nil
	case TxnStatusRejected:
		return []byte("REJECTED"), nil
	case TxnStatusAcceptedOnL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case TxnStatusAcceptedOnL2:
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown ExecutionStatus %v", s)
	}
}

type TxnExecutionStatus uint8

const (
	TxnSuccess TxnExecutionStatus = iota + 1
	TxnFailure
)

func (es TxnExecutionStatus) MarshalText() ([]byte, error) {
	switch es {
	case TxnSuccess:
		return []byte("SUCCEEDED"), nil
	case TxnFailure:
		return []byte("REVERTED"), nil
	default:
		return nil, fmt.Errorf("unknown ExecutionStatus %v", es)
	}
}

type TxnFinalityStatus uint8

const (
	TxnAcceptedOnL2 TxnFinalityStatus = iota + 3
	TxnAcceptedOnL1
)

func (fs TxnFinalityStatus) MarshalText() ([]byte, error) {
	switch fs {
	case TxnAcceptedOnL1:
		return []byte("ACCEPTED_ON_L1"), nil
	case TxnAcceptedOnL2:
		return []byte("ACCEPTED_ON_L2"), nil
	default:
		return nil, fmt.Errorf("unknown FinalityStatus %v", fs)
	}
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

func (m DataAvailabilityMode) MarshalText() ([]byte, error) {
	switch m {
	case DAModeL1:
		return []byte("L1"), nil
	case DAModeL2:
		return []byte("L2"), nil
	default:
		return nil, fmt.Errorf("unknown DataAvailabilityMode %v", m)
	}
}

func (m *DataAvailabilityMode) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"L1"`:
		*m = DAModeL1
	case `"L2"`:
		*m = DAModeL2
	default:
		return fmt.Errorf("unknown DataAvailabilityMode: %q", string(data))
	}
	return nil
}

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
)

func (r Resource) MarshalText() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("l1_gas"), nil
	case ResourceL2Gas:
		return []byte("l2_gas"), nil
	default:
		return nil, fmt.Errorf("unknown Resource %v", r)
	}
}

func (r *Resource) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"l1_gas"`:
		*r = ResourceL1Gas
	case `"l2_gas"`:
		*r = ResourceL2Gas
	default:
		return fmt.Errorf("unknown Resource: %q", string(data))
	}
	return nil
}

func (r *Resource) UnmarshalText(data []byte) error {
	return r.UnmarshalJSON(data)
}

type ResourceBounds struct {
	MaxAmount       *felt.Felt `json:"max_amount"`
	MaxPricePerUnit *felt.Felt `json:"max_price_per_unit"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
//
//nolint:lll
type Transaction struct {
	Hash                  *felt.Felt                   `json:"transaction_hash,omitempty"`
	Type                  TransactionType              `json:"type" validate:"required"`
	Version               *felt.Felt                   `json:"version,omitempty" validate:"required"`
	Nonce                 *felt.Felt                   `json:"nonce,omitempty" validate:"required_unless=Version 0x0"`
	MaxFee                *felt.Felt                   `json:"max_fee,omitempty" validate:"required_if=Version 0x0,required_if=Version 0x1,required_if=Version 0x2"`
	ContractAddress       *felt.Felt                   `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt                   `json:"contract_address_salt,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ClassHash             *felt.Felt                   `json:"class_hash,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ConstructorCallData   *[]*felt.Felt                `json:"constructor_calldata,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	SenderAddress         *felt.Felt                   `json:"sender_address,omitempty" validate:"required_if=Type DECLARE,required_if=Type INVOKE Version 0x1,required_if=Type INVOKE Version 0x3"`
	Signature             *[]*felt.Felt                `json:"signature,omitempty" validate:"required"`
	CallData              *[]*felt.Felt                `json:"calldata,omitempty" validate:"required_if=Type INVOKE"`
	EntryPointSelector    *felt.Felt                   `json:"entry_point_selector,omitempty" validate:"required_if=Type INVOKE Version 0x0"`
	CompiledClassHash     *felt.Felt                   `json:"compiled_class_hash,omitempty" validate:"required_if=Type DECLARE Version 0x2"`
	ResourceBounds        *map[Resource]ResourceBounds `json:"resource_bounds,omitempty" validate:"required_if=Version 0x3"`
	Tip                   *felt.Felt                   `json:"tip,omitempty" validate:"required_if=Version 0x3"`
	PaymasterData         *[]*felt.Felt                `json:"paymaster_data,omitempty" validate:"required_if=Version 0x3"`
	AccountDeploymentData *[]*felt.Felt                `json:"account_deployment_data,omitempty" validate:"required_if=Type INVOKE Version 0x3,required_if=Type DECLARE Version 0x3"`
	NonceDAMode           *DataAvailabilityMode        `json:"nonce_data_availability_mode,omitempty" validate:"required_if=Version 0x3"`
	FeeDAMode             *DataAvailabilityMode        `json:"fee_data_availability_mode,omitempty" validate:"required_if=Version 0x3"`
}

type TransactionStatus struct {
	Finality      TxnStatus          `json:"finality_status"`
	Execution     TxnExecutionStatus `json:"execution_status,omitempty"`
	FailureReason string             `json:"failure_reason,omitempty"`
}

type MsgToL1 struct {
	From    *felt.Felt     `json:"from_address,omitempty"`
	To      common.Address `json:"to_address"`
	Payload []*felt.Felt   `json:"payload"`
}

type ComputationResources struct {
	Steps        uint64 `json:"steps"`
	MemoryHoles  uint64 `json:"memory_holes,omitempty"`
	Pedersen     uint64 `json:"pedersen_builtin_applications,omitempty"`
	RangeCheck   uint64 `json:"range_check_builtin_applications,omitempty"`
	Bitwise      uint64 `json:"bitwise_builtin_applications,omitempty"`
	Ecdsa        uint64 `json:"ecdsa_builtin_applications,omitempty"`
	EcOp         uint64 `json:"ec_op_builtin_applications,omitempty"`
	Keccak       uint64 `json:"keccak_builtin_applications,omitempty"`
	Poseidon     uint64 `json:"poseidon_builtin_applications,omitempty"`
	SegmentArena uint64 `json:"segment_arena_builtin,omitempty"`
}

type DataAvailability struct {
	L1Gas     uint64 `json:"l1_gas"`
	L1DataGas uint64 `json:"l1_data_gas"`
}

type ExecutionResources struct {
	ComputationResources
	DataAvailability *DataAvailability `json:"data_availability"`
}

// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L1871
type TransactionReceipt struct {
	Type               TransactionType     `json:"type"`
	Hash               *felt.Felt          `json:"transaction_hash"`
	ActualFee          *FeePayment         `json:"actual_fee"`
	ExecutionStatus    TxnExecutionStatus  `json:"execution_status"`
	FinalityStatus     TxnFinalityStatus   `json:"finality_status"`
	BlockHash          *felt.Felt          `json:"block_hash,omitempty"`
	BlockNumber        *uint64             `json:"block_number,omitempty"`
	MessagesSent       []*MsgToL1          `json:"messages_sent"`
	Events             []*Event            `json:"events"`
	ContractAddress    *felt.Felt          `json:"contract_address,omitempty"`
	RevertReason       string              `json:"revert_reason,omitempty"`
	ExecutionResources *ExecutionResources `json:"execution_resources,omitempty"`
	MessageHash        string              `json:"message_hash,omitempty"`
}

type FeePayment struct {
	Amount *felt.Felt `json:"amount"`
	Unit   FeeUnit    `json:"unit"`
}

type AddTxResponse struct {
	TransactionHash *felt.Felt `json:"transaction_hash"`
	ContractAddress *felt.Felt `json:"contract_address,omitempty"`
	ClassHash       *felt.Felt `json:"class_hash,omitempty"`
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1273-L1287
type BroadcastedTransaction struct {
	Transaction
	ContractClass json.RawMessage `json:"contract_class,omitempty" validate:"required_if=Transaction.Type DECLARE"`
	PaidFeeOnL1   *felt.Felt      `json:"paid_fee_on_l1,omitempty" validate:"required_if=Transaction.Type L1_HANDLER"`
}

/****************************************************
		Transaction Handlers
*****************************************************/

// TransactionByBlockIDAndIndex returns the details of a transaction identified by the given
// BlockID and index.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L184
func (h *Handler) TransactionByBlockIDAndIndex(id BlockID, txIndex int) (*Transaction, *jsonrpc.Error) {
	if txIndex < 0 {
		return nil, rpccore.ErrInvalidTxIndex
	}

	if id.Pending {
		pending, err := h.syncReader.Pending()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}

		if uint64(txIndex) > pending.Block.TransactionCount {
			return nil, rpccore.ErrInvalidTxIndex
		}

		return utils.HeapPtr(AdaptCoreTransaction(pending.Block.Transactions[txIndex])), nil
	}

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(header.Number, uint64(txIndex))
	if err != nil {
		return nil, rpccore.ErrInvalidTxIndex
	}

	return utils.HeapPtr(AdaptCoreTransaction(txn)), nil
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHash(hash felt.Felt) (*TransactionReceipt, *jsonrpc.Error) {
	var (
		pendingB      *core.Block
		pendingBIndex int
	)

	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}

		pendingB = h.syncReader.PendingBlock()
		if pendingB == nil {
			return nil, rpccore.ErrTxnHashNotFound
		}

		for i, t := range pendingB.Transactions {
			if hash.Equal(t.Hash()) {
				pendingBIndex = i
				txn = t
				break
			}
		}

		if txn == nil {
			return nil, rpccore.ErrTxnHashNotFound
		}
	}

	var (
		receipt     *core.TransactionReceipt
		blockHash   *felt.Felt
		blockNumber uint64
	)

	if pendingB != nil {
		receipt = pendingB.Receipts[pendingBIndex]
	} else {
		receipt, blockHash, blockNumber, err = h.bcReader.Receipt(&hash)
		if err != nil {
			return nil, rpccore.ErrTxnHashNotFound
		}
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

	return utils.HeapPtr(AdaptCoreReceipt(receipt, txn, status, blockHash, blockNumber)), nil
}

var errTransactionNotFound = errors.New("transaction not found")

func (h *Handler) TransactionStatus(ctx context.Context, hash felt.Felt) (*TransactionStatus, *jsonrpc.Error) {
	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		return &TransactionStatus{
			Finality:      TxnStatus(receipt.FinalityStatus),
			Execution:     receipt.ExecutionStatus,
			FailureReason: receipt.RevertReason,
		}, nil
	case rpccore.ErrTxnHashNotFound:
		if h.feederClient == nil {
			break
		}

		txStatus, err := h.feederClient.Transaction(ctx, &hash)
		if err != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}

		status, err := adaptFeederTransactionStatus(txStatus)
		if err != nil {
			if !errors.Is(err, errTransactionNotFound) {
				h.log.Errorw("Failed to adapt transaction status", "err", err)
			}
			return nil, rpccore.ErrTxnHashNotFound
		}
		return status, nil
	}
	return nil, txErr
}

// In 0.7.0, the failure reason is not returned in the TransactionStatus response.
type TransactionStatusV0_7 struct {
	Finality  TxnStatus          `json:"finality_status"`
	Execution TxnExecutionStatus `json:"execution_status,omitempty"`
}

func (h *Handler) TransactionStatusV0_7(ctx context.Context, hash felt.Felt) (*TransactionStatusV0_7, *jsonrpc.Error) {
	res, err := h.TransactionStatus(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &TransactionStatusV0_7{
		Finality:  res.Finality,
		Execution: res.Execution,
	}, nil
}
