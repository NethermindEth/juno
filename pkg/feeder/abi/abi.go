package feeder

import (
	"encoding/json"
	"fmt"
)

const (
	TypeFunction    = "function"
	TypeEvent       = "event"
	TypeStruct      = "struct"
	TypeConstructor = "constructor"
	TypeL1Handler   = "l1_handler"
)

// Unmarshals JSON into abi object
func (abi *Abi) UnmarshalAbiJSON(data []byte) error {
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
	// abi := new(Abi)
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
	return nil
}
