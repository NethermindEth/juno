package p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/grpcclient"
	"github.com/ethereum/go-ethereum/common"
	"reflect"
)

var typeMapper map[string]map[string]func(interface{}) interface{} = map[string]map[string]func(interface{}) interface{}{}

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

func init() {
	registerMapping[*felt.Felt, *grpcclient.FieldElement](feltToFieldElement)
	registerMapping[*grpcclient.FieldElement, *felt.Felt](fieldElementToFelt)
	registerMapping[common.Address, *grpcclient.EthereumAddress](addressToProto)
	registerMapping[*grpcclient.EthereumAddress, common.Address](protoToAddress)
}

func MapStructRet[T any](source interface{}) *T {
	var destination T
	MapStructs(reflect.ValueOf(source), reflect.ValueOf(&destination))
	return &destination
}

func MapStructs(sourceValue reflect.Value, destValue reflect.Value) {
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

	for sourceValue.Kind() == reflect.Ptr {
		sourceValue = reflect.Indirect(sourceValue)
	}
	for destValue.Kind() == reflect.Ptr {
		destValue = reflect.Indirect(destValue)
	}

	if sourceType.Kind() == reflect.Slice {
		mapArray(sourceValue, destValue)
		return
	}

	if sourceValue.Kind() != destValue.Kind() {
		panic("Both source and destination must have same kind")
	}

	sourceType = sourceValue.Type()
	destType = destValue.Type()

	numFields := sourceValue.NumField()

	for i := 0; i < numFields; i++ {
		sourceFieldType := sourceType.Field(i)
		sourceField := sourceValue.Field(i)
		destField := destValue.FieldByName(sourceFieldType.Name)

		if !destField.CanSet() {
			continue // Skip unexported fields
		}

		MapStructs(sourceField, destField)
	}
}

func mapArray(sourceField reflect.Value, destField reflect.Value) {
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

		MapStructs(sourceElem, destElem) // Recursively map nested structs within the array
	}
}

func coreExecutionResourcesToProtobuf(resources *core.ExecutionResources) *grpcclient.CommonTransactionReceiptProperties_ExecutionResources {
	if resources == nil {
		return nil
	}

	return &grpcclient.CommonTransactionReceiptProperties_ExecutionResources{
		BuiltinInstanceCounter: &grpcclient.CommonTransactionReceiptProperties_BuiltinInstanceCounter{
			Bitwise:    resources.BuiltinInstanceCounter.Bitwise,
			EcOp:       resources.BuiltinInstanceCounter.EcOp,
			Ecsda:      resources.BuiltinInstanceCounter.Ecsda,
			Output:     resources.BuiltinInstanceCounter.Output,
			Pedersen:   resources.BuiltinInstanceCounter.Pedersen,
			RangeCheck: resources.BuiltinInstanceCounter.RangeCheck,
		},
		MemoryHoles: resources.MemoryHoles,
		Steps:       resources.Steps,
	}
}

func protobufToCoreExecutionResources(pbResources *grpcclient.CommonTransactionReceiptProperties_ExecutionResources) *core.ExecutionResources {
	if pbResources == nil {
		return nil
	}

	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Bitwise:    pbResources.BuiltinInstanceCounter.Bitwise,
			EcOp:       pbResources.BuiltinInstanceCounter.EcOp,
			Ecsda:      pbResources.BuiltinInstanceCounter.Ecsda,
			Output:     pbResources.BuiltinInstanceCounter.Output,
			Pedersen:   pbResources.BuiltinInstanceCounter.Pedersen,
			RangeCheck: pbResources.BuiltinInstanceCounter.RangeCheck,
		},
		MemoryHoles: pbResources.MemoryHoles,
		Steps:       pbResources.Steps,
	}
}
