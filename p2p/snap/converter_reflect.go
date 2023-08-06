package snap

import (
	"github.com/NethermindEth/juno/core/trie"
	"reflect"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/snap/p2pproto"
)

var typeMapper = map[string]map[string]func(interface{}) interface{}{}

func registerMapping[T1 any, T2 any](f func(T1) T2) {
	var t1 *T1
	var t2 *T2

	tp1 := reflect.TypeOf(t1).Elem()
	tp2 := reflect.TypeOf(t2).Elem()

	to, ok := typeMapper[tp1.String()]
	if !ok {
		typeMapper[tp1.String()] = map[string]func(interface{}) interface{}{}
		to = typeMapper[tp1.String()]
	}

	to[tp2.String()] = func(in interface{}) interface{} {
		inc := in.(T1)
		out := f(inc)
		return out
	}
}

//nolint:all
func init() {
	registerMapping[*felt.Felt, *p2pproto.FieldElement](feltToFieldElement)
	registerMapping[*p2pproto.FieldElement, *felt.Felt](fieldElementToFelt)
	registerMapping[*trie.Key, *p2pproto.Path](bitsetToProto)
	registerMapping[*p2pproto.Path, *trie.Key](protoToBitset)
}

func MapValueViaReflect[T any](source interface{}) T {
	var destination T
	destinationPtr := &destination
	reflectMapValue(reflect.ValueOf(source), reflect.ValueOf(destinationPtr))
	return destination
}

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
