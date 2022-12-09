package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

// Contract is an instance of a [Class].
type Contract struct {
	// The number of transactions sent from this contract.
	// Only account contracts can have a non-zero nonce.
	Nonce uint
	// Hash of the class that this contract instantiates.
	ClassHash *felt.Felt
	// Root of the contract's storage trie.
	StorageRoot *felt.Felt // TODO: is this field necessary?
}

// EntryPoint uniquely identifies a Cairo function to execute.
type EntryPoint struct {
	// starknet_keccak hash of the function signature.
	Selector *felt.Felt `json:"selector"`
	// The offset of the instruction in the class's bytecode.
	Offset *felt.Felt `json:"offset"`
}

// Class unambiguously defines a [Contract]'s semantics.
type Class struct {
	// The version of the class, currently always 0.
	APIVersion uint
	// External functions defined in the class.
	Externals []EntryPoint
	// Functions that receive L1 messages. See
	// https://www.cairo-lang.org/docs/hello_starknet/l1l2.html#receiving-a-message-from-l1
	L1Handlers []EntryPoint
	// Constructors for the class. Currently, only one is allowed.
	Constructors []EntryPoint
	// An ascii-encoded array of builtin names imported by the class.
	Builtins []string
	// The starknet_keccak hash of the ".json" file compiler output.
	ProgramHash *felt.Felt
	Bytecode    []*felt.Felt
}

// NewClass Creates new empty class instance
func NewClass() *Class {
	return &Class{
		APIVersion:   0,
		Externals:    []EntryPoint{},
		L1Handlers:   []EntryPoint{},
		Constructors: []EntryPoint{},
		Builtins:     []string{},
		ProgramHash:  nil,
		Bytecode:     []*felt.Felt{},
	}
}

// ClassHash computes the [Pedersen Hash] of the class.
//
// [Pedersen Hash]: https://docs.starknet.io/documentation/develop/Contracts/contract-hash/#how_the_class_hash_is_computed
func (c *Class) ClassHash() (*felt.Felt, error) {
	var hashFelt []*felt.Felt

	apiVersion := new(felt.Felt).SetUint64(uint64(c.APIVersion))
	hashFelt = append(hashFelt, apiVersion)

	// Pedersen hash of the externals
	externalsPedersen, err := EntryPointToFelt(c.Externals)
	if err != nil {
		return nil, err
	}
	hashFelt = append(hashFelt, externalsPedersen)

	// Pedersen hash of the L1 handlers
	l1HandlersPedersen, err := EntryPointToFelt(c.L1Handlers)
	if err != nil {
		return nil, err
	}
	hashFelt = append(hashFelt, l1HandlersPedersen)

	// Pedersen hash of the constructors
	constructorsPedersen, err := EntryPointToFelt(c.Constructors)
	if err != nil {
		return nil, err
	}
	hashFelt = append(hashFelt, constructorsPedersen)

	// Pedersen hash of the builtins
	//
	// Convert []string to []*felt.Felt
	var builtinFeltArray []*felt.Felt
	if len(c.Builtins) > 0 {
		for _, builtin := range c.Builtins {
			builtinFelt := new(felt.Felt).SetBytes([]byte(builtin))
			builtinFeltArray = append(builtinFeltArray, builtinFelt)
		}
	}
	builtinHash, err := crypto.PedersenArray(builtinFeltArray...)
	if err != nil {
		return nil, err
	}
	hashFelt = append(hashFelt, builtinHash)
	hashFelt = append(hashFelt, c.ProgramHash)

	// Pedersen hash of the bytecode
	bytecodeHash, err := crypto.PedersenArray(c.Bytecode...)
	if err != nil {
		return nil, err
	}
	hashFelt = append(hashFelt, bytecodeHash)

	// Pedersen hash of all the above
	classHash, err := crypto.PedersenArray(hashFelt...)
	if err != nil {
		return nil, err
	}

	textClassHash, err := new(felt.Felt).SetString("0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3")
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(classHash, textClassHash) {
		fmt.Println("class hash mismatch: ", classHash, " != ", textClassHash, "")
		return nil, fmt.Errorf("class hash mismatch: %s != %s", classHash, textClassHash)
	}

	return classHash, nil
}

