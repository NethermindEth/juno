package vm

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
)

func marshalCompiledClass(class core.Class) (json.RawMessage, error) {
	switch c := class.(type) {
	case *core.Cairo0Class:
		compiledCairo0Class, err := core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
		return json.Marshal(compiledCairo0Class)
	case *core.Cairo1Class:
		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		compiledCairo1Class := core2sn.AdaptCompiledClass(&c.Compiled)
		return json.Marshal(compiledCairo1Class)
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
}

func marshalDeclaredClass(class core.Class) (json.RawMessage, error) {
	switch c := class.(type) {
	case *core.Cairo0Class:
		declaredClass, err := core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
		return json.Marshal(declaredClass)
	case *core.Cairo1Class:
		declaredClass := core2sn.AdaptSierraClass(c)
		return json.Marshal(declaredClass)
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
}
