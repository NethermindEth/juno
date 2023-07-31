package vm

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/jinzhu/copier"
)

const queryBit = 128

var queryVersion = new(felt.Felt).Exp(new(felt.Felt).SetUint64(2), new(big.Int).SetUint64(queryBit))

func marshalTxn(txn core.Transaction) (json.RawMessage, error) {
	txnMap := make(map[string]any)

	var t feeder.Transaction
	if err := copier.Copy(&t, txn); err != nil {
		return nil, err
	}

	switch txn.(type) {
	case *core.InvokeTransaction:
		versionFelt := clearQueryBit(t.Version)
		// workaround until starknet_api::transaction::InvokeTranscationV0 is fixed
		if versionFelt.IsZero() {
			t.Nonce = &felt.Zero
			t.SenderAddress = t.ContractAddress
		}
		txnMap["Invoke"] = map[string]any{
			"V" + clearQueryBit(t.Version).Text(felt.Base10): t,
		}
	case *core.DeployAccountTransaction:
		txnMap["DeployAccount"] = t
	case *core.DeclareTransaction:
		txnMap["Declare"] = map[string]any{
			"V" + clearQueryBit(t.Version).Text(felt.Base10): t,
		}
	case *core.L1HandlerTransaction:
		txnMap["L1Handler"] = t
	default:
		return nil, errors.New("unsupported txn type")
	}
	return json.Marshal(txnMap)
}

func clearQueryBit(v *felt.Felt) *felt.Felt {
	versionWithoutQueryBit := new(felt.Felt).Set(v)
	// if versionWithoutQueryBit >= queryBit
	if versionWithoutQueryBit.Cmp(queryVersion) != -1 {
		versionWithoutQueryBit.Sub(versionWithoutQueryBit, queryVersion)
	}
	return versionWithoutQueryBit
}
