package core

//#include <stdint.h>
//#include <stdlib.h>
//#include <stddef.h>
//
// extern void Cairo0ClassHash(char* class_json_str, char* hash);
// #cgo vm_debug  LDFLAGS: -L./rust/target/debug   -ljuno_starknet_core_rs
// #cgo !vm_debug LDFLAGS: -L./rust/target/release -ljuno_starknet_core_rs
import "C"

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/wk8/go-ordered-map/v2"
)

func cairo0ClassHashNoFFI(class *Cairo0Class) (*felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(class)
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

	wg.Add(6)

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
	hintedClassHashJSON.Grow(len(formattedABI) + len(formattedSpacesProgramStr) + 20)
	hintedClassHashJSON.WriteString("{\"abi\": ")
	hintedClassHashJSON.WriteString(formattedABI)
	hintedClassHashJSON.WriteString(", \"program\": ")
	hintedClassHashJSON.WriteString(formattedSpacesProgramStr)
	hintedClassHashJSON.WriteString("}")

	f, err := os.Create("juno_hinted_class.txt")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = f.WriteString(hintedClassHashJSON.String())

	return crypto.StarknetKeccak([]byte(hintedClassHashJSON.String())), nil
}

func cairo0ClassHash(class *Cairo0Class) (*felt.Felt, error) {
	definition, err := makeDeprecatedVMClass(class)
	if err != nil {
		return nil, err
	}

	classJSON, err := json.Marshal(definition)
	if err != nil {
		return nil, err
	}
	classJSONCStr := cstring(classJSON)

	var hash felt.Felt
	hashBytes := hash.Bytes()

	C.Cairo0ClassHash(classJSONCStr, (*C.char)(unsafe.Pointer(&hashBytes[0])))
	hash.SetBytes(hashBytes[:])
	C.free(unsafe.Pointer(classJSONCStr))
	if hash.IsZero() {
		return nil, errors.New("failed to calculate class hash")
	}
	return &hash, nil
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

// nullSkipReplacer is a custom JSON replacer that handles specific keys and null values
func nullSkipReplacer(key string, value interface{}) interface{} {
	switch key {
	case "attributes", "accessible_scopes", "flow_tracking_data":
		if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
			return nil
		}
	case "debug_info":
		return nil
	}

	return value
}

func identifiersNullSkipReplacer(key string, value interface{}) interface{} {
	switch key {
	case "cairo_type":
		if str, ok := value.(string); ok {
			return strings.Replace(str, ": ", " : ", -1)
		}
	case "attributes", "accessible_scopes", "flow_tracking_data":
		if arr, ok := value.([]interface{}); ok && len(arr) == 0 {
			return nil
		}
	case "debug_info":
		return nil
	}

	return value
}

// applyReplacer recursively applies the replacer function to the JSON data
func applyReplacer(data interface{}, replacer func(string, interface{}) interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			v[key] = applyReplacer(replacer(key, val), replacer)
			if v[key] == nil && key != "debug_info" {
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
	modifiedJsonBytes, err := json.Marshal(modifiedData)
	if err != nil {
		return "", err
	}

	// Convert the JSON bytes to a string and return
	return string(modifiedJsonBytes), nil
}

// cstring creates a null-terminated C string from the given byte slice.
// the caller is responsible for freeing the underlying memory
func cstring(data []byte) *C.char {
	str := unsafe.String(unsafe.SliceData(data), len(data))
	return C.CString(str)
}
