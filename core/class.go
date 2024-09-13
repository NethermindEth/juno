package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/wk8/go-ordered-map/v2"
)

var (
	_ Class = (*Cairo0Class)(nil)
	_ Class = (*Cairo1Class)(nil)
)

const (
	debugInfo = "debug_info"
)

// Class unambiguously defines a [Contract]'s semantics.
type Class interface {
	Version() uint64
	Hash() (*felt.Felt, error)
}

// Cairo0Class unambiguously defines a [Contract]'s semantics.
type Cairo0Class struct {
	Abi json.RawMessage
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// Base64 encoding of compressed Program
	Program string
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt
}

func (c *Cairo0Class) Version() uint64 {
	return 0
}

//nolint:funlen
func (c *Cairo0Class) Hash() (*felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(c)
	if err != nil {
		return nil, err
	}

	var program Program
	err = json.Unmarshal(definition.Program, &program)
	if err != nil {
		return nil, err
	}

	externalEntryPointElements := make([]*felt.Felt, 0, len(definition.EntryPoints.External)*2)
	l1HandlerEntryPointElements := make([]*felt.Felt, 0, len(definition.EntryPoints.L1Handler)*2)
	constructorEntryPointElements := make([]*felt.Felt, 0, len(definition.EntryPoints.Constructor)*2)
	builtInsHashElements := make([]*felt.Felt, 0, len(program.Builtins))
	dataHashElements := make([]*felt.Felt, 0, len(program.Data))

	// Use goroutines to parallelize hash computations
	var wg sync.WaitGroup
	var externalEntryPointHash, l1HandlerEntryPointHash, constructorEntryPointHash, builtInsHash, dataHash *felt.Felt
	var hintedClassHash *felt.Felt
	var hintedClassHashErr error

	wg.Add(6) //nolint:mnd

	go func() {
		defer wg.Done()
		for _, ep := range definition.EntryPoints.External {
			externalEntryPointElements = append(externalEntryPointElements, ep.Selector, ep.Offset)
		}
		externalEntryPointHash = crypto.PedersenArray(externalEntryPointElements...)
	}()

	go func() {
		defer wg.Done()
		for _, ep := range definition.EntryPoints.L1Handler {
			l1HandlerEntryPointElements = append(l1HandlerEntryPointElements, ep.Selector, ep.Offset)
		}
		l1HandlerEntryPointHash = crypto.PedersenArray(l1HandlerEntryPointElements...)
	}()

	go func() {
		defer wg.Done()
		for _, ep := range definition.EntryPoints.Constructor {
			constructorEntryPointElements = append(constructorEntryPointElements, ep.Selector, ep.Offset)
		}
		constructorEntryPointHash = crypto.PedersenArray(constructorEntryPointElements...)
	}()

	go func() {
		defer wg.Done()
		for _, builtIn := range program.Builtins {
			builtInHex := hex.EncodeToString([]byte(builtIn))
			builtInFelt, err := new(felt.Felt).SetString("0x" + builtInHex)
			if err != nil {
				return
			}
			builtInsHashElements = append(builtInsHashElements, builtInFelt)
		}
		builtInsHash = crypto.PedersenArray(builtInsHashElements...)
	}()

	go func() {
		defer wg.Done()
		hintedClassHash, hintedClassHashErr = computeHintedClassHash(definition.Abi, definition.Program)
	}()

	go func() {
		defer wg.Done()
		for _, data := range program.Data {
			dataFelt, err := new(felt.Felt).SetString(data)
			if err != nil {
				return
			}
			dataHashElements = append(dataHashElements, dataFelt)
		}
		dataHash = crypto.PedersenArray(dataHashElements...)
	}()

	wg.Wait()

	if hintedClassHashErr != nil {
		return nil, hintedClassHashErr
	}

	clashHash := crypto.PedersenArray(
		&felt.Zero,
		externalEntryPointHash,
		l1HandlerEntryPointHash,
		constructorEntryPointHash,
		builtInsHash,
		hintedClassHash,
		dataHash,
	)

	return clashHash, nil
}

