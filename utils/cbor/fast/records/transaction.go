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

// ReadInvoke reads the map of a stored invoke transaction, after its tag.
func ReadInvoke(r *fast.Reader, dst *core.InvokeTransaction) error {
	var seen uint64
	return readFields(r, foldInvokeTransaction, func(key string) (bool, error) {
		var err error
		switch key {
		case "TransactionHash":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.TransactionHash)
		case "CallData":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.CallData)
		case "TransactionSignature":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.TransactionSignature)
		case "MaxFee":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.MaxFee)
		case "ContractAddress":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.ContractAddress)
		case "Version":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readLimbPtr(r, &dst.Version)
		case "EntryPointSelector":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.EntryPointSelector)
		case "Nonce":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Nonce)
		case "SenderAddress":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.SenderAddress)
		case "ResourceBounds":
			if once(&seen, 10) {
				return false, fast.ErrUnsupported
			}
			err = readResourceBounds(r, &dst.ResourceBounds)
		case "Tip":
			if once(&seen, 11) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Tip)
		case "PaymasterData":
			if once(&seen, 12) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.PaymasterData)
		case "AccountDeploymentData":
			if once(&seen, 13) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.AccountDeploymentData)
		case "NonceDAMode":
			if once(&seen, 14) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.NonceDAMode)
		case "FeeDAMode":
			if once(&seen, 15) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.FeeDAMode)
		case "ProofFacts":
			if once(&seen, 16) {
				return false, fast.ErrUnsupported
			}
			dst.ProofFacts, err = fast.LimbSlice[felt.Felt](r)
		default:
			return false, nil
		}
		return true, err
	})
}

// ReadDeclare reads the map of a stored declare transaction, after its tag.
func ReadDeclare(r *fast.Reader, dst *core.DeclareTransaction) error {
	var seen uint64
	return readFields(r, foldDeclareTransaction, func(key string) (bool, error) {
		var err error
		switch key {
		case "TransactionHash":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.TransactionHash)
		case "ClassHash":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.ClassHash)
		case "SenderAddress":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.SenderAddress)
		case "MaxFee":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.MaxFee)
		case "TransactionSignature":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.TransactionSignature)
		case "Nonce":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Nonce)
		case "Version":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readLimbPtr(r, &dst.Version)
		case "CompiledClassHash":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.CompiledClassHash)
		case "ResourceBounds":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readResourceBounds(r, &dst.ResourceBounds)
		case "Tip":
			if once(&seen, 10) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Tip)
		case "PaymasterData":
			if once(&seen, 11) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.PaymasterData)
		case "AccountDeploymentData":
			if once(&seen, 12) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.AccountDeploymentData)
		case "NonceDAMode":
			if once(&seen, 13) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.NonceDAMode)
		case "FeeDAMode":
			if once(&seen, 14) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.FeeDAMode)
		default:
			return false, nil
		}
		return true, err
	})
}

func readDeploy(r *fast.Reader, dst *core.DeployTransaction) error {
	var seen uint64
	return readFields(r, foldDeployTransaction, func(key string) (bool, error) {
		return readDeployField(r, key, &seen, dst)
	})
}

// readDeployField reads one field of a deploy transaction. A deploy-account transaction embeds
// one, so its map carries these keys too.
func readDeployField(r *fast.Reader, key string, seen *uint64, dst *core.DeployTransaction) (bool, error) {
	var err error
	switch key {
	case "TransactionHash":
		if once(seen, 1) {
			return false, fast.ErrUnsupported
		}
		err = readFelt(r, &dst.TransactionHash)
	case "ContractAddressSalt":
		if once(seen, 2) {
			return false, fast.ErrUnsupported
		}
		err = readFelt(r, &dst.ContractAddressSalt)
	case "ContractAddress":
		if once(seen, 3) {
			return false, fast.ErrUnsupported
		}
		err = readFelt(r, &dst.ContractAddress)
	case "ClassHash":
		if once(seen, 4) {
			return false, fast.ErrUnsupported
		}
		err = readFelt(r, &dst.ClassHash)
	case "ConstructorCallData":
		if once(seen, 5) {
			return false, fast.ErrUnsupported
		}
		err = readFeltSlice(r, &dst.ConstructorCallData)
	case "Version":
		if once(seen, 6) {
			return false, fast.ErrUnsupported
		}
		err = readLimbPtr(r, &dst.Version)
	default:
		return false, nil
	}
	return true, err
}

func readDeployAccount(r *fast.Reader, dst *core.DeployAccountTransaction) error {
	var seen uint64
	return readFields(r, foldDeployAccountTransaction, func(key string) (bool, error) {
		if known, err := readDeployField(r, key, &seen, &dst.DeployTransaction); known || err != nil {
			return known, err
		}
		var err error
		switch key {
		case "MaxFee":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.MaxFee)
		case "TransactionSignature":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.TransactionSignature)
		case "Nonce":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Nonce)
		case "ResourceBounds":
			if once(&seen, 10) {
				return false, fast.ErrUnsupported
			}
			err = readResourceBounds(r, &dst.ResourceBounds)
		case "Tip":
			if once(&seen, 11) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Tip)
		case "PaymasterData":
			if once(&seen, 12) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.PaymasterData)
		case "NonceDAMode":
			if once(&seen, 13) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.NonceDAMode)
		case "FeeDAMode":
			if once(&seen, 14) {
				return false, fast.ErrUnsupported
			}
			err = readUint32(r, &dst.FeeDAMode)
		default:
			return false, nil
		}
		return true, err
	})
}

func readL1Handler(r *fast.Reader, dst *core.L1HandlerTransaction) error {
	var seen uint64
	return readFields(r, foldL1HandlerTransaction, func(key string) (bool, error) {
		var err error
		switch key {
		case "TransactionHash":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.TransactionHash)
		case "ContractAddress":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.ContractAddress)
		case "EntryPointSelector":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.EntryPointSelector)
		case "Nonce":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Nonce)
		case "CallData":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.CallData)
		case "Version":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readLimbPtr(r, &dst.Version)
		default:
			return false, nil
		}
		return true, err
	})
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

func readBounds(r *fast.Reader, dst *core.ResourceBounds) error {
	var seen uint64
	return readFields(r, foldResourceBounds, func(key string) (bool, error) {
		var err error
		switch key {
		case "MaxAmount":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.MaxAmount)
		case "MaxPricePerUnit":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.MaxPricePerUnit)
		default:
			return false, nil
		}
		return true, err
	})
}
