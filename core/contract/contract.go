package contract

import (
	"encoding/json"
	"reflect"
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
	Selector *felt.Felt
	// The offset of the instruction in the class's bytecode.
	Offset uint
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

// Creates new empty class instance
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

// Hash computes the [Pedersen Hash] of the class.
//
// [Pedersen Hash]: https://docs.starknet.io/documentation/develop/Contracts/contract-hash/#how_the_class_hash_is_computed
func (c *Class) ClassHash() (*felt.Felt, error) {
	var hashFelt []*felt.Felt

	zeroNonce := new(felt.Felt).SetUint64(0)
	hashFelt = append(hashFelt, zeroNonce)

	apiVersion := zeroNonce.SetUint64(uint64(c.APIVersion))
	hashFelt = append(hashFelt, apiVersion)

	// Pederesen hash of the externals
	if len(c.Externals) > 0 {
		externalsPedersen, err := EntryPointToFelt(c.Externals)
		if err != nil {
			return nil, err
		}
		hashFelt = append(hashFelt, externalsPedersen)
	}

	// Pederesen hash of the L1 handlers
	if len(c.L1Handlers) > 0 {
		l1HandlersPedersen, err := EntryPointToFelt(c.L1Handlers)
		if err != nil {
			return nil, err
		}
		hashFelt = append(hashFelt, l1HandlersPedersen)
	}

	// Pederesen hash of the constructors
	if len(c.Constructors) > 0 {
		constructorsPedersen, err := EntryPointToFelt(c.Constructors)
		if err != nil {
			return nil, err
		}
		hashFelt = append(hashFelt, constructorsPedersen)
	}

	// Convert builtin array to felt
	for _, builtin := range c.Builtins {
		builtinFelt := new(felt.Felt).SetBytes([]byte(builtin))
		hashFelt = append(hashFelt, builtinFelt)
	}

	hashFelt = append(hashFelt, c.ProgramHash)

	// Pederesen hash of the bytecode
	if len(c.Bytecode) > 2 {
		bytecodePedersen, err := crypto.PedersenArray(c.Bytecode...)
		if err != nil {
			return nil, err
		}
		hashFelt = append(hashFelt, bytecodePedersen)
	} else if len(c.Bytecode) == 2 {
		bytecodePedersen, err := crypto.Pedersen(c.Bytecode[0], c.Bytecode[1])
		if err != nil {
			return nil, err
		}
		hashFelt = append(hashFelt, bytecodePedersen)
	}

	// Pedersen hash of all the above
	classHash, err := crypto.PedersenArray(hashFelt...)
	if err != nil {
		return nil, err
	}

	return classHash, nil
}

// Hash computes the address of a StarkNet contract.
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
	for _, entry := range entryPoint {
		entryPointFeltArray = append(entryPointFeltArray, entry.Selector)
		entryPointFeltArray = append(entryPointFeltArray, new(felt.Felt).SetUint64(uint64(entry.Offset)))
	}

	var entryPointPedersen *felt.Felt
	// Pederesen hash of the entry points
	if len(entryPointFeltArray) == 2 {
		pedersen, err := crypto.Pedersen(entryPointFeltArray[0], entryPointFeltArray[1])
		if err != nil {
			return nil, err
		}
		entryPointPedersen = pedersen
	} else if len(entryPointFeltArray) > 2 {
		pedersen, err := crypto.PedersenArray(entryPointFeltArray...)
		if err != nil {
			return nil, err
		}
		entryPointPedersen = pedersen
	}
	return entryPointPedersen, nil
}

func GenerateClass(contractDefinition []byte) (Class, error) {
	c := NewClass()
	c.APIVersion = 0

	var fullContract map[string]interface{}
	err := json.Unmarshal(contractDefinition, &fullContract)
	if err != nil {
		return Class{}, err
	}

	// Get Entry Points
	entryPoints := make(map[string][]EntryPoint)
	entryPointsByType := fullContract["entry_points_by_type"].(map[string]interface{})
	for key, entryPoint := range entryPointsByType {
		entryPointsInterface := entryPoint.([]interface{})
		var offsetsAndSelectors []EntryPoint
		for _, entryPointInterface := range entryPointsInterface {
			entryPointMap := entryPointInterface.(map[string]interface{})
			offset := entryPointMap["offset"].(string)
			offset = strings.Replace(offset, "0x", "", -1)
			offsetUint, err := strconv.ParseInt(offset, 16, 64)
			if err != nil {
				return Class{}, err
			}
			selector := entryPointMap["selector"].(string)
			offsetsAndSelectors = append(offsetsAndSelectors, EntryPoint{
				Offset:   uint(offsetUint),
				Selector: new(felt.Felt).SetBytes([]byte(selector)),
			})
		}

		entryPoints[key] = offsetsAndSelectors
	}

	c.Constructors = entryPoints["CONSTRUCTOR"]
	c.L1Handlers = entryPoints["L1_HANDLER"]
	c.Externals = entryPoints["EXTERNAL"]

	program := fullContract["program"].(map[string]interface{})

	// Get Builtins
	builtins := program["builtins"].([]interface{})
	for _, builtin := range builtins {
		c.Builtins = append(c.Builtins, builtin.(string))
	}

	// make debug info None
	program["debug_info"] = nil

	// Cairo 0.8 added "accessible_scopes" and "flow_tracking_data" attribute fields, which were
	// not present in older contracts. They present as null/empty for older contracts deployed
	// prior to adding this feature and should not be included in the hash calculation in these cases.
	//
	// We therefore check and remove them from the definition before calculating the hash.
	attributes := program["attributes"].([]interface{})

	if len(attributes) == 0 {
		delete(program, "attributes")
	} else {
		for _, attribute := range attributes {
			attributeInterface := attribute.(map[string]interface{})
			if len(attributeInterface["accessible_scopes"].([]interface{})) == 0 {
				delete(attributeInterface, "accessible_scopes")
			}

			if len(attributeInterface["flow_tracking_data"].([]interface{})) == 0 {
				delete(attributeInterface, "flow_tracking_data")
			}
		}
	}

	// Compilers before version 0.10.0 use the "(a : felt)" syntax instead of the "(a: felt)" syntax.
	// So, we need to add an extra space before the colon in these cases for backwards compatibility.
	//
	// If compiler_version is not present, this was compiled with a compiler before version 0.10.0.
	if program["compiler_version"] == nil {
		identifiers := program["identifiers"]
		if identifiers != nil {
			identifiers, err := addExtraSpaceToCairoNamedTuples(identifiers)
			if err != nil {
				return Class{}, err
			}
			program["identifiers"] = identifiers
		}

		referenceManager := program["reference_manager"]
		if referenceManager != nil {
			referenceManager, err := addExtraSpaceToCairoNamedTuples(referenceManager)
			if err != nil {
				return Class{}, err
			}
			program["reference_manager"] = referenceManager
		}
	}

	// TODO: Implement https://github.com/eqlabs/pathfinder/blob/98cb3549f3b9fbb0010b984c2b9e33f15daa82dc/crates/pathfinder/src/state/class_hash.rs#L172-L182

	programKeccak, err := crypto.StarkNetKeccak(contractDefinition)
	if err != nil {
		return Class{}, err
	}
	c.ProgramHash = programKeccak

	// Get Bytecode
	bytecode := program["data"].([]interface{})
	for _, code := range bytecode {
		byteCodeFelt, err := new(felt.Felt).SetString((code.(string)))
		if err != nil {
			return Class{}, err
		}
		c.Bytecode = append(c.Bytecode, byteCodeFelt)
	}

	return *c, err
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
