package abi

import (
	"encoding/json"
	"fmt"
)

// Abi represents an ABI of a Starknet contract
type Abi struct {
	Functions   []Function
	Events      []Event
	Structs     []Struct
	L1Handlers  []L1Handler
	Constructor *Constructor
}

func (x *Abi) UnmarshalJSON(data []byte) error {
	// Unmarshal all the common parts of the fields to get the field types
	var common []FieldCommon
	if err := json.Unmarshal(data, &common); err != nil {
		return err
	}
	// Unmarshal as a list of raw fields to then unmarshal each to the correct
	// type stored at the same index in the common array
	items := make([]json.RawMessage, len(common))
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	// Unmarshal each raw field
	abi := new(Abi)
	for i, item := range items {
		switch common[i].Type {
		case TypeEvent:
			decodedItem := &Event{}
			if err := json.Unmarshal(item, decodedItem); err != nil {
				return err
			}
			abi.Events = append(abi.Events, *decodedItem)
		case TypeFunction:
			decodedItem := &Function{}
			if err := json.Unmarshal(item, decodedItem); err != nil {
				return err
			}
			abi.Functions = append(abi.Functions, *decodedItem)
		case TypeStruct:
			decodedItem := &Struct{}
			if err := json.Unmarshal(item, decodedItem); err != nil {
				return err
			}
			abi.Structs = append(abi.Structs, *decodedItem)
		case TypeConstructor:
			decodedItem := &Constructor{}
			if err := json.Unmarshal(item, decodedItem); err != nil {
				return err
			}
			abi.Constructor = decodedItem
		case TypeL1Handler:
			decodedItem := &L1Handler{}
			if err := json.Unmarshal(item, decodedItem); err != nil {
				return err
			}
			abi.L1Handlers = append(abi.L1Handlers, *decodedItem)
		default:
			return fmt.Errorf("unexpected type %s", common[i].Type)
		}
	}
	*x = *abi
	return nil
}

func (x *Abi) MarshalJSON() ([]byte, error) {
	n := len(x.Functions) + len(x.Events) + len(x.Structs)
	if x.Constructor != nil {
		n += 1
	}
	rawFields := make([]json.RawMessage, 0, n)
	// Marshal events
	for _, event := range x.Events {
		rawEvent, err := json.Marshal(&event)
		if err != nil {
			return nil, err
		}
		rawFields = append(rawFields, rawEvent)
	}
	// Marshal structs
	for _, _struct := range x.Structs {
		rawStruct, err := json.Marshal(&_struct)
		if err != nil {
			return nil, err
		}
		rawFields = append(rawFields, rawStruct)
	}
	// Marshal functions
	for _, function := range x.Functions {
		rawFunction, err := json.Marshal(&function)
		if err != nil {
			return nil, err
		}
		rawFields = append(rawFields, rawFunction)
	}
	// Marshal l1 handlers
	for _, l1Handler := range x.L1Handlers {
		rawL1Handler, err := json.Marshal(&l1Handler)
		if err != nil {
			return nil, err
		}
		rawFields = append(rawFields, rawL1Handler)
	}
	// Marshal constructor
	if x.Constructor != nil {
		rawConstructor, err := json.Marshal(x.Constructor)
		if err != nil {
			return nil, err
		}
		rawFields = append(rawFields, rawConstructor)
	}
	return json.Marshal(rawFields)
}

// Variable represents a variable item of the ABI, and at the same time, is a
// variable in the Cairo contract code
type Variable struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FieldType represents the type name of the all possible field types in an
// ABI
type FieldType string

const (
	TypeFunction    = "function"
	TypeEvent       = "event"
	TypeStruct      = "struct"
	TypeConstructor = "constructor"
	TypeL1Handler   = "l1_handler"
)

// FieldCommon has all the fields in common between all the possible ABI
// field types
type FieldCommon struct {
	Type FieldType `json:"type"`
}

// Function represents an ABI field of type function
type Function struct {
	FieldCommon
	Inputs  []Variable `json:"inputs"`
	Name    string     `json:"name"`
	Outputs []Variable `json:"outputs"`
}

// Constructor represents an ABI field of type constructor
type Constructor struct {
	Function
}

// L1Handler represents an ABI field of type l1_handler
type L1Handler struct {
	Function
}

// Event represents an ABI field of type event
type Event struct {
	FieldCommon
	Data []Variable `json:"data"`
	Keys []string   `json:"keys"`
	Name string     `json:"name"`
}

// StructMember represents a member of the ABI Struct field
type StructMember struct {
	Variable
	Offset int64 `json:"offset"`
}

// Struct represents an ABI field of type Struct
type Struct struct {
	FieldCommon
	Members []StructMember `json:"members"`
	Name    string         `json:"name"`
	Size    int64          `json:"size"`
}
