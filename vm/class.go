package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

func marshalClassInfo(class core.Class) (json.RawMessage, error) {
	var classInfo struct {
		CairoVersion  uint32 `json:"cairo_version"`
		Class         any    `json:"contract_class"`
		AbiLength     uint32 `json:"abi_length"`
		SierraLength  uint32 `json:"sierra_program_length"`
		SierraVersion string `json:"sierra_version"`
	}

	switch c := class.(type) {
	case *core.Cairo0Class:
		var err error
		classInfo.CairoVersion = 0
		classInfo.Class, err = core2sn.AdaptCairo0Class(c)
		if err != nil {
			return nil, err
		}
		classInfo.AbiLength = uint32(len(c.Abi))
	case *core.Cairo1Class:
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}

		// we adapt the core type to the feeder type to avoid using JSON tags in core.Class.CompiledClass
		classInfo.CairoVersion = 1
		classInfo.Class = core2sn.AdaptCompiledClass(c.Compiled)
		classInfo.AbiLength = uint32(len(c.Abi))
		classInfo.SierraLength = uint32(len(c.Program))
		sierraVersion, err := parseSierraVersion(c.Program)
		if err != nil {
			return nil, err
		}
		classInfo.SierraVersion = sierraVersion
	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}
	return json.Marshal(classInfo)
}

var pre01, _ = new(felt.Felt).SetString("0x302e312e30")

// Parse Sierra version from the JSON representation of the program.
//
// Sierra programs contain the version number in two possible formats.
// For pre-1.0-rc0 Cairo versions the program contains the Sierra version
// "0.1.0" as a shortstring in its first Felt (0x302e312e30 = "0.1.0").
// For all subsequent versions the version number is the first three felts
// representing the three parts of a semantic version number.
// TODO: There should be an implementation in the blockifier. If there is, move it to the rust part.
func parseSierraVersion(prog []*felt.Felt) (string, error) {
	if len(prog) == 0 {
		return "", errors.New("failed to parse sierra version in classInfo")
	}

	if prog[0].Equal(pre01) {
		return "0.1.0", nil
	}

	if len(prog) < 3 {
		return "", errors.New("failed to parse sierra version in classInfo")
	}

	const base = 10
	var buf [32]byte
	b := buf[:0]
	b = strconv.AppendUint(b, prog[0].Uint64(), base)
	b = append(b, '.')
	b = strconv.AppendUint(b, prog[1].Uint64(), base)
	b = append(b, '.')
	b = strconv.AppendUint(b, prog[2].Uint64(), base)
	return string(b), nil
}
