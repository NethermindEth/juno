// Package interfaces contains language-feature witnesses for transaction and
// class interfaces. Its variant numbers are local test labels, not CBOR wire tags.
package interfaces

import "github.com/NethermindEth/juno/core"

func BoxDeploy(tx *core.DeployTransaction) core.Transaction { return tx }

func BoxDeployAccount(tx *core.DeployAccountTransaction) core.Transaction { return tx }

func BoxInvoke(tx *core.InvokeTransaction) core.Transaction { return tx }

func BoxDeclare(tx *core.DeclareTransaction) core.Transaction { return tx }

func BoxL1Handler(tx *core.L1HandlerTransaction) core.Transaction { return tx }

// ClassifyTransaction exercises the concrete type assertions and nil checks a
// typed interface consumer needs. A nil interface is distinct from a typed nil.
// No transaction payload is read and no interface method is called.
func ClassifyTransaction(tx core.Transaction) (uint8, bool) {
	switch value := tx.(type) {
	case nil:
		return 0, false
	case *core.DeployTransaction:
		return 1, value == nil
	case *core.DeployAccountTransaction:
		return 2, value == nil
	case *core.InvokeTransaction:
		return 3, value == nil
	case *core.DeclareTransaction:
		return 4, value == nil
	case *core.L1HandlerTransaction:
		return 5, value == nil
	default:
		return 6, false
	}
}

func BoxCairo(class *core.DeprecatedCairoClass) core.ClassDefinition { return class }

func BoxSierra(class *core.SierraClass) core.ClassDefinition { return class }

// ClassifyClass checks dynamic types and typed nil without invoking class methods.
func ClassifyClass(class core.ClassDefinition) (uint8, bool) {
	switch value := class.(type) {
	case nil:
		return 0, false
	case *core.DeprecatedCairoClass:
		return 1, value == nil
	case *core.SierraClass:
		return 2, value == nil
	default:
		return 3, false
	}
}
