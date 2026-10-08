//nolint:goconst // A wire name reads best spelled out beside the field number it maps to.
package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// TransactionTags are the CBOR tags of the concrete transaction types.
type TransactionTags struct {
	Deploy, DeployAccount, Declare, Invoke, L1Handler uint64
}

// ReadTransaction reads a stored transaction: a tag naming its type, then the transaction.
// A tag that is none of the five is an error in the data, not an unsupported form.
func ReadTransaction(tags TransactionTags) func(r *fast.Reader, dst *core.Transaction) error {
	return func(r *fast.Reader, dst *core.Transaction) error {
		if r.Null() {
			*dst = nil
			return nil
		}
		tag, err := r.Tag()
		if err != nil {
			return err
		}
		switch tag {
		case tags.Invoke:
			tx := new(core.InvokeTransaction)
			err, *dst = ReadInvoke(r, tx), tx
		case tags.Declare:
			tx := new(core.DeclareTransaction)
			err, *dst = ReadDeclare(r, tx), tx
		case tags.Deploy:
			tx := new(core.DeployTransaction)
			err, *dst = readDeploy(r, tx), tx
		case tags.DeployAccount:
			tx := new(core.DeployAccountTransaction)
			err, *dst = readDeployAccount(r, tx), tx
		case tags.L1Handler:
			tx := new(core.L1HandlerTransaction)
			err, *dst = readL1Handler(r, tx), tx
		default:
			return fast.ErrUnsupported
		}
		if err != nil {
			*dst = nil
		}
		return err
	}
}

const (
	invokeTransactionHash = iota
	invokeCallData
	invokeTransactionSignature
	invokeMaxFee
	invokeContractAddress
	invokeVersion
	invokeEntryPointSelector
	invokeNonce
	invokeSenderAddress
	invokeResourceBounds
	invokeTip
	invokePaymasterData
	invokeAccountDeploymentData
	invokeNonceDAMode
	invokeFeeDAMode
	invokeProofFacts
)

var invokeFields = fieldsOf[core.InvokeTransaction]([]string{
	invokeTransactionHash:       "TransactionHash",
	invokeCallData:              "CallData",
	invokeTransactionSignature:  "TransactionSignature",
	invokeMaxFee:                "MaxFee",
	invokeContractAddress:       "ContractAddress",
	invokeVersion:               "Version",
	invokeEntryPointSelector:    "EntryPointSelector",
	invokeNonce:                 "Nonce",
	invokeSenderAddress:         "SenderAddress",
	invokeResourceBounds:        "ResourceBounds",
	invokeTip:                   "Tip",
	invokePaymasterData:         "PaymasterData",
	invokeAccountDeploymentData: "AccountDeploymentData",
	invokeNonceDAMode:           "NonceDAMode",
	invokeFeeDAMode:             "FeeDAMode",
	invokeProofFacts:            "ProofFacts",
})

