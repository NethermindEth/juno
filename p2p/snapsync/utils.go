package snap

import (
	"reflect"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snapsync/p2pproto"
)

func fieldElementsToFelts(fieldElements []*p2pproto.FieldElement) []*felt.Felt {
	felts := make([]*felt.Felt, len(fieldElements))
	for i, fe := range fieldElements {
		felts[i] = fieldElementToFelt(fe)
	}

	return felts
}

func fieldElementToFelt(field *p2pproto.FieldElement) *felt.Felt {
	if field == nil {
		return nil
	}
	thefelt := felt.Zero
	thefelt.SetBytes(field.Elements)
	return &thefelt
}

func MapValueViaReflect[T any](source interface{}) T {
	var destination T
	destinationPtr := &destination
	reflectMapValue(reflect.ValueOf(source), reflect.ValueOf(destinationPtr))
	return destination
}

var typeMapper = map[string]map[string]func(interface{}) interface{}{}

func reflectMapValue(sourceValue, destValue reflect.Value) {
	sourceType := sourceValue.Type()
	destType := destValue.Type()

	nmap, ok := typeMapper[sourceType.String()]
	if ok {
		f, ok := nmap[destType.String()]
		if ok {
			newval := f(sourceValue.Interface())
			destValue.Set(reflect.ValueOf(newval))
			return
		}
	}

	if sourceValue.Kind() == reflect.Ptr && sourceValue.IsNil() {
		if destValue.CanAddr() {
			destValue.Set(reflect.Zero(destType))
		}
		return
	}

	for sourceValue.Kind() == reflect.Ptr {
		sourceValue = reflect.Indirect(sourceValue)
	}
	for destValue.Kind() == reflect.Ptr {
		if destValue.IsNil() {
			nval := reflect.New(destValue.Type().Elem())
			destValue.Set(nval.Elem().Addr())
		}

		destValue = reflect.Indirect(destValue)
	}

	if sourceType.Kind() == reflect.Slice {
		mapArray(sourceValue, destValue)
		return
	}

	if sourceValue.Kind() != destValue.Kind() {
		panic("Both source and destination must have same kind")
	}

	switch sourceValue.Kind() {
	case reflect.Struct:
		mapStruct(sourceValue, destValue)
	case reflect.Slice, reflect.Array:
		mapArray(sourceValue, destValue)
	default:
		destValue.Set(sourceValue)
	}
}

func mapArray(sourceField, destField reflect.Value) {
	sourceLen := sourceField.Len()

	if destField.IsNil() || destField.Len() != sourceLen {
		destField.Set(reflect.MakeSlice(destField.Type(), sourceLen, sourceLen))
	}

	destLen := destField.Len()

	if sourceLen != destLen {
		panic("Source and destination array lengths do not match")
	}

	for i := 0; i < sourceLen; i++ {
		sourceElem := sourceField.Index(i)
		destElem := destField.Index(i)

		reflectMapValue(sourceElem, destElem) // Recursively map nested structs within the array
	}
}

func mapStruct(sourceValue, destValue reflect.Value) {
	sourceType := sourceValue.Type()
	numFields := sourceValue.NumField()

	for i := 0; i < numFields; i++ {
		sourceFieldType := sourceType.Field(i)
		sourceField := sourceValue.Field(i)
		destField := destValue.FieldByName(sourceFieldType.Name)

		if !destField.CanSet() {
			continue // Skip unexported fields
		}

		reflectMapValue(sourceField, destField)
	}
}

func feltToFieldElement(flt *felt.Felt) *p2pproto.FieldElement {
	if flt == nil {
		return nil
	}
	return &p2pproto.FieldElement{Elements: flt.Marshal()}
}

func feltsToFieldElements(felts []*felt.Felt) []*p2pproto.FieldElement {
	return toProtoMapArray(felts, feltToFieldElement)
}

func toProtoMapArray[F any, T any](from []F, mapper func(F) T) (to []T) {
	if len(from) == 0 {
		// protobuf does not distinguish between nil array or empty array. But we put it here for testing reason
		// as when deserializing it always deserialize empty array as nil
		return nil
	}

	toArray := make([]T, len(from))

	for i, f := range from {
		toArray[i] = mapper(f)
	}

	return toArray
}
