package vm

import (
	"encoding/json"
	"errors"
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
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}

		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		return json.Marshal(core2sn.AdaptCompiledClass(c.Compiled))
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
}