// ReadInvoke reads the map of a stored invoke transaction, after its tag.
func ReadInvoke(r *fast.Reader, dst *core.InvokeTransaction) error {
	s, err := r.OpenStruct(invokeFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case invokeTransactionHash:
			err = readFelt(r, &dst.TransactionHash)
		case invokeCallData:
			err = readFeltSlice(r, &dst.CallData)
		case invokeTransactionSignature:
			err = readFeltSlice(r, &dst.TransactionSignature)
		case invokeMaxFee:
			err = readFelt(r, &dst.MaxFee)
		case invokeContractAddress:
			err = readFelt(r, &dst.ContractAddress)
		case invokeVersion:
			err = readLimbPtr(r, &dst.Version)
		case invokeEntryPointSelector:
			err = readFelt(r, &dst.EntryPointSelector)
		case invokeNonce:
			err = readFelt(r, &dst.Nonce)
		case invokeSenderAddress:
			err = readFelt(r, &dst.SenderAddress)
		case invokeResourceBounds:
			err = readResourceBounds(r, &dst.ResourceBounds)
		case invokeTip:
			err = readUint64(r, &dst.Tip)
		case invokePaymasterData:
			err = readFeltSlice(r, &dst.PaymasterData)
		case invokeAccountDeploymentData:
			err = readFeltSlice(r, &dst.AccountDeploymentData)
		case invokeNonceDAMode:
			err = readUint32(r, &dst.NonceDAMode)
		case invokeFeeDAMode:
			err = readUint32(r, &dst.FeeDAMode)
		case invokeProofFacts:
			dst.ProofFacts, err = fast.LimbSlice[felt.Felt](r)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	declareTransactionHash = iota
	declareClassHash
	declareSenderAddress
	declareMaxFee
	declareTransactionSignature
	declareNonce
	declareVersion
	declareCompiledClassHash
	declareResourceBounds
	declareTip
	declarePaymasterData
	declareAccountDeploymentData
	declareNonceDAMode
	declareFeeDAMode
)

var declareFields = fieldsOf[core.DeclareTransaction]([]string{
	declareTransactionHash:       "TransactionHash",
	declareClassHash:             "ClassHash",
	declareSenderAddress:         "SenderAddress",
	declareMaxFee:                "MaxFee",
	declareTransactionSignature:  "TransactionSignature",
	declareNonce:                 "Nonce",
	declareVersion:               "Version",
	declareCompiledClassHash:     "CompiledClassHash",
	declareResourceBounds:        "ResourceBounds",
	declareTip:                   "Tip",
	declarePaymasterData:         "PaymasterData",
	declareAccountDeploymentData: "AccountDeploymentData",
	declareNonceDAMode:           "NonceDAMode",
	declareFeeDAMode:             "FeeDAMode",
})

// ReadDeclare reads the map of a stored declare transaction, after its tag.
func ReadDeclare(r *fast.Reader, dst *core.DeclareTransaction) error {
	s, err := r.OpenStruct(declareFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case declareTransactionHash:
			err = readFelt(r, &dst.TransactionHash)
		case declareClassHash:
			err = readFelt(r, &dst.ClassHash)
		case declareSenderAddress:
			err = readFelt(r, &dst.SenderAddress)
		case declareMaxFee:
			err = readFelt(r, &dst.MaxFee)
		case declareTransactionSignature:
			err = readFeltSlice(r, &dst.TransactionSignature)
		case declareNonce:
			err = readFelt(r, &dst.Nonce)
		case declareVersion:
			err = readLimbPtr(r, &dst.Version)
		case declareCompiledClassHash:
			err = readFelt(r, &dst.CompiledClassHash)
		case declareResourceBounds:
			err = readResourceBounds(r, &dst.ResourceBounds)
		case declareTip:
			err = readUint64(r, &dst.Tip)
		case declarePaymasterData:
			err = readFeltSlice(r, &dst.PaymasterData)
		case declareAccountDeploymentData:
			err = readFeltSlice(r, &dst.AccountDeploymentData)
		case declareNonceDAMode:
			err = readUint32(r, &dst.NonceDAMode)
		case declareFeeDAMode:
			err = readUint32(r, &dst.FeeDAMode)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	deployTransactionHash = iota
	deployContractAddressSalt
	deployContractAddress
	deployClassHash
	deployConstructorCallData
	deployVersion
	deployFieldCount
)

var deployFields = fieldsOf[core.DeployTransaction]([]string{
	deployTransactionHash:     "TransactionHash",
	deployContractAddressSalt: "ContractAddressSalt",
	deployContractAddress:     "ContractAddress",
	deployClassHash:           "ClassHash",
	deployConstructorCallData: "ConstructorCallData",
	deployVersion:             "Version",
})

func readDeploy(r *fast.Reader, dst *core.DeployTransaction) error {
	s, err := r.OpenStruct(deployFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		if field == fast.Done {
			return err
		}
		err = readDeployField(r, field, dst)
	}
	return err
}

// readDeployField reads field number field of a deploy transaction, or skips the value of a
// field it does not have.
func readDeployField(r *fast.Reader, field int, dst *core.DeployTransaction) error {
	switch field {
	case deployTransactionHash:
		return readFelt(r, &dst.TransactionHash)
	case deployContractAddressSalt:
		return readFelt(r, &dst.ContractAddressSalt)
	case deployContractAddress:
		return readFelt(r, &dst.ContractAddress)
	case deployClassHash:
		return readFelt(r, &dst.ClassHash)
	case deployConstructorCallData:
		return readFeltSlice(r, &dst.ConstructorCallData)
	case deployVersion:
		return readLimbPtr(r, &dst.Version)
	default:
		return r.Skip()
	}
}

const (
	deployAccountMaxFee = deployFieldCount + iota
	deployAccountTransactionSignature
	deployAccountNonce
	deployAccountResourceBounds
	deployAccountTip
	deployAccountPaymasterData
	deployAccountNonceDAMode
	deployAccountFeeDAMode
)

// A deploy-account transaction embeds a deploy transaction, so its map has the keys of both.
var deployAccountFields = fieldsOf[core.DeployAccountTransaction]([]string{
	deployTransactionHash:             "TransactionHash",
	deployContractAddressSalt:         "ContractAddressSalt",
	deployContractAddress:             "ContractAddress",
	deployClassHash:                   "ClassHash",
	deployConstructorCallData:         "ConstructorCallData",
	deployVersion:                     "Version",
	deployAccountMaxFee:               "MaxFee",
	deployAccountTransactionSignature: "TransactionSignature",
	deployAccountNonce:                "Nonce",
	deployAccountResourceBounds:       "ResourceBounds",
	deployAccountTip:                  "Tip",
	deployAccountPaymasterData:        "PaymasterData",
	deployAccountNonceDAMode:          "NonceDAMode",
	deployAccountFeeDAMode:            "FeeDAMode",
})

func readDeployAccount(r *fast.Reader, dst *core.DeployAccountTransaction) error {
	s, err := r.OpenStruct(deployAccountFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case deployAccountMaxFee:
			err = readFelt(r, &dst.MaxFee)
		case deployAccountTransactionSignature:
			err = readFeltSlice(r, &dst.TransactionSignature)
		case deployAccountNonce:
			err = readFelt(r, &dst.Nonce)
		case deployAccountResourceBounds:
			err = readResourceBounds(r, &dst.ResourceBounds)
		case deployAccountTip:
			err = readUint64(r, &dst.Tip)
		case deployAccountPaymasterData:
			err = readFeltSlice(r, &dst.PaymasterData)
		case deployAccountNonceDAMode:
			err = readUint32(r, &dst.NonceDAMode)
		case deployAccountFeeDAMode:
			err = readUint32(r, &dst.FeeDAMode)
		default:
			err = readDeployField(r, field, &dst.DeployTransaction)
		}
	}
	return err
}

const (
	l1HandlerTransactionHash = iota
	l1HandlerContractAddress
	l1HandlerEntryPointSelector
	l1HandlerNonce
	l1HandlerCallData
	l1HandlerVersion
)

var l1HandlerFields = fieldsOf[core.L1HandlerTransaction]([]string{
	l1HandlerTransactionHash:    "TransactionHash",
	l1HandlerContractAddress:    "ContractAddress",
	l1HandlerEntryPointSelector: "EntryPointSelector",
	l1HandlerNonce:              "Nonce",
	l1HandlerCallData:           "CallData",
	l1HandlerVersion:            "Version",
})

func readL1Handler(r *fast.Reader, dst *core.L1HandlerTransaction) error {
	s, err := r.OpenStruct(l1HandlerFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case l1HandlerTransactionHash:
			err = readFelt(r, &dst.TransactionHash)
		case l1HandlerContractAddress:
			err = readFelt(r, &dst.ContractAddress)
		case l1HandlerEntryPointSelector:
			err = readFelt(r, &dst.EntryPointSelector)
		case l1HandlerNonce:
			err = readFelt(r, &dst.Nonce)
		case l1HandlerCallData:
			err = readFeltSlice(r, &dst.CallData)
		case l1HandlerVersion:
			err = readLimbPtr(r, &dst.Version)
		default:
			err = r.Skip()
		}
	}
	return err
}