func computeHintedClassHash(abi, program json.RawMessage) (*felt.Felt, error) {
	var mProgram Program
	d := json.NewDecoder(bytes.NewReader(program))
	d.UseNumber()
	if err := d.Decode(&mProgram); err != nil {
		return nil, err
	}
	err := mProgram.Format()
	if err != nil {
		return nil, err
	}

	formattedProgramBytes, err := json.Marshal(mProgram)
	if err != nil {
		return nil, err
	}

	formattedSpacesProgramStr, err := utils.FormatJSONString(string(formattedProgramBytes))
	if err != nil {
		return nil, err
	}

	stringifyABI, err := stringify(abi, nullSkipReplacer)
	if err != nil {
		return nil, err
	}
	formattedABI, err := utils.FormatJSONString(stringifyABI)
	if err != nil {
		return nil, err
	}

	// Use a more efficient string concatenation method
	var hintedClassHashJSON strings.Builder
	hintedClassHashJSON.Grow(len(formattedABI) + len(formattedSpacesProgramStr))
	hintedClassHashJSON.WriteString("{\"abi\": ")
	hintedClassHashJSON.WriteString(formattedABI)
	hintedClassHashJSON.WriteString(", \"program\": ")
	hintedClassHashJSON.WriteString(formattedSpacesProgramStr)
	hintedClassHashJSON.WriteString("}")

	return crypto.StarknetKeccak([]byte(hintedClassHashJSON.String())), nil
}

func makeDeprecatedVMClass(class *Cairo0Class) (*starknet.Cairo0Definition, error) {
	adaptEntryPoint := func(ep EntryPoint) starknet.EntryPoint {
		return starknet.EntryPoint{
			Selector: ep.Selector,
			Offset:   ep.Offset,
		}
	}

	constructors := utils.Map(utils.NonNilSlice(class.Constructors), adaptEntryPoint)
	external := utils.Map(utils.NonNilSlice(class.Externals), adaptEntryPoint)
	handlers := utils.Map(utils.NonNilSlice(class.L1Handlers), adaptEntryPoint)

	decompressedProgram, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	return &starknet.Cairo0Definition{
		Program: decompressedProgram,
		Abi:     class.Abi,
		EntryPoints: starknet.EntryPoints{
			Constructor: constructors,
			External:    external,
			L1Handler:   handlers,
		},
	}, nil
}

// applyReplacer recursively applies the replacer function to the JSON data
func applyReplacer(data interface{}, replacer func(string, interface{}) interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			v[key] = applyReplacer(replacer(key, val), replacer)
			if v[key] == nil && key != debugInfo {
				delete(v, key)
			}
		}

		return v
	case []interface{}:
		for i, val := range v {
			v[i] = applyReplacer(replacer("", val), replacer)
		}
		return v
	case *orderedmap.OrderedMap[string, interface{}]:
		for pair := v.Oldest(); pair != nil; pair = pair.Next() {
			val := applyReplacer(replacer(pair.Key, pair.Value), replacer)
			if val == nil {
				v.Delete(pair.Key)
			} else {
				v.Set(pair.Key, val)
			}
		}
		return v
	default:
		return replacer("", v)
	}
}

// nullSkipReplacer is a custom JSON replacer that handles specific keys and null values
func nullSkipReplacer(key string, value interface{}) interface{} {
	switch key {
	case "attributes", "accessible_scopes", "flow_tracking_data":
		if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
			return nil
		}
	case debugInfo:
		return nil
	}

	return value
}

// identifiersNullSkipReplacer is same as nullSkipReplacer but only used for identifiers field
func identifiersNullSkipReplacer(key string, value interface{}) interface{} {
	switch key {
	case "cairo_type":
		if str, ok := value.(string); ok {
			return strings.ReplaceAll(str, ": ", " : ")
		}
	case "attributes", "accessible_scopes", "flow_tracking_data":
		if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
			return nil
		}
	case debugInfo:
		return nil
	}

	return value
}

// stringify converts a Go value to a JSON string, using a custom replacer function
func stringify(value interface{}, replacer func(string, interface{}) interface{}) (string, error) {
	// Marshal the value to JSON
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}

	// Determine the type of the JSON data
	var jsonData interface{}
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return "", err
	}

	// Apply the replacer function recursively
	modifiedData := applyReplacer(jsonData, replacer)

	// Marshal the modified data back to JSON
	modifiedJSONBytes, err := json.Marshal(modifiedData)
	if err != nil {
		return "", err
	}

	// Convert the JSON bytes to a string and return
	return string(modifiedJSONBytes), nil
}

// Cairo1Class unambiguously defines a [Contract]'s semantics.
type Cairo1Class struct {
	Abi         string
	AbiHash     *felt.Felt
	EntryPoints struct {
		Constructor []SierraEntryPoint
		External    []SierraEntryPoint
		L1Handler   []SierraEntryPoint
	}
	Program         []*felt.Felt
	ProgramHash     *felt.Felt
	SemanticVersion string
	Compiled        *CompiledClass
}

type SegmentLengths struct {
	Children []SegmentLengths
	Length   uint64
}

