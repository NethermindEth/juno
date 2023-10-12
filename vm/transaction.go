package vm

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/jinzhu/copier"
)

func marshalTxn(txn core.Transaction) (json.RawMessage, error) {
	txnMap := make(map[string]any)

	var t starknet.Transaction
	if err := copier.Copy(&t, txn); err != nil {
		return nil, err
	}

	versionWithoutQueryBit := (*core.TransactionVersion)(t.Version).WithoutQueryBit()
	t.Version = versionWithoutQueryBit.AsFelt()
	switch txn.(type) {
	case *core.InvokeTransaction:
		// workaround until starknet_api::transaction::InvokeTranscationV0 is fixed
		if t.Version.IsZero() {
			t.Nonce = &felt.Zero
			t.SenderAddress = t.ContractAddress
		}
		txnMap["Invoke"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.DeployAccountTransaction:
		txnMap["DeployAccount"] = t
	case *core.DeclareTransaction:
		txnMap["Declare"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.L1HandlerTransaction:
		txnMap["L1Handler"] = t
	case *core.DeployTransaction:
		txnMap["Deploy"] = t
	default:
		return nil, fmt.Errorf("unsupported txn type %T", txn)
	}
	return json.Marshal(txnMap)
}
