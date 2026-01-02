package rpcv8

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
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
	ResourceL1DataGas
)

func (r Resource) MarshalText() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("l1_gas"), nil
	case ResourceL2Gas:
		return []byte("l2_gas"), nil
	case ResourceL1DataGas:
		return []byte("l1_data_gas"), nil
	default:
		return nil, fmt.Errorf("unknown Resource %v", r)
	}
}

func (r *Resource) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch strings.ToLower(strings.Trim(str, `"`)) {
	case "l1_gas":
		*r = ResourceL1Gas
	case "l2_gas":
		*r = ResourceL2Gas
	case "l1_data_gas":
		*r = ResourceL1DataGas
	default:
		return fmt.Errorf("unknown Resource: %q", str)
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

// TODO: using Value fields here is a good idea, however
// we are currently keeping the field's type Reference since the current
// validation tags we are using does not work well with Value field.
// We should revisit this when we start implementing custom validations.
type ResourceBoundsMap struct {
	L1Gas     *ResourceBounds `json:"l1_gas" validate:"required"`
	L2Gas     *ResourceBounds `json:"l2_gas" validate:"required"`
	L1DataGas *ResourceBounds `json:"l1_data_gas" validate:"required"`
}

func (r *ResourceBoundsMap) MarshalJSON() ([]byte, error) {
	// Check if L1DataGas is nil, if it is, provide default values
	if r.L1DataGas == nil {
		r.L1DataGas = &ResourceBounds{
			MaxAmount:       &felt.Zero,
			MaxPricePerUnit: &felt.Zero,
		}
	}

	// Define an alias to avoid recursion
	type alias ResourceBoundsMap
	return json.Marshal((*alias)(r))
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1252
//
//nolint:lll
type Transaction struct {
	Hash                  *felt.Felt            `json:"transaction_hash,omitempty"`
	Type                  TransactionType       `json:"type" validate:"required"`
	Version               *felt.Felt            `json:"version,omitempty" validate:"required,version_0x3"`
	Nonce                 *felt.Felt            `json:"nonce,omitempty" validate:"required"`
	MaxFee                *felt.Felt            `json:"max_fee,omitempty"`
	ContractAddress       *felt.Felt            `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt            `json:"contract_address_salt,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ClassHash             *felt.Felt            `json:"class_hash,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	ConstructorCallData   *[]*felt.Felt         `json:"constructor_calldata,omitempty" validate:"required_if=Type DEPLOY,required_if=Type DEPLOY_ACCOUNT"`
	SenderAddress         *felt.Felt            `json:"sender_address,omitempty" validate:"required_if=Type DECLARE,required_if=Type INVOKE"`
	Signature             *[]*felt.Felt         `json:"signature,omitempty" validate:"required"`
	CallData              *[]*felt.Felt         `json:"calldata,omitempty" validate:"required_if=Type INVOKE"`
	EntryPointSelector    *felt.Felt            `json:"entry_point_selector,omitempty"`
	CompiledClassHash     *felt.Felt            `json:"compiled_class_hash,omitempty"`
	ResourceBounds        *ResourceBoundsMap    `json:"resource_bounds,omitempty" validate:"resource_bounds_required"`
	Tip                   *felt.Felt            `json:"tip,omitempty" validate:"required"`
	PaymasterData         *[]*felt.Felt         `json:"paymaster_data,omitempty" validate:"required"`
	AccountDeploymentData *[]*felt.Felt         `json:"account_deployment_data,omitempty" validate:"required_if=Type INVOKE,required_if=Type DECLARE"`
	NonceDAMode           *DataAvailabilityMode `json:"nonce_data_availability_mode,omitempty" validate:"required"`
	FeeDAMode             *DataAvailabilityMode `json:"fee_data_availability_mode,omitempty" validate:"required"`
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
	Ecsda        uint64 `json:"ecdsa_builtin_applications,omitempty"`
	EcOp         uint64 `json:"ec_op_builtin_applications,omitempty"`
	Keccak       uint64 `json:"keccak_builtin_applications,omitempty"`
	Poseidon     uint64 `json:"poseidon_builtin_applications,omitempty"`
	SegmentArena uint64 `json:"segment_arena_builtin,omitempty"`
}

type ExecutionResources struct {
	InnerExecutionResources
	L1DataGas uint64 `json:"l1_data_gas"`
}

type InnerExecutionResources struct {
	L1Gas uint64 `json:"l1_gas"`
	L2Gas uint64 `json:"l2_gas"`
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

func AdaptBroadcastedTransaction(broadcastedTxn *BroadcastedTransaction,
	network *utils.Network,
) (core.Transaction, core.ClassDefinition, *felt.Felt, error) {
	feederTxn := adaptRPCTxToFeederTx(&broadcastedTxn.Transaction)

	txn, err := sn2core.AdaptTransaction(feederTxn)
	if err != nil {
		return nil, nil, nil, err
	}

	var declaredClass core.ClassDefinition
	if len(broadcastedTxn.ContractClass) != 0 {
		declaredClass, err = adaptDeclaredClass(broadcastedTxn.ContractClass)
		if err != nil {
			return nil, nil, nil, err
		}
	} else if broadcastedTxn.Type == TxnDeclare {
		return nil, nil, nil, errors.New("declare without a class definition")
	}

	if t, ok := txn.(*core.DeclareTransaction); ok {
		classHash, err := declaredClass.Hash()
		if err != nil {
			return nil, nil, nil, err
		}
		t.ClassHash = &classHash
	}

	txnHash, err := core.TransactionHash(txn, network)
	if err != nil {
		return nil, nil, nil, err
	}

	var paidFeeOnL1 *felt.Felt
	switch t := txn.(type) {
	case *core.DeclareTransaction:
		t.TransactionHash = &txnHash
	case *core.InvokeTransaction:
		t.TransactionHash = &txnHash
	case *core.DeployAccountTransaction:
		t.TransactionHash = &txnHash
	case *core.L1HandlerTransaction:
		t.TransactionHash = &txnHash
		paidFeeOnL1 = broadcastedTxn.PaidFeeOnL1
	default:
		return nil, nil, nil, errors.New("unsupported transaction")
	}

	if txn.Hash() == nil {
		return nil, nil, nil, errors.New("deprecated transaction type")
	}
	return txn, declaredClass, paidFeeOnL1, nil
}

func adaptResourceBounds(rb map[core.Resource]core.ResourceBounds) ResourceBoundsMap {
	// Check if L1DataGas exists in the map
	var l1DataGasResourceBounds *ResourceBounds
	if _, ok := rb[core.ResourceL1DataGas]; ok {
		l1DataGasResourceBounds = &ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(rb[core.ResourceL1DataGas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL1DataGas].MaxPricePerUnit,
		}
	} else {
		l1DataGasResourceBounds = &ResourceBounds{
			MaxAmount:       &felt.Zero,
			MaxPricePerUnit: &felt.Zero,
		}
	}

	// As L1Gas & L2Gas will always be present, we can directly assign them
	rpcResourceBounds := ResourceBoundsMap{
		L1Gas: &ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(rb[core.ResourceL1Gas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL1Gas].MaxPricePerUnit,
		},
		L2Gas: &ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(rb[core.ResourceL2Gas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL2Gas].MaxPricePerUnit,
		},
		L1DataGas: l1DataGasResourceBounds,
	}
	return rpcResourceBounds
}

func adaptToFeederResourceBounds(rb *ResourceBoundsMap) *map[starknet.Resource]starknet.ResourceBounds { //nolint:gocritic
	if rb == nil {
		return nil
	}
	feederResourceBounds := make(map[starknet.Resource]starknet.ResourceBounds)
	feederResourceBounds[starknet.ResourceL1Gas] = starknet.ResourceBounds{
		MaxAmount:       rb.L1Gas.MaxAmount,
		MaxPricePerUnit: rb.L1Gas.MaxPricePerUnit,
	}
	feederResourceBounds[starknet.ResourceL2Gas] = starknet.ResourceBounds{
		MaxAmount:       rb.L2Gas.MaxAmount,
		MaxPricePerUnit: rb.L2Gas.MaxPricePerUnit,
	}
	feederResourceBounds[starknet.ResourceL1DataGas] = starknet.ResourceBounds{
		MaxAmount:       rb.L1DataGas.MaxAmount,
		MaxPricePerUnit: rb.L1DataGas.MaxPricePerUnit,
	}

	return &feederResourceBounds
}

func adaptToFeederDAMode(mode *DataAvailabilityMode) *starknet.DataAvailabilityMode {
	if mode == nil {
		return nil
	}
	return utils.HeapPtr(starknet.DataAvailabilityMode(*mode))
}

func adaptRPCTxToFeederTx(rpcTx *Transaction) *starknet.Transaction {
	return &starknet.Transaction{
		Hash:                  rpcTx.Hash,
		Version:               rpcTx.Version,
		ContractAddress:       rpcTx.ContractAddress,
		ContractAddressSalt:   rpcTx.ContractAddressSalt,
		ClassHash:             rpcTx.ClassHash,
		ConstructorCallData:   rpcTx.ConstructorCallData,
		Type:                  starknet.TransactionType(rpcTx.Type),
		SenderAddress:         rpcTx.SenderAddress,
		MaxFee:                rpcTx.MaxFee,
		Signature:             rpcTx.Signature,
		CallData:              rpcTx.CallData,
		EntryPointSelector:    rpcTx.EntryPointSelector,
		Nonce:                 rpcTx.Nonce,
		CompiledClassHash:     rpcTx.CompiledClassHash,
		ResourceBounds:        adaptToFeederResourceBounds(rpcTx.ResourceBounds),
		Tip:                   rpcTx.Tip,
		NonceDAMode:           adaptToFeederDAMode(rpcTx.NonceDAMode),
		FeeDAMode:             adaptToFeederDAMode(rpcTx.FeeDAMode),
		AccountDeploymentData: rpcTx.AccountDeploymentData,
		PaymasterData:         rpcTx.PaymasterData,
	}
}

/****************************************************
		Transaction Handlers
*****************************************************/

// TransactionByHash returns the details of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L158
func (h *Handler) TransactionByHash(hash felt.Felt) (*Transaction, *jsonrpc.Error) {
	txn, err := h.bcReader.TransactionByHash(&hash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}

		pendingB := h.PendingBlock()
		if pendingB == nil {
			return nil, rpccore.ErrTxnHashNotFound
		}

		for _, t := range pendingB.Transactions {
			if hash.Equal(t.Hash()) {
				txn = t
				break
			}
		}

		if txn == nil {
			return nil, rpccore.ErrTxnHashNotFound
		}
	}
	return AdaptTransaction(txn), nil
}

// TransactionByBlockIDAndIndex returns the details of a transaction identified by the given
// BlockID and index.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L184
func (h *Handler) TransactionByBlockIDAndIndex(
	blockID *BlockID, txIndex int,
) (*Transaction, *jsonrpc.Error) {
	if txIndex < 0 {
		return nil, rpccore.ErrInvalidTxIndex
	}

	var blockNumber uint64
	var err error
	switch blockID.Type() {
	case pending:
		pending, err := h.PendingData()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}

		if uint64(txIndex) >= pending.GetBlock().TransactionCount {
			return nil, rpccore.ErrInvalidTxIndex
		}

		return AdaptTransaction(pending.GetBlock().Transactions[txIndex]), nil
	case latest:
		header, err := h.bcReader.HeadsHeader()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}
		blockNumber = header.Number
	case hash:
		blockNumber, err = h.bcReader.BlockNumberByHash(blockID.Hash())
	case number:
		blockNumber = blockID.Number()
	default:
		panic("unknown block type id")
	}

	if err != nil {
		return nil, rpccore.ErrBlockNotFound
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(blockNumber, uint64(txIndex))
	if err != nil {
		return nil, rpccore.ErrInvalidTxIndex
	}

	return AdaptTransaction(txn), nil
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

		pendingB = h.PendingBlock()
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

	return AdaptReceipt(receipt, txn, status, blockHash, blockNumber), nil
}

// AddTransaction relays a transaction to the gateway, or to the sequencer if enabled
func (h *Handler) AddTransaction(ctx context.Context, tx *BroadcastedTransaction) (AddTxResponse, *jsonrpc.Error) {
	var (
		res AddTxResponse
		err *jsonrpc.Error
	)
	if h.memPool != nil {
		res, err = h.addToMempool(ctx, tx)
	} else {
		res, err = h.pushToFeederGateway(ctx, tx)
	}

	if err != nil {
		return AddTxResponse{}, err
	}

	if h.submittedTransactionsCache != nil {
		h.submittedTransactionsCache.Add(res.TransactionHash)
	}

	return res, nil
}

func (h *Handler) addToMempool(ctx context.Context, tx *BroadcastedTransaction) (AddTxResponse, *jsonrpc.Error) {
	userTxn, userClass, paidFeeOnL1, err := AdaptBroadcastedTransaction(tx, h.bcReader.Network())
	if err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	err = h.memPool.Push(ctx, &mempool.BroadcastedTransaction{
		Transaction:   userTxn,
		DeclaredClass: userClass,
		PaidFeeOnL1:   paidFeeOnL1,
	})
	if err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	res := AddTxResponse{TransactionHash: userTxn.Hash()}
	if tx.Type == TxnDeployAccount {
		contractAddress := core.ContractAddress(
			&felt.Zero,
			tx.ClassHash,
			tx.ContractAddressSalt,
			*tx.ConstructorCallData,
		)
		res.ContractAddress = &contractAddress
	} else if tx.Type == TxnDeclare {
		classHash, err := userClass.Hash()
		if err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
		}
		res.ClassHash = &classHash
	}
	return res, nil
}

