package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

// marshalTxn returns a json structure that includes the transaction serde will
// unmarshal to the Blockifier type and a boolean to indicate if version has
// the query bit set or not.
func marshalTxn(txn core.Transaction) (json.RawMessage, error) {
	t := adaptTransaction(txn)
	version := (*core.TransactionVersion)(t.Version)
	txnAndQueryBit := struct {
		QueryBit bool           `json:"query_bit"`
		Txn      map[string]any `json:"txn"`
		TxnHash  *felt.Felt     `json:"txn_hash"`
	}{Txn: make(map[string]any), QueryBit: version.HasQueryBit(), TxnHash: txn.Hash()}

	versionWithoutQueryBit := version.WithoutQueryBit()
	t.Version = versionWithoutQueryBit.AsFelt()
	switch txn.(type) {
	case *core.InvokeTransaction:
		txnAndQueryBit.Txn["Invoke"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.DeployAccountTransaction:
		txnAndQueryBit.Txn["DeployAccount"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.DeclareTransaction:
		txnAndQueryBit.Txn["Declare"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.L1HandlerTransaction:
		txnAndQueryBit.Txn["L1Handler"] = t
	case *core.DeployTransaction:
		txnAndQueryBit.Txn["Deploy"] = t
	default:
		return nil, fmt.Errorf("unsupported txn type %T", txn)
	}
	result, err := json.Marshal(txnAndQueryBit)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type Transaction struct {
	Version               *felt.Felt                   `json:"version,omitempty"`
	ContractAddress       *felt.Felt                   `json:"contract_address,omitempty"`
	ContractAddressSalt   *felt.Felt                   `json:"contract_address_salt,omitempty"`
	ClassHash             *felt.Felt                   `json:"class_hash,omitempty"`
	ConstructorCallData   *[]*felt.Felt                `json:"constructor_calldata,omitempty"`
	SenderAddress         *felt.Felt                   `json:"sender_address,omitempty"`
	MaxFee                *felt.Felt                   `json:"max_fee,omitempty"`
	Signature             *[]*felt.Felt                `json:"signature,omitempty"`
	CallData              *[]*felt.Felt                `json:"calldata,omitempty"`
	EntryPointSelector    *felt.Felt                   `json:"entry_point_selector,omitempty"`
	Nonce                 *felt.Felt                   `json:"nonce,omitempty"`
	CompiledClassHash     *felt.Felt                   `json:"compiled_class_hash,omitempty"`
	ResourceBounds        *map[Resource]ResourceBounds `json:"resource_bounds,omitempty"`
	Tip                   *felt.Felt                   `json:"tip,omitempty"`
	NonceDAMode           *DataAvailabilityMode        `json:"nonce_data_availability_mode,omitempty"`
	FeeDAMode             *DataAvailabilityMode        `json:"fee_data_availability_mode,omitempty"`
	AccountDeploymentData *[]*felt.Felt                `json:"account_deployment_data,omitempty"`
	PaymasterData         *[]*felt.Felt                `json:"paymaster_data,omitempty"`
}

type DataAvailabilityMode uint32

const (
	DAModeL1 DataAvailabilityMode = iota
	DAModeL2
)

func (m DataAvailabilityMode) MarshalJSON() ([]byte, error) {
	switch m {
	case DAModeL1:
		return []byte(`"L1"`), nil
	case DAModeL2:
		return []byte(`"L2"`), nil
	default:
		return nil, errors.New("unknown resource")
	}
}

type Resource uint32

const (
	ResourceL1Gas Resource = iota + 1
	ResourceL2Gas
)

func (r Resource) MarshalText() ([]byte, error) {
	switch r {
	case ResourceL1Gas:
		return []byte("L1_GAS"), nil
	case ResourceL2Gas:
		return []byte("L2_GAS"), nil
	default:
		return nil, errors.New("unknown resource")
	}
}

func (r Resource) MarshalJSON() ([]byte, error) {
	result, err := r.MarshalText()
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(result) + `"`), nil
}

type ResourceBounds struct {
	MaxAmount       *felt.Felt `json:"max_amount"`
	MaxPricePerUnit *felt.Felt `json:"max_price_per_unit"`
}

func adaptTransaction(txn core.Transaction) *Transaction {
	var tx *Transaction
	switch t := txn.(type) {
	case *core.DeclareTransaction:
		tx = &Transaction{
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
	case *core.InvokeTransaction:
		tx = &Transaction{
			MaxFee:             t.MaxFee,
			Version:            t.Version.AsFelt(),
			Signature:          utils.Ptr(t.Signature()),
			Nonce:              t.Nonce,
			CallData:           &t.CallData,
			ContractAddress:    t.ContractAddress,
			SenderAddress:      t.SenderAddress,
			EntryPointSelector: t.EntryPointSelector,
		}

		if t.Version.Is(3) {
			tx.ResourceBounds = utils.Ptr(adaptResourceBounds(t.ResourceBounds))
			tx.Tip = new(felt.Felt).SetUint64(t.Tip)
			tx.PaymasterData = &t.PaymasterData
			tx.AccountDeploymentData = &t.AccountDeploymentData
			tx.NonceDAMode = utils.Ptr(DataAvailabilityMode(t.NonceDAMode))
			tx.FeeDAMode = utils.Ptr(DataAvailabilityMode(t.FeeDAMode))
		}
	case *core.DeployTransaction:
		return &Transaction{
			ClassHash:           t.ClassHash,
			Version:             t.Version.AsFelt(),
			ContractAddressSalt: t.ContractAddressSalt,
			ConstructorCallData: &t.ConstructorCallData,
			ContractAddress:     t.ContractAddress,
		}
	case *core.L1HandlerTransaction:
		return &Transaction{
			Version:            t.Version.AsFelt(),
			Nonce:              t.Nonce,
			ContractAddress:    t.ContractAddress,
			EntryPointSelector: t.EntryPointSelector,
			CallData:           &t.CallData,
		}
	case *core.DeployAccountTransaction:
		tx = &Transaction{
			MaxFee:              t.MaxFee,
			Version:             t.Version.AsFelt(),
			Signature:           utils.Ptr(t.Signature()),
			Nonce:               t.Nonce,
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
	default:
		panic(fmt.Sprintf("unknown txn type in core2sn.AdaptTransaction: %T", t))
	}
	return tx
}

func adaptResourceBounds(rb map[core.Resource]core.ResourceBounds) map[Resource]ResourceBounds {
	rpcResourceBounds := make(map[Resource]ResourceBounds)
	for resource, bounds := range rb {
		rpcResourceBounds[Resource(resource)] = ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(bounds.MaxAmount),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}
	return rpcResourceBounds
}
