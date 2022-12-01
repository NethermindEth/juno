package contract

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	fmt.Println(c.ProgramHash)
	hashFelt = append(hashFelt, c.ProgramHash)
	a, _ := new(felt.Felt).SetString("0x88562ac88adfc7760ff452d048d39d72978bcc0f8d7b0fcfb34f33970b3df3")
	// b, _ := crypto.PedersenArray(hashFelt...)
	fmt.Println(a.Text(10))

	// Pedersen hash of the bytecode
	bytecodeHash, err := crypto.PedersenArray(c.Bytecode...)
	if err != nil {
		return nil, err
	}
	fmt.Println(bytecodeHash)
	hashFelt = append(hashFelt, bytecodeHash)

	// Pedersen hash of all the above
	classHash, err := crypto.PedersenArray(hashFelt...)
	if err != nil {
		return nil, err
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
	Name   string `json:"name"`
	Type   string `json:"type"`
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs"`
	Outputs []interface{} `json:"outputs"`
}

type Program struct {
	Builtins         []string     `json:"builtins"`
	Prime            string       `json:"prime"`
	ReferenceManager interface{}  `json:"reference_manager"`
	Identifiers      interface{}  `json:"identifiers"`
	Attributes       interface{}  `json:"attributes"`
	Data             []*felt.Felt `json:"data"`
	DebugInfo        interface{}  `json:"debug_info"`
	MainScope        interface{}  `json:"main_scope"`
	Hints            interface{}  `json:"hints"`
	CompilerVersion  string       `json:"compiler_version"`
}

type ClassDefinition struct {
	Abi         Abi `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program Program `json:"program"`
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

	// Get Builtins
	class.Builtins = definition.Program.Builtins

	// make debug info None
	definition.Program.DebugInfo = nil

	// Cairo 0.8 added "accessible_scopes" and "flow_tracking_data" attribute fields, which were
	// not present in older contracts. They present as null/empty for older contracts deployed
	// prior to adding this feature and should not be included in the hash calculation in these cases.
	//
	// We therefore check and remove them from the definition before calculating the hash.
	attributes := definition.Program.Attributes.([]interface{})
	if len(attributes) == 0 {
		definition.Program.Attributes = nil
	} else {
		for key, attribute := range attributes {
			attributeInterface := attribute.(map[string]interface{})
			if len(attributeInterface["accessible_scopes"].([]interface{})) == 0 {
				delete(attributeInterface, "accessible_scopes")
			}

			if len(attributeInterface["flow_tracking_data"].(map[string]interface{})) == 0 {
				delete(attributeInterface, "flow_tracking_data")
			}

			attributes[key] = attributeInterface
		}

		definition.Program.Attributes = attributes
	}

	// Compilers before version 0.10.0 use the "(a : felt)" syntax instead of the "(a: felt)" syntax.
	// So, we need to add an extra space before the colon in these cases for backwards compatibility.
	//
	// If compiler_version is not present, this was compiled with a compiler before version 0.10.0.
	if definition.Program.CompilerVersion == "" {
		identifiers := definition.Program.Identifiers.(map[string]interface{})
		if identifiers != nil {
			identifiers, err := addExtraSpaceToCairoNamedTuples(identifiers)
			if err != nil {
				return Class{}, err
			}
			definition.Program.Identifiers = identifiers
		}

		referenceManager := definition.Program.ReferenceManager.(map[string]interface{})
		if referenceManager != nil {
			referenceManager, err := addExtraSpaceToCairoNamedTuples(referenceManager)
			if err != nil {
				return Class{}, err
			}
			definition.Program.ReferenceManager = referenceManager
		}
	}

	// TODO: Implement https://github.com/eqlabs/pathfinder/blob/98cb3549f3b9fbb0010b984c2b9e33f15daa82dc/crates/pathfinder/src/state/class_hash.rs#L172-L182

	// Convert update program to bytes
	programBytes, err := json.Marshal(definition.Program)
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
	class.Bytecode = definition.Program.Data

	return *class, err
}

// TODO: Confirm whether this is the correct way to update (a: felt)
func addExtraSpaceToCairoNamedTuples(value interface{}) (interface{}, error) {
	switch value.(type) {
	case []interface{}:
		valueArray := value.([]interface{})
		for i, v := range valueArray {
			result, err := addExtraSpaceToCairoNamedTuples(v)
			if err != nil {
				return nil, err
			}
			valueArray[i] = result
		}
		return valueArray, nil
	case map[string]interface{}:
		valueMap := value.(map[string]interface{})
		for k, v := range valueMap {
			if reflect.TypeOf(v).Kind() == reflect.String {
				if k == "value" || k == "cairo_type" {
					v = strings.ReplaceAll(v.(string), ": ", " : ")
					valueMap[k] = strings.ReplaceAll(v.(string), "  :", " :")
				} else {
					valueMap[k] = v
				}
			} else {
				result, err := addExtraSpaceToCairoNamedTuples(v)
				if err != nil {
					return nil, err
				}
				valueMap[k] = result
			}
		}
		return valueMap, nil
	default:
		return value, nil
	}
}