// Address computes the address of a StarkNet contract.
func (c *Class) Address(callerAddress, salt *felt.Felt, constructorCalldata []*felt.Felt) (*felt.Felt, error) {
	prefix := new(felt.Felt).SetBytes([]byte("STARKNET_CONTRACT_ADDRESS"))
	classHash, err := c.ClassHash()
	if err != nil {
		return nil, err
	}
	callerPedersenHash, err := crypto.PedersenArray(constructorCalldata...)
	if err != nil {
		return nil, err
	}

	// Pedersen hash of prefix, caller address, salt, class hash, and constructor calldata
	address, err := crypto.PedersenArray(prefix, callerAddress, salt, classHash, callerPedersenHash)
	if err != nil {
		return nil, err
	}

	return address, nil
}

func EntryPointToFelt(entryPoint []EntryPoint) (*felt.Felt, error) {
	// Convert entry points to felt
	var entryPointFeltArray []*felt.Felt
	if len(entryPoint) > 0 {
		for _, entry := range entryPoint {
			entryPointFeltArray = append(entryPointFeltArray, entry.Selector)
			entryPointFeltArray = append(entryPointFeltArray, entry.Offset)
		}
	}

	// Pedersen hash of the entry points
	entryPointPedersen, err := crypto.PedersenArray(entryPointFeltArray...)
	if err != nil {
		return nil, err
	}

	return entryPointPedersen, nil
}

type Abi []struct {
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs"`
	Name    string        `json:"name"`
	Outputs []interface{} `json:"outputs"`
	Type    string        `json:"type"`
}

type (
	Hints       map[uint64]interface{}
	Identifiers map[string]struct {
		CairoType   string        `json:"cairo_type,omitempty"`
		Destination string        `json:"destination,omitempty"`
		FullName    string        `json:"full_name,omitempty"`
		Members     interface{}   `json:"members,omitempty"`
		References  []interface{} `json:"references,omitempty"`
		Size        uint64        `json:"size,omitempty"`
		Type        string        `json:"type,omitempty"`
		Value       json.Number   `json:"value,omitempty"`
	}
	Program struct {
		Attributes       interface{} `json:"attributes,omitempty"`
		Builtins         []string    `json:"builtins"`
		CompilerVersion  string      `json:"compiler_version,omitempty"`
		Data             []string    `json:"data"`
		DebugInfo        interface{} `json:"debug_info"`
		Hints            Hints       `json:"hints"`
		Identifiers      interface{} `json:"identifiers,omitempty"`
		MainScope        interface{} `json:"main_scope"`
		Prime            string      `json:"prime"`
		ReferenceManager interface{} `json:"reference_manager"`
	}
	TypedProgram struct {
		Attributes       interface{} `json:"attributes,omitempty"`
		Builtins         []string    `json:"builtins"`
		CompilerVersion  string      `json:"compiler_version,omitempty"`
		Data             []string    `json:"data"`
		DebugInfo        interface{} `json:"debug_info"`
		Hints            Hints       `json:"hints"`
		Identifiers      Identifiers `json:"identifiers"`
		MainScope        interface{} `json:"main_scope"`
		Prime            string      `json:"prime"`
		ReferenceManager interface{} `json:"reference_manager"`
	}
)

type ClassDefinition struct {
	Abi         interface{} `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program Program `json:"program"`
}

type TypedClassDefinition struct {
	Abi         interface{} `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program TypedProgram `json:"program"`
}

type ContractCode struct {
	Abi     interface{} `json:"abi"`
	Program Program     `json:"program"`
}

