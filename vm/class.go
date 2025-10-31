package vm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2sn"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
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
	compiledClass := &junopb.CompiledClass{}

	switch c := class.(type) {
	case *core.DeprecatedCairoClass:
		compiledClass.CairoVersion = 0
		adaptedClass, err := core2sn.AdaptDeprecatedCairoClass(c)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt deprecated cairo class: %w", err)
		}
		dep := &junopb.DeprecatedCairoClass{
			AbiJson:      []byte(adaptedClass.Abi),
			ProgramB64:   adaptedClass.Program,
			Externals:    make([]*junopb.DeprecatedEntryPoint, len(adaptedClass.EntryPoints.External)),
			L1Handlers:   make([]*junopb.DeprecatedEntryPoint, len(adaptedClass.EntryPoints.L1Handler)),
			Constructors: make([]*junopb.DeprecatedEntryPoint, len(adaptedClass.EntryPoints.Constructor)),
		}
		fill := func(dst []*junopb.DeprecatedEntryPoint, src []starknet.EntryPoint) {
			for i := range src {
				dst[i] = &junopb.DeprecatedEntryPoint{
					Selector: toPBFelt(src[i].Selector),
					Offset:   toPBFelt(src[i].Offset),
				}
			}
		}
		fill(dep.Externals, adaptedClass.EntryPoints.External)
		fill(dep.L1Handlers, adaptedClass.EntryPoints.L1Handler)
		fill(dep.Constructors, adaptedClass.EntryPoints.Constructor)

		compiledClass.Class = &junopb.CompiledClass_Deprecated{Deprecated: dep}
		compiledClass.AbiLength = uint32(len(c.Abi))

	case *core.SierraClass:
		if c.Compiled == nil {
			return nil, errors.New("sierra class doesnt have a compiled class associated with it")
		}
		compiledClass.CairoVersion = 1
		compiledClass.AbiLength = uint32(len(c.Abi))
		compiledClass.SierraProgramLength = uint32(len(c.Program))
		compiledClass.SierraVersion = c.SierraVersion()

		adaptedClass := core2sn.AdaptCasmClass(c.Compiled)
		bytecodeSegmentLengths, err := json.Marshal(adaptedClass.BytecodeSegmentLengths)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal bytecode segment lengths: %w", err)
		}

		pb := &junopb.CasmClass{
			Prime:                  adaptedClass.Prime,
			Bytecode:               make([]*junopb.Felt, len(adaptedClass.Bytecode)),
			HintsJson:              []byte(adaptedClass.Hints),
			PythonicHintsJson:      []byte(adaptedClass.PythonicHints),
			CompilerVersion:        adaptedClass.CompilerVersion,
			BytecodeSegmentLengths: bytecodeSegmentLengths,
			External:               make([]*junopb.CompiledEntryPoint, len(adaptedClass.EntryPoints.External)),
			L1Handler:              make([]*junopb.CompiledEntryPoint, len(adaptedClass.EntryPoints.L1Handler)),
			Constructor:            make([]*junopb.CompiledEntryPoint, len(adaptedClass.EntryPoints.Constructor)),
		}
		for i := range adaptedClass.Bytecode {
			pb.Bytecode[i] = toPBFelt(adaptedClass.Bytecode[i])
		}
		fillEP := func(dst []*junopb.CompiledEntryPoint, src []starknet.CompiledEntryPoint) {
			for i := range src {
				dst[i] = &junopb.CompiledEntryPoint{
					Selector: toPBFelt(src[i].Selector),
					Offset:   src[i].Offset,
					Builtins: append([]string(nil), src[i].Builtins...),
				}
			}
		}
		fillEP(pb.External, adaptedClass.EntryPoints.External)
		fillEP(pb.L1Handler, adaptedClass.EntryPoints.L1Handler)
		fillEP(pb.Constructor, adaptedClass.EntryPoints.Constructor)

		compiledClass.Class = &junopb.CompiledClass_Casm{Casm: pb}

	default:
		return nil, fmt.Errorf("unsupported class type %T", c)
	}

	return proto.Marshal(compiledClass)
}
