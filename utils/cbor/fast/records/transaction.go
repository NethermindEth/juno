package records

import (
	"github.com/NethermindEth/juno/core"
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