func GenerateClass(contractDefinition []byte) (Class, error) {
	definition := new(ClassDefinition)
	err := json.Unmarshal(contractDefinition, &definition)
	if err != nil {
		fmt.Println("Error decoding contract definition")
		return Class{}, err
	}

	class := new(Class)
	class.APIVersion = 0

	// Get Entry Points
	class.Constructors = definition.EntryPoints.Constructor
	class.L1Handlers = definition.EntryPoints.L1Handler
	class.Externals = definition.EntryPoints.External

	program := definition.Program

	// Get Builtins
	builtins := program.Builtins
	class.Builtins = append(class.Builtins, builtins...)

	// make debug info None
	program.DebugInfo = nil

	// Cairo 0.8 added "accessible_scopes" and "flow_tracking_data" attribute fields, which were
	// not present in older contracts. They present as null/empty for older contracts deployed
	// prior to adding this feature and should not be included in the hash calculation in these cases.
	//
	// We therefore check and remove them from the definition before calculating the hash.
	if program.Attributes != nil {
		attributes := program.Attributes.([]interface{})
		if len(attributes) == 0 {
			program.Attributes = nil
		} else {
			for key, attribute := range attributes {
				attributeInterface := attribute.(map[string]interface{})
				if len(attributeInterface["accessible_scopes"].([]interface{})) == 0 {
					delete(attributeInterface, "accessible_scopes")
				}

				if attributeInterface["flow_tracking_data"] == nil {
					delete(attributeInterface, "flow_tracking_data")
				}

				attributes[key] = attributeInterface
			}

			program.Attributes = attributes
		}
	}

	contractCode := new(ContractCode)
	contractCode.Abi = definition.Abi
	contractCode.Program = program

	// Explanation of why we have contractDefinition and typedContractDefinition
	//
	// contractDefinition uses interface{} to marshal Identifiers and therefore maintain the
	// original order of the map[string]interface{}. However, it introduces a problem by
	// converting big numbers (e.g. -106710729501573572985208420194530329073740042555888586719489)
	// into floats (e.g. -1.0671072950157357e+59). This affects the computation.
	//
	// On the other hand, typedContractDefinition uses the Identifier type to marshal identifiers
	// without converting big numbers to floats. It, however, does not maintain the orginal order
	// of the objects. The order is important to generate the correct class hash.
	typedContractDefinition := new(TypedClassDefinition)
	err = json.Unmarshal(contractDefinition, &typedContractDefinition)
	if err != nil {
		fmt.Println("Error decoding contract definition")
		return Class{}, err
	}

	// Convert update program to bytes
	programBytes, err := contractCode.MarshalsJSON(typedContractDefinition.Program.Identifiers)
	if err != nil {
		fmt.Println("Error encoding program")
		return Class{}, err
	}

	programKeccak, err := crypto.StarkNetKeccak(programBytes)
	if err != nil {
		return Class{}, err
	}

	class.ProgramHash = programKeccak

	// Get Bytecode
	var bytecode []*felt.Felt
	for _, entry := range program.Data {
		code, err := new(felt.Felt).SetString(entry)
		if err != nil {
			return Class{}, err
		}
		bytecode = append(bytecode, code)
	}
	class.Bytecode = bytecode

	return *class, err
}

