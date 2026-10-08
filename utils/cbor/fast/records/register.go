package records

import (
	"reflect"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
)

// Register sends the types this package reads to the fast decoder. It needs the tags
// cbor.RegisterType gave, so call it after the types are registered.
func Register() {
	sierraTag, ok := cbor.TagOf(reflect.TypeFor[core.SierraClass]())
	if !ok {
		panic("records: SierraClass has no CBOR tag")
	}

	cbor.RegisterReader(ReadReceipt)
	cbor.RegisterReader(ReadClassDefinition(sierraTag))
}
