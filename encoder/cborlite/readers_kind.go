package cborlite

import (
	"fmt"
	"reflect"
)

// maxPresizedBytes caps the allocation a count is trusted for up front.
const maxPresizedBytes = 1 << 20

var byteType = reflect.TypeFor[byte]()

// kindReader maps a Go kind onto a reader, for what specialTypeReader did not claim.
func kindReader(valueType reflect.Type, strict bool) (reader, error) {
	if scalar := scalarReader(valueType.Kind()); scalar != nil {
		return scalar, nil
	}

	switch valueType.Kind() {
	case reflect.Struct:
		structPlan, err := planFor(valueType, strict)
		if err != nil {
			return nil, err
		}
		return structPlan.reader, nil

	case reflect.Pointer:
		return pointerReader(valueType, strict)

	case reflect.Slice:
		return sliceReader(valueType, strict)

	case reflect.Array:
		return arrayReader(valueType, strict)

	case reflect.Map:
		return mapReader(valueType, strict)

	case reflect.Interface:
		return interfaceReader(valueType, strict), nil

	default:
		return nil, fmt.Errorf("no reader for %s (kind %s)", valueType, valueType.Kind())
	}
}

// scalarReader covers the kinds that need nothing but the kind itself.
// Keeping them inlined for performance reasons.
func scalarReader(kind reflect.Kind) reader {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(target reflect.Value, data []byte) (int, error) {
			value, consumed, ok := Int64(data)
			if !ok || target.OverflowInt(value) {
				return 0, errShape
			}
			target.SetInt(value)
			return consumed, nil
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(target reflect.Value, data []byte) (int, error) {
			value, consumed, ok := Uint64(data)
			if !ok || target.OverflowUint(value) {
				return 0, errShape
			}
			target.SetUint(value)
			return consumed, nil
		}

	case reflect.String:
		return func(target reflect.Value, data []byte) (int, error) {
			value, consumed, ok := String(data)
			if !ok {
				return 0, errShape
			}
			target.SetString(value)
			return consumed, nil
		}

	case reflect.Bool:
		return func(target reflect.Value, data []byte) (int, error) {
			value, consumed, ok := Bool(data)
			if !ok {
				return 0, errShape
			}
			target.SetBool(value)
			return consumed, nil
		}

	default:
		return nil
	}
}

// readNullInto writes the zero value when data is null, reporting whether it did.
func readNullInto(target reflect.Value, data []byte) (consumed int, isNull bool) {
	if consumed, isNull = ReadNull(data); isNull {
		target.SetZero()
	}
	return consumed, isNull
}

func pointerReader(pointerType reflect.Type, strict bool) (reader, error) {
	elem := pointerType.Elem()

	elemReader, err := buildReader(elem, strict)
	if err != nil {
		return nil, err
	}

	return func(target reflect.Value, data []byte) (int, error) {
		if consumed, isNull := readNullInto(target, data); isNull {
			return consumed, nil
		}

		if target.IsNil() {
			target.Set(reflect.New(elem))
		}

		return elemReader(target.Elem(), data)
	}, nil
}

// readByteSlice fills a byte slice.
// Any slice of uint8 lands here, so a named form like json.RawMessage does too.
func readByteSlice(target reflect.Value, data []byte) (consumed int, err error) {
	if consumed, isNull := readNullInto(target, data); isNull {
		return consumed, nil
	}

	value, consumed, ok := Bytes(data)
	if !ok {
		return 0, errShape
	}

	target.SetBytes(value)
	return consumed, nil
}

// readByteArray fills a fixed-size byte array.
// The length has to match exactly, since an array cannot grow or shrink to fit.
func readByteArray(arrayType reflect.Type) reader {
	length := arrayType.Len()
	elemIsByte := arrayType.Elem() == byteType

	return func(target reflect.Value, data []byte) (int, error) {
		value, consumed, ok := BytesNoCopy(data)
		if !ok || len(value) != length {
			return 0, errShape
		}

		if elemIsByte {
			reflect.Copy(target, reflect.ValueOf(value))
			return consumed, nil
		}

		// reflect.Copy needs the element types to be the same. Same kind is not enough.
		// So setting one by one
		for index, b := range value {
			target.Index(index).SetUint(uint64(b))
		}
		return consumed, nil
	}
}