type CompiledClass struct {
	Bytecode               []*felt.Felt
	PythonicHints          json.RawMessage
	CompilerVersion        string
	Hints                  json.RawMessage
	Prime                  *big.Int
	External               []CompiledEntryPoint
	L1Handler              []CompiledEntryPoint
	Constructor            []CompiledEntryPoint
	BytecodeSegmentLengths SegmentLengths
}

type CompiledEntryPoint struct {
	Offset   uint64
	Builtins []string
	Selector *felt.Felt
}

type SierraEntryPoint struct {
	Index    uint64
	Selector *felt.Felt
}

func (c *Cairo1Class) Version() uint64 {
	return 1
}

func (c *Cairo1Class) Hash() (*felt.Felt, error) {
	return crypto.PoseidonArray(
		new(felt.Felt).SetBytes([]byte("CONTRACT_CLASS_V"+c.SemanticVersion)),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.External)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.L1Handler)...),
		crypto.PoseidonArray(flattenSierraEntryPoints(c.EntryPoints.Constructor)...),
		c.AbiHash,
		c.ProgramHash,
	), nil
}

var compiledClassV1Prefix = new(felt.Felt).SetBytes([]byte("COMPILED_CLASS_V1"))

func (c *CompiledClass) Hash() *felt.Felt {
	var bytecodeHash *felt.Felt
	if len(c.BytecodeSegmentLengths.Children) == 0 {
		bytecodeHash = crypto.PoseidonArray(c.Bytecode...)
	} else {
		bytecodeHash = SegmentedBytecodeHash(c.Bytecode, c.BytecodeSegmentLengths.Children)
	}

	return crypto.PoseidonArray(
		compiledClassV1Prefix,
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.External)...),
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.L1Handler)...),
		crypto.PoseidonArray(flattenCompiledEntryPoints(c.Constructor)...),
		bytecodeHash,
	)
}

func SegmentedBytecodeHash(bytecode []*felt.Felt, segmentLengths []SegmentLengths) *felt.Felt {
	var startingOffset uint64
	var digestSegment func(segments []SegmentLengths) (uint64, *felt.Felt)
	digestSegment = func(segments []SegmentLengths) (uint64, *felt.Felt) {
		var totalLength uint64
		var digest crypto.PoseidonDigest
		for _, segment := range segments {
			var curSegmentLength uint64
			var curSegmentHash *felt.Felt

			if len(segment.Children) == 0 {
				curSegmentLength = segment.Length
				segmentBytecode := bytecode[startingOffset : startingOffset+segment.Length]
				curSegmentHash = crypto.PoseidonArray(segmentBytecode...)
			} else {
				curSegmentLength, curSegmentHash = digestSegment(segment.Children)
			}

			digest.Update(new(felt.Felt).SetUint64(curSegmentLength))
			digest.Update(curSegmentHash)

			startingOffset += curSegmentLength
			totalLength += curSegmentLength
		}
		digestRes := digest.Finish()
		return totalLength, digestRes.Add(digestRes, new(felt.Felt).SetUint64(1))
	}

	_, hash := digestSegment(segmentLengths)
	return hash
}

func flattenSierraEntryPoints(entryPoints []SierraEntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*2)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first because the order
		// influences the class hash.
		result[2*i] = entryPoint.Selector
		result[2*i+1] = new(felt.Felt).SetUint64(entryPoint.Index)
	}
	return result
}

func flattenCompiledEntryPoints(entryPoints []CompiledEntryPoint) []*felt.Felt {
	result := make([]*felt.Felt, len(entryPoints)*3)
	for i, entryPoint := range entryPoints {
		// It is important that Selector is first, then Offset is second because the order
		// influences the class hash.
		result[3*i] = entryPoint.Selector
		result[3*i+1] = new(felt.Felt).SetUint64(entryPoint.Offset)
		builtins := make([]*felt.Felt, len(entryPoint.Builtins))
		for idx, buil := range entryPoint.Builtins {
			builtins[idx] = new(felt.Felt).SetBytes([]byte(buil))
		}
		result[3*i+2] = crypto.PoseidonArray(builtins...)
	}

	return result
}

func VerifyClassHashes(classes map[felt.Felt]Class) error {
	for hash, class := range classes {
		if _, ok := class.(*Cairo0Class); ok {
			// skip validation of cairo0 class hash
			continue
		}

		cHash, err := class.Hash()
		if err != nil {
			return err
		}

		if !cHash.Equal(&hash) {
			return fmt.Errorf("cannot verify class hash: calculated hash %v, received hash %v", cHash.String(), hash.String())
		}
	}

	return nil
}
