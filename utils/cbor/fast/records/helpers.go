package records

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// readFelt reads a felt. Null leaves the pointer nil.
func readFelt(r *fast.Reader, dst **felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(felt.Felt)
	if err := r.Limbs((*[fast.Limbs]uint64)(value)); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readFeltSlice reads an array of felts, or null as nil.
func readFeltSlice(r *fast.Reader, dst *felt.Slice[felt.Felt]) error {
	value, err := fast.LimbSlice[felt.Felt](r)
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint64(r *fast.Reader, dst *uint64) error {
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint32[T ~uint32](r *fast.Reader, dst *T) error {
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > 1<<32-1 {
		return fast.ErrType
	}
	*dst = T(value)
	return nil
}

func readByte(r *fast.Reader, dst *byte) error {
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	if value > 255 {
		return fast.ErrType
	}
	*dst = byte(value)
	return nil
}

func readBool(r *fast.Reader, dst *bool) error {
	value, err := r.Bool()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readString(r *fast.Reader, dst *string) error {
	value, err := r.String()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// readRaw reads a byte string into a byte slice, or null as nil.
func readRaw(r *fast.Reader, dst *[]byte) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value, err := r.Bytes()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// readPtr reads a struct behind a pointer. Null leaves the pointer nil.
func readPtr[T any](r *fast.Reader, dst **T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(T)
	if err := read(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readSlice reads an array of structs, or null as nil.
func readSlice[T any](r *fast.Reader, dst *[]T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]T, count)
	for i := range out {
		if err := read(r, &out[i]); err != nil {
			return err
		}
	}
	*dst = out
	return nil
}

// readPtrSlice reads an array of struct pointers, or null as nil. An element may be null.
func readPtrSlice[T any](r *fast.Reader, dst *[]*T, read func(*fast.Reader, *T) error) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]*T, count)
	for i := range out {
		if err := readPtr(r, &out[i], read); err != nil {
			return err
		}
	}
	*dst = out
	return nil
}

func readStringSlice(r *fast.Reader, dst *[]string) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]string, count)
	for i := range out {
		if err := readString(r, &out[i]); err != nil {
			return err
		}
	}
	*dst = out
	return nil
}

// readFields walks a struct's map, giving field the key of each entry to read. An unknown key
// is skipped. Null leaves the struct as it is.
func readFields(r *fast.Reader, field func(key []byte) (known bool, err error)) error {
	if r.Null() {
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	for ; count > 0; count-- {
		key, err := r.KeyNoCopy()
		if err != nil {
			return err
		}
		known, err := field(key)
		if err != nil {
			return err
		}
		if !known {
			if err := r.Skip(); err != nil {
				return err
			}
		}
	}
	return nil
}