func sliceReader(sliceType reflect.Type, strict bool) (reader, error) {
	// A []byte is a byte string, the same as a [N]byte, not an array of small ints.
	if sliceType.Elem().Kind() == reflect.Uint8 {
		return readByteSlice, nil
	}

	elemReader, err := buildReader(sliceType.Elem(), strict)
	if err != nil {
		return nil, err
	}

	zero := reflect.Zero(sliceType.Elem())

	// One wire byte can become a whole element, so the count alone is not a safe size.
	maxElements := maxPresizedBytes / max(1, int(sliceType.Elem().Size()))

	return func(target reflect.Value, data []byte) (int, error) {
		if consumed, isNull := readNullInto(target, data); isNull {
			return consumed, nil
		}

		length, consumed, ok := ArrayHeader(data)
		if !ok {
			return 0, errShape
		}

		presized := min(length, maxElements)
		slice := reflect.MakeSlice(sliceType, presized, presized)

		for index := range length {
			if index >= presized {
				slice = reflect.Append(slice, zero)
			}

			used, err := elemReader(slice.Index(index), data[consumed:])
			if err != nil {
				return 0, fmt.Errorf("[%d]: %w", index, err)
			}
			consumed += used
		}

		target.Set(slice)
		return consumed, nil
	}, nil
}

func arrayReader(arrayType reflect.Type, strict bool) (reader, error) {
	// A [N]byte is the same as a []byte, not an array of small ints.
	if arrayType.Elem().Kind() == reflect.Uint8 {
		return readByteArray(arrayType), nil
	}

	length := arrayType.Len()

	elemReader, err := buildReader(arrayType.Elem(), strict)
	if err != nil {
		return nil, err
	}

	return func(target reflect.Value, data []byte) (int, error) {
		count, consumed, ok := ArrayHeader(data)
		// A fixed-size array has to match exactly
		if !ok || count != length {
			return 0, errShape
		}

		for index := range length {
			used, err := elemReader(target.Index(index), data[consumed:])
			if err != nil {
				return 0, fmt.Errorf("[%d]: %w", index, err)
			}
			consumed += used
		}
		return consumed, nil
	}, nil
}

func mapReader(mapType reflect.Type, strict bool) (reader, error) {
	keyReader, err := buildReader(mapType.Key(), strict)
	if err != nil {
		return nil, err
	}
	valueReader, err := buildReader(mapType.Elem(), strict)
	if err != nil {
		return nil, err
	}

	maxEntries := maxPresizedBytes / max(1, int(mapType.Key().Size()+mapType.Elem().Size()))

	return func(target reflect.Value, data []byte) (int, error) {
		if consumed, isNull := readNullInto(target, data); isNull {
			return consumed, nil
		}

		pairs, consumed, ok := MapHeader(data)
		if !ok {
			return 0, errShape
		}

		out := reflect.MakeMapWithSize(mapType, min(pairs, maxEntries))

		key := reflect.New(mapType.Key()).Elem()
		value := reflect.New(mapType.Elem()).Elem()

		for range pairs {
			key.SetZero()
			value.SetZero()

			used, err := keyReader(key, data[consumed:])
			if err != nil {
				return 0, fmt.Errorf("key: %w", err)
			}
			consumed += used

			used, err = valueReader(value, data[consumed:])
			if err != nil {
				return 0, fmt.Errorf("value: %w", err)
			}
			consumed += used

			out.SetMapIndex(key, value)
		}

		// A repeated key means the map holds fewer entries than the header counted.
		if out.Len() != pairs {
			return 0, errShape
		}

		target.Set(out)
		return consumed, nil
	}, nil
}

func interfaceReader(interfaceType reflect.Type, strict bool) reader {
	return func(target reflect.Value, data []byte) (int, error) {
		if consumed, isNull := readNullInto(target, data); isNull {
			return consumed, nil
		}

		tag, consumed, ok := Tag(data)
		if !ok {
			return 0, errShape
		}

		concrete, known := tagTypes.Load(tag)
		if !known {
			return 0, errShape
		}
		concreteType := concrete.(reflect.Type)
		if !reflect.PointerTo(concreteType).Implements(interfaceType) {
			return 0, errShape
		}

		read, err := cachedReader(concreteType, strict)
		if err != nil {
			return 0, fmt.Errorf("tag %d (%s): %w", tag, concreteType, err)
		}

		value := reflect.New(concreteType)
		used, err := read(value.Elem(), data[consumed:])
		if err != nil {
			return 0, fmt.Errorf("tag %d (%s): %w", tag, concreteType, err)
		}

		target.Set(value)
		return consumed + used, nil
	}
}
