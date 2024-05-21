package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
)

func marshalClassInfo(class core.Class) (json.RawMessage, error) {
	println("Attempting to marshalClassInfo")
	var classInfo struct {
		Class        any    `json:"contract_class"`
		AbiLength    uint32 `json:"abi_length"`
		SierraLength uint32 `json:"sierra_program_length"`
	}

	switch c := class.(type) {
	case *core.Cairo0Class:
		var err error
		classInfo.Class, err = core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
		classInfo.AbiLength = uint32(len(c.Abi))

		// Used only for debugging purposes
		hash, err := c.Hash()
		var hashStr string
		if err == nil {
			hashStr = "0x" + hash.Text(16)
		} else {
			hashStr = "error: " + err.Error()
		}
		println(fmt.Sprintf("Marshalling Cairo Zero class. Hash: %s Version: %v\n", hashStr, c.Version()))
	case *core.Cairo1Class:
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}

		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		// classInfo.Class = core2sn.AdaptCompiledClass(c.Compiled)
		classInfo.Class = core2sn.AdaptSierraClass(c)
		classInfo.AbiLength = uint32(len(c.Abi))
		classInfo.SierraLength = uint32(len(c.Program))

		// Used only for debugging purposes
		hash, err := c.Hash()
		var hashStr string
		if err == nil {
			hashStr = "0x" + hash.Text(16)
		} else {
			hashStr = "error: " + err.Error()
		}

		println(fmt.Sprintf("Marshalling Sierra class. Hash: %s Version: %v\n", hashStr, c.Version()))
	default:
		println("Failed marshalling, unknown type.")
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
	return json.Marshal(classInfo)
}
