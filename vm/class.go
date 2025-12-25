package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
)

func marshalClassInfo(class core.ClassDefinition) (json.RawMessage, error) {
	var classInfo struct {
		CairoVersion  uint32 `json:"cairo_version"`
		Class         any    `json:"contract_class"`
		AbiLength     uint32 `json:"abi_length"`
		SierraLength  uint32 `json:"sierra_program_length"`
		SierraVersion string `json:"sierra_version"`
	}

	switch c := class.(type) {
	case *core.DeprecatedCairoClass:
		var err error
		classInfo.CairoVersion = 0
		classInfo.Class, err = core2sn.AdaptDeprecatedCairoClass(c)
		if err != nil {
			return nil, err
		}
		classInfo.AbiLength = uint32(len(c.Abi))
	case *core.SierraClass:
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}

		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		classInfo.CairoVersion = 1
		classInfo.Class = core2sn.AdaptCasmClass(c.Compiled)
		classInfo.AbiLength = uint32(len(c.Abi))
		classInfo.SierraLength = uint32(len(c.Program))
		classInfo.SierraVersion = c.SierraVersion()
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
	return json.Marshal(classInfo)
}