// Create custom json marshaller
func (c ContractCode) MarshalsJSON(identifiers Identifiers) ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.Write([]byte("{"))
	buf.Write([]byte("\"abi\": "))
	err := formatter(buf, c.Abi, "", identifiers)
	if err != nil {
		return nil, err
	}

	buf.Write([]byte(", "))
	buf.Write([]byte("\"program\": " + "{"))
	program := c.Program
	if program.Attributes != nil {
		buf.Write([]byte("\"attributes\": "))
		formatter(buf, program.Attributes, "", identifiers)
		buf.Write([]byte(", "))
	}
	buf.Write([]byte("\"builtins\": "))
	formatter(buf, program.Builtins, "", identifiers)
	buf.Write([]byte(", "))
	if program.CompilerVersion != "" {
		buf.Write([]byte("\"compiler_version\": "))
		formatter(buf, program.CompilerVersion, "", identifiers)
		buf.Write([]byte(", "))
	}
	buf.Write([]byte("\"data\": "))
	formatter(buf, program.Data, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"debug_info\": "))
	formatter(buf, program.DebugInfo, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"hints\": "))
	formatter(buf, program.Hints, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"identifiers\": "))
	formatter(buf, program.Identifiers, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"main_scope\": "))
	formatter(buf, program.MainScope, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"prime\": "))
	formatter(buf, program.Prime, "", identifiers)
	buf.Write([]byte(", "))
	buf.Write([]byte("\"reference_manager\": "))
	formatter(buf, program.ReferenceManager, "", identifiers)
	buf.Write([]byte("}}"))

	return buf.Bytes(), nil
}

func formatter(buf *bytes.Buffer, value interface{}, tree string, identifiers Identifiers) error {
	switch v := value.(type) {
	case string:
		var result string
		if strings.ContainsAny(v, "\n\\") {
			result = strings.ReplaceAll(v, "\\", "\\\\")
			result = strings.ReplaceAll(result, "\n", "\\n")
		} else {
			result = v
		}
		buf.WriteString("\"" + result + "\"")
	case uint:
		result := strconv.FormatUint(uint64(v), 10)
		buf.WriteString(result)
	case uint64:
		result := strconv.FormatUint(v, 10)
		buf.WriteString(result)
	case float64:
		result := strconv.FormatFloat(v, 'f', 0, 64)
		buf.WriteString(result)
	case json.Number:
		result := string(v)
		buf.WriteString(result)
	case map[string]interface{}:
		buf.Write([]byte{'{'})
		// Arrange lexigraphically
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			buf.Write([]byte{'"'})
			buf.WriteString(k)
			buf.Write([]byte{'"'})
			buf.Write([]byte(": "))
			if k == "value" && reflect.TypeOf(v[k]) == reflect.TypeOf(float64(0)) && v[k] != float64(0) {
				value := identifiers[tree].Value
				buf.Write([]byte(value))
			} else {
				if reflect.TypeOf(v[k]) == reflect.TypeOf(map[string]interface{}{}) {
					tree = k
				}
				err := formatter(buf, v[k], tree, identifiers)
				if err != nil {
					return err
				}
			}
			if i < len(keys)-1 {
				buf.Write([]byte(", "))
			}
		}
		buf.Write([]byte{'}'})
	case Hints:
		buf.Write([]byte{'{'})
		// Arrange numerically
		keys := make([]uint64, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for i, k := range keys {
			buf.Write([]byte{'"'})
			buf.WriteString(strconv.FormatUint(k, 10))
			buf.Write([]byte{'"'})
			buf.Write([]byte(": "))
			err := formatter(buf, v[k], "", identifiers)
			if err != nil {
				return err
			}
			if i < len(keys)-1 {
				buf.Write([]byte(", "))
			}
		}
		buf.Write([]byte{'}'})
	case []interface{}:
		buf.Write([]byte{'['})
		count := 0
		for _, value := range v {
			err := formatter(buf, value, "", identifiers)
			if err != nil {
				return err
			}
			if count < len(v)-1 {
				buf.Write([]byte(", "))
			}
			count++
		}
		buf.Write([]byte{']'})
	case []string:
		buf.Write([]byte{'['})
		count := 0
		for _, value := range v {
			buf.WriteString(`"` + value + `"`)
			if count < len(v)-1 {
				buf.Write([]byte(", "))
			}
			count++
		}
		buf.Write([]byte{']'})
	default:
		if value == nil {
			buf.WriteString("null")
		} else {
			fmt.Println("Unknown type: ", reflect.TypeOf(value))
			return fmt.Errorf("unknown type: %T", value)
		}
	}
	return nil
}