func (h *Handler) pushToFeederGateway(ctx context.Context, tx *BroadcastedTransaction) (AddTxResponse, *jsonrpc.Error) {
	if tx.Type == TxnDeclare && tx.Version.Cmp(new(felt.Felt).SetUint64(2)) != -1 {
		contractClass := make(map[string]any)
		if err := json.Unmarshal(tx.ContractClass, &contractClass); err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(fmt.Sprintf("unmarshal contract class: %v", err))
		}
		sierraProg, ok := contractClass["sierra_program"]
		if !ok {
			return AddTxResponse{}, jsonrpc.Err(jsonrpc.InvalidParams, "{'sierra_program': ['Missing data for required field.']}")
		}

		sierraProgBytes, errIn := json.Marshal(sierraProg)
		if errIn != nil {
			return AddTxResponse{}, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		gwSierraProg, errIn := utils.Gzip64Encode(sierraProgBytes)
		if errIn != nil {
			return AddTxResponse{}, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		contractClass["sierra_program"] = gwSierraProg
		newContractClass, err := json.Marshal(contractClass)
		if err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(fmt.Sprintf("marshal revised contract class: %v", err))
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
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(fmt.Sprintf("marshal transaction: %v", err))
	}

	if h.gatewayClient == nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData("no gateway client configured")
	}

	respJSON, err := h.gatewayClient.AddTransaction(ctx, txJSON)
	if err != nil {
		return AddTxResponse{}, makeJSONErrorFromGatewayError(err)
	}

	var gatewayResponse struct {
		TransactionHash *felt.Felt `json:"transaction_hash"`
		ContractAddress *felt.Felt `json:"address"`
		ClassHash       *felt.Felt `json:"class_hash"`
	}
	if err = json.Unmarshal(respJSON, &gatewayResponse); err != nil {
		return AddTxResponse{}, jsonrpc.Err(jsonrpc.InternalError, fmt.Sprintf("unmarshal gateway response: %v", err))
	}

	return AddTxResponse{
		TransactionHash: gatewayResponse.TransactionHash,
		ContractAddress: gatewayResponse.ContractAddress,
		ClassHash:       gatewayResponse.ClassHash,
	}, nil
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

		if txStatus.FinalityStatus == starknet.NotReceived && h.submittedTransactionsCache != nil {
			if h.submittedTransactionsCache.Contains(&hash) {
				txStatus.FinalityStatus = starknet.Received
			}
		}

		status, err := adaptTransactionStatus(txStatus)
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

func (h *Handler) TransactionStatusV0_7(
	// Todo make `hash` by reference
	ctx context.Context, hash felt.Felt,
) (*TransactionStatusV0_7, *jsonrpc.Error) {
	res, err := h.TransactionStatus(ctx, hash)
	if err != nil {
		return nil, err
	}

	return &TransactionStatusV0_7{
		Finality:  res.Finality,
		Execution: res.Execution,
	}, nil
}

func makeJSONErrorFromGatewayError(err error) *jsonrpc.Error {
	gatewayErr, ok := err.(*gateway.Error)
	if !ok {
		return jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch gatewayErr.Code {
	case gateway.InvalidContractClass:
		return rpccore.ErrInvalidContractClass
	case gateway.UndeclaredClass:
		return rpccore.ErrClassHashNotFound
	case gateway.ClassAlreadyDeclared:
		return rpccore.ErrClassAlreadyDeclared
	case gateway.InsufficientResourcesForValidate:
		return rpccore.ErrInsufficientResourcesForValidate
	case gateway.InsufficientAccountBalance:
		return rpccore.ErrInsufficientAccountBalanceV0_8
	case gateway.ValidateFailure:
		return rpccore.ErrValidationFailure.CloneWithData(gatewayErr.Message)
	case gateway.ContractBytecodeSizeTooLarge, gateway.ContractClassObjectSizeTooLarge:
		return rpccore.ErrContractClassSizeTooLarge
	case gateway.DuplicatedTransaction:
		return rpccore.ErrDuplicateTx
	case gateway.InvalidTransactionNonce:
		return rpccore.ErrInvalidTransactionNonce
	case gateway.CompilationFailed:
		return rpccore.ErrCompilationFailed.CloneWithData(gatewayErr.Message)
	case gateway.InvalidCompiledClassHash:
		return rpccore.ErrCompiledClassHashMismatch
	case gateway.InvalidTransactionVersion:
		return rpccore.ErrUnsupportedTxVersion
	case gateway.InvalidContractClassVersion:
		return rpccore.ErrUnsupportedContractClassVersion
	default:
		return rpccore.ErrUnexpectedError.CloneWithData(gatewayErr.Message)
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

// todo(Kirill): try to replace core.Transaction with rpc.Transaction type
func AdaptReceipt(receipt *core.TransactionReceipt, txn core.Transaction, finalityStatus TxnFinalityStatus,
	blockHash *felt.Felt, blockNumber uint64,
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
		ExecutionResources: adaptExecutionResources(receipt.ExecutionResources),
		MessageHash:        messageHash,
	}
}

func adaptTransactionStatus(txStatus *starknet.TransactionStatus) (*TransactionStatus, error) {
	var status TransactionStatus

	switch finalityStatus := txStatus.FinalityStatus; finalityStatus {
	case starknet.AcceptedOnL1:
		status.Finality = TxnStatusAcceptedOnL1
	case starknet.AcceptedOnL2:
		status.Finality = TxnStatusAcceptedOnL2
	case starknet.Received:
		status.Finality = TxnStatusReceived
	case starknet.NotReceived:
		return nil, errTransactionNotFound
	default:
		return nil, fmt.Errorf("unknown finality status: %v", finalityStatus)
	}

	switch txStatus.ExecutionStatus {
	case starknet.Succeeded:
		status.Execution = TxnSuccess
	case starknet.Reverted:
		status.Execution = TxnFailure
		status.FailureReason = txStatus.RevertError
	case starknet.Rejected:
		status.Finality = TxnStatusRejected
		if txStatus.FailureReason != nil {
			status.FailureReason = txStatus.FailureReason.Message
		}
	default: // Omit the field on error. It's optional in the spec.
	}

	return &status, nil
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1605
func adaptInvokeTransaction(t *core.InvokeTransaction) *Transaction {
	tx := &Transaction{
		Type:               TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version.AsFelt(),
		Signature:          utils.HeapPtr(t.Signature()),
		Nonce:              t.Nonce,
		CallData:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
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
		Signature:         utils.HeapPtr(t.Signature()),
		Nonce:             t.Nonce,
		ClassHash:         t.ClassHash,
		SenderAddress:     t.SenderAddress,
		CompiledClassHash: t.CompiledClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptDeployAccountTrandaction(t *core.DeployAccountTransaction) *Transaction {
	tx := &Transaction{
		Hash:                t.Hash(),
		MaxFee:              t.MaxFee,
		Version:             t.Version.AsFelt(),
		Signature:           utils.HeapPtr(t.Signature()),
		Nonce:               t.Nonce,
		Type:                TxnDeployAccount,
		ContractAddressSalt: t.ContractAddressSalt,
		ConstructorCallData: &t.ConstructorCallData,
		ClassHash:           t.ClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}