// readResourceBounds reads the map of resource bounds of a version 3 transaction.
func readResourceBounds(r *fast.Reader, dst *map[core.Resource]core.ResourceBounds) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	out := make(map[core.Resource]core.ResourceBounds, count)
	for ; count > 0; count-- {
		var resource core.Resource
		if r.Null() {
			// fxamacker keeps the previous key for a null one; it is not worth copying.
			return fast.ErrUnsupported
		}
		if err := readUint32(r, &resource); err != nil {
			return err
		}
		var bounds core.ResourceBounds
		if err := readBounds(r, &bounds); err != nil {
			return err
		}
		out[resource] = bounds
	}
	r.Leave()
	*dst = out
	return nil
}

const (
	boundsMaxAmount = iota
	boundsMaxPricePerUnit
)

var boundsFields = fieldsOf[core.ResourceBounds]([]string{
	boundsMaxAmount:       "MaxAmount",
	boundsMaxPricePerUnit: "MaxPricePerUnit",
})

func readBounds(r *fast.Reader, dst *core.ResourceBounds) error {
	s, err := r.OpenStruct(boundsFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case boundsMaxAmount:
			err = readUint64(r, &dst.MaxAmount)
		case boundsMaxPricePerUnit:
			err = readFelt(r, &dst.MaxPricePerUnit)
		default:
			err = r.Skip()
		}
	}
	return err
}
