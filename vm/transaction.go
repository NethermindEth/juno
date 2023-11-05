package vm

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/jinzhu/copier"
)

// marshalTxn returns a json structure that includes the transaction serde will
// unmarshal to the Blockifier type and a boolean to indicate if version has
// the query bit set or not.
func marshalTxn(txn core.Transaction) (json.RawMessage, error) {
	var t starknet.Transaction
	if err := copier.Copy(&t, txn); err != nil {
		return nil, err
	}
	version := (*core.TransactionVersion)(t.Version)
	txnAndQueryBit := struct {
		QueryBit bool           `json:"query_bit"`
		Txn      map[string]any `json:"txn"`
	}{Txn: make(map[string]any), QueryBit: version.HasQueryBit()}

	versionWithoutQueryBit := version.WithoutQueryBit()
	t.Version = versionWithoutQueryBit.AsFelt()
	switch txn.(type) {
	case *core.InvokeTransaction:
		// workaround until starknet_api::transaction::InvokeTranscationV0 is fixed
		if t.Version.IsZero() {
			t.Nonce = &felt.Zero
			t.SenderAddress = t.ContractAddress
		}
		txnAndQueryBit.Txn["Invoke"] = map[string]any{
			"V" + t.Version.Text(felt.Base10): t,
		}
	case *core.DeployAccountTransaction:
		txnAndQueryBit.Txn["DeployAccount"] = t
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
