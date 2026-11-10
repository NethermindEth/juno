package interfaces_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	witness "github.com/NethermindEth/juno/verification/fastcbor-proof/feasibility/interfaces"
)

type otherTransaction struct{}

func (*otherTransaction) Hash() *felt.Felt { panic("classification must not call Hash") }
func (*otherTransaction) Signature() []felt.Felt {
	panic("classification must not call Signature")
}
func (*otherTransaction) TxVersion() *core.TransactionVersion {
	panic("classification must not call TxVersion")
}

func TestTransactionInterfaceWitnesses(t *testing.T) {
	deploy := new(core.DeployTransaction)
	account := new(core.DeployAccountTransaction)
	invoke := new(core.InvokeTransaction)
	declare := new(core.DeclareTransaction)
	handler := new(core.L1HandlerTransaction)
	for _, test := range []struct {
		name     string
		boxed    core.Transaction
		original core.Transaction
		variant  uint8
		typedNil core.Transaction
	}{
		{"deploy", witness.BoxDeploy(deploy), deploy, 1, witness.BoxDeploy(nil)},
		{"deploy-account", witness.BoxDeployAccount(account), account, 2, witness.BoxDeployAccount(nil)},
		{"invoke", witness.BoxInvoke(invoke), invoke, 3, witness.BoxInvoke(nil)},
		{"declare", witness.BoxDeclare(declare), declare, 4, witness.BoxDeclare(nil)},
		{"l1-handler", witness.BoxL1Handler(handler), handler, 5, witness.BoxL1Handler(nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.boxed != test.original {
				t.Fatal("boxing changed the concrete pointer")
			}
			if variant, isNil := witness.ClassifyTransaction(test.boxed); variant != test.variant || isNil {
				t.Fatalf("non-nil pointer: variant=%d nil=%v", variant, isNil)
			}
			if test.typedNil == nil {
				t.Fatal("typed nil was collapsed to a nil interface")
			}
			if variant, isNil := witness.ClassifyTransaction(test.typedNil); variant != test.variant || !isNil {
				t.Fatalf("typed nil: variant=%d nil=%v", variant, isNil)
			}
		})
	}
	if variant, isNil := witness.ClassifyTransaction(nil); variant != 0 || isNil {
		t.Fatalf("nil interface: variant=%d nil=%v", variant, isNil)
	}
	for _, tx := range []core.Transaction{new(otherTransaction), (*otherTransaction)(nil)} {
		if variant, isNil := witness.ClassifyTransaction(tx); variant != 6 || isNil {
			t.Fatalf("unrecognized dynamic type: variant=%d nil=%v", variant, isNil)
		}
	}
}
