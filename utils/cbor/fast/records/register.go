package records

import (
	"reflect"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
)

// Register sends the types this package reads to the fast decoder. It needs the tags
// cbor.RegisterType gave, so call it after the types are registered.
func Register() {
	tag := func(t reflect.Type) uint64 {
		tag, ok := cbor.TagOf(t)
		if !ok {
			panic("records: " + t.String() + " has no CBOR tag")
		}
		return tag
	}

	cbor.RegisterReader(readReceipt)
	cbor.RegisterReader(readHeader)
	cbor.RegisterReader(readStateUpdate)
	cbor.RegisterReader(readTransaction(transactionTags{
		Deploy:        tag(reflect.TypeFor[core.DeployTransaction]()),
		DeployAccount: tag(reflect.TypeFor[core.DeployAccountTransaction]()),
		Declare:       tag(reflect.TypeFor[core.DeclareTransaction]()),
		Invoke:        tag(reflect.TypeFor[core.InvokeTransaction]()),
		L1Handler:     tag(reflect.TypeFor[core.L1HandlerTransaction]()),
	}))
	cbor.RegisterReader(readClassDefinition(tag(reflect.TypeFor[core.SierraClass]())))
}
