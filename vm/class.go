package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	junopb "github.com/NethermindEth/juno/vm/protobuf"
	"google.golang.org/protobuf/proto"
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

func toPBFelt(f *felt.Felt) *junopb.Felt {
	if f == nil {
		return &junopb.Felt{Be32: make([]byte, felt.Bytes)}
	}
	b := f.Bytes()
	out := make([]byte, len(b))
	copy(out, b[:])
	return &junopb.Felt{Be32: out}
}

func marshalClassInfoProtoBytes(class core.ClassDefinition) ([]byte, error) {
	env := &junopb.CompiledClass{}

	switch c := class.(type) {
	case *core.DeprecatedCairoClass:
		env.CairoVersion = 0
		decompressedProgram, err := utils.Gzip64Decode(c.Program)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress program: %w", err)
		}
		dep := &junopb.DeprecatedCairoClass{
			AbiJson:      []byte(c.Abi),
			ProgramB64:   decompressedProgram,
			Externals:    make([]*junopb.DeprecatedEntryPoint, len(c.Externals)),
			L1Handlers:   make([]*junopb.DeprecatedEntryPoint, len(c.L1Handlers)),
			Constructors: make([]*junopb.DeprecatedEntryPoint, len(c.Constructors)),
		}
		fill := func(dst []*junopb.DeprecatedEntryPoint, src []core.DeprecatedEntryPoint) {
			for i := range src {
				dst[i] = &junopb.DeprecatedEntryPoint{
					Selector: toPBFelt(src[i].Selector),
					Offset:   toPBFelt(src[i].Offset),
				}
			}
		}
		fill(dep.Externals, c.Externals)
		fill(dep.L1Handlers, c.L1Handlers)
		fill(dep.Constructors, c.Constructors)

		env.Class = &junopb.CompiledClass_Deprecated{Deprecated: dep}
		env.AbiLength = uint32(len(c.Abi))

	case *core.SierraClass:
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}
		env.CairoVersion = 1
		env.AbiLength = uint32(len(c.Abi))
		env.SierraProgramLength = uint32(len(c.Program))
		env.SierraVersion = c.SierraVersion()

		cc := c.Compiled
		bytecodeSegmentLengths, err := json.Marshal(cc.BytecodeSegmentLengths)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal bytecode segment lengths: %w", err)
		}
		pb := &junopb.CasmClass{
			Prime:                  utils.ToHex(cc.Prime),
			Bytecode:               make([]*junopb.Felt, len(cc.Bytecode)),
			HintsJson:              []byte(cc.Hints),
			PythonicHintsJson:      []byte(cc.PythonicHints),
			CompilerVersion:        cc.CompilerVersion,
			BytecodeSegmentLengths: bytecodeSegmentLengths,
			External:               make([]*junopb.CompiledEntryPoint, len(cc.External)),
			L1Handler:              make([]*junopb.CompiledEntryPoint, len(cc.L1Handler)),
			Constructor:            make([]*junopb.CompiledEntryPoint, len(cc.Constructor)),
		}
		for i := range cc.Bytecode {
			pb.Bytecode[i] = toPBFelt(cc.Bytecode[i])
		}
		fillEP := func(dst []*junopb.CompiledEntryPoint, src []core.CasmEntryPoint) {
			for i := range src {
				dst[i] = &junopb.CompiledEntryPoint{
					Selector: toPBFelt(src[i].Selector),
					Offset:   src[i].Offset,
					Builtins: append([]string(nil), src[i].Builtins...),
				}
			}
		}
		fillEP(pb.External, cc.External)
		fillEP(pb.L1Handler, cc.L1Handler)
		fillEP(pb.Constructor, cc.Constructor)

		env.Class = &junopb.CompiledClass_Casm{Casm: pb}

	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}

	return proto.Marshal(env)
}
