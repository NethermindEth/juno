// Package cborlite is a CBOR unmarshaler. It has two main goals:
//
//   - To only see a byte once. No revisiting.
//   - To fill the fields you ask for and skip the rest.
//
// Skipping is cheap. No hidden byte checks. Less CPU usage.
//
// No reader checks null, since that would be a hidden byte check on every call. A caller
// that can meet null calls [ReadNull] first.
//
// This unmarshaler is not exhaustive. It declines a shape it does not know.
// It throws [ErrShape] and [ErrUnsupportedType], anything else is a defect.
// A caller can fall back to the generic decoder. Add support when a shape needs it.
// Do not use it for external inputs.
//
// To unmarshal a new type, implement [PrefixUnmarshaler].
// Use the readers in headers.go and values.go to help.
package cborlite

import (
	"errors"
	"fmt"
	"reflect"
)

// PrefixUnmarshaler is the interface a type uses to unmarshal itself from a buffer front.
// It must copy what it keeps, since data is freed after reading.
type PrefixUnmarshaler interface {
	UnmarshalCBORPrefix(data []byte) (consumed int, err error)
}

// ErrShape means these bytes are not wellformed.
var ErrShape = errors.New("shape does not match")

// ErrUnsupportedType means this Go type is not supported by cborlite.
var ErrUnsupportedType = errors.New("unsupported type")

// Unmarshal decodes data into target.
func Unmarshal(data []byte, target any) error {
	return unmarshalAll(data, target, false)
}

// UnmarshalPrefix decodes the value at the start of data and reports its length.
func UnmarshalPrefix(data []byte, target any) (consumed int, err error) {
	return unmarshalPrefix(data, target, false)
}

// UnmarshalStrict decodes like [Unmarshal] but fails if any key would be skipped.
func UnmarshalStrict(data []byte, target any) error {
	return unmarshalAll(data, target, true)
}

func unmarshalAll(data []byte, target any, strict bool) error {
	consumed, err := unmarshalPrefix(data, target, strict)
	if err != nil {
		return err
	}
	if consumed != len(data) {
		return fmt.Errorf("cborlite: %T: %d bytes left over", target, len(data)-consumed)
	}

	return nil
}

func unmarshalPrefix(data []byte, target any, strict bool) (consumed int, err error) {
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return 0, fmt.Errorf("cborlite: needs a non-nil pointer, got %T", target)
	}

	valueType := value.Type().Elem()

	reader, err := cachedReader(valueType, strict)
	if err != nil {
		return 0, err
	}

	consumed, err = reader(value.Elem(), data)
	if err != nil {
		return 0, fmt.Errorf("cborlite: %w", err)
	}

	return consumed, nil
}
