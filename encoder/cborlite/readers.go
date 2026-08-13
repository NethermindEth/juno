package cborlite

import (
	"encoding"
	"fmt"
	"math/big"
	"reflect"
)

var (
	prefixUnmarshalerType = reflect.TypeFor[PrefixUnmarshaler]()
	binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()

	// A *big.Int looks like a normal number until it outgrows a uint64.
	// We need to check the type explicitly to match it to the correct reader.
	bigIntType = reflect.TypeFor[*big.Int]()
)

// specialTypeReader covers everything that can't be resolved through its kind.
func specialTypeReader(valueType reflect.Type) (reader, bool) {
	// Implements UnmarshalCBORPrefix
	if reflect.PointerTo(valueType).Implements(prefixUnmarshalerType) {
		return readPrefixUnmarshaler, true
	}

	// Implements UnmarshalBinary
	if reflect.PointerTo(valueType).Implements(binaryUnmarshalerType) {
		return readBinaryUnmarshaler, true
	}

	if valueType == bigIntType {
		return readBigInt, true
	}

	return nil, false
}

func readPrefixUnmarshaler(target reflect.Value, data []byte) (consumed int, err error) {
	unmarshaler, ok := target.Addr().Interface().(PrefixUnmarshaler)
	if !ok {
		return 0, errShape
	}

	consumed, err = unmarshaler.UnmarshalCBORPrefix(data)
	if err != nil {
		return 0, err
	}

	if consumed <= 0 || consumed > len(data) {
		return 0, fmt.Errorf("%T reported consuming %d of %d bytes",
			unmarshaler, consumed, len(data))
	}
	return consumed, nil
}

func readBinaryUnmarshaler(target reflect.Value, data []byte) (consumed int, err error) {
	bytes, consumed, ok := BytesNoCopy(data)
	if !ok {
		return 0, errShape
	}

	unmarshaler, ok := target.Addr().Interface().(encoding.BinaryUnmarshaler)
	if !ok {
		return 0, errShape
	}
	if err := unmarshaler.UnmarshalBinary(bytes); err != nil {
		return 0, fmt.Errorf("%T: %w", unmarshaler, err)
	}

	return consumed, nil
}

func readBigInt(target reflect.Value, data []byte) (consumed int, err error) {
	if consumed, isNull := readNullInto(target, data); isNull {
		return consumed, nil
	}

	value, consumed, ok := BigInt(data)
	if !ok {
		return 0, errShape
	}

	target.Set(reflect.ValueOf(value))
	return consumed, nil
}
