package core

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/bits-and-blooms/bitset"
	bloom "github.com/bits-and-blooms/bloom/v3"
)

// The projections are read by the fast decoder: only the fields a projection names with a typed
// field are decoded, every other value is skipped with its syntax checked. The readers follow
// fxamacker's struct rules, which the projections rely on: a key names a field exactly or, failing
// that, ignoring ASCII case; the first of two keys naming one field wins; integer keys and unknown
// keys are skipped; null leaves a plain field as it was and a pointer nil.
//
// The readers need no type tags, so they register here rather than in utils/cbor/registry, which
// imports this package. Init runs before any decode.
//
//nolint:gochecknoinits
func init() {
	cbor.RegisterReader(readHeaderHashProjection)
	cbor.RegisterReader(readHeaderGlobalStateRootProjection)
	cbor.RegisterReader(readHeaderTransactionCountProjection)
	cbor.RegisterReader(readHeaderTimestampProjection)
	cbor.RegisterReader(readHeaderEventsBloomProjection)
	cbor.RegisterReader(readHeaderHashAndStateRootProjection)
	cbor.RegisterReader(readReceiptExecutionStatusProjection)
	cbor.RegisterReader(readReceiptEventsProjection)
	cbor.RegisterReader(readTransactionHashProjection)
	cbor.RegisterReader(readBlockTransactionsIndexes)
}

// walkFields reads a struct's map and calls read with the index in fields of each field a key
// names, the first time it is named. Every other value is skipped. Null leaves the struct as it is.
func walkFields(r *fast.Reader, fields []string, read func(field int) error) error {
	if r.Null() {
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	var seen uint64
	all := uint64(1)<<len(fields) - 1
	for ; count > 0; count-- {
		key, isText, err := r.Key()
		if err != nil {
			return err
		}
		field := -1
		if isText && seen != all { // once every field is read, the rest only needs skipping
			field = fieldIndex(fields, key)
		}
		if field < 0 || seen&(1<<field) != 0 {
			if err := r.Skip(); err != nil {
				return err
			}
			continue
		}
		seen |= 1 << field
		if err := read(field); err != nil {
			return err
		}
	}
	r.Leave()
	return nil
}

// fieldIndex returns the index of the field key names, or -1. The names of a stored struct's
// fields differ in more than case, so a key that names a skipped field exactly cannot name a wanted
// one ignoring case, and the wanted fields are the only ones to look for.
func fieldIndex(fields []string, key []byte) int {
	for i, name := range fields {
		if string(key) == name {
			return i
		}
	}
	// fxamacker compares ignoring case only names of the key's byte length, where Unicode folding
	// cannot reach an ASCII name.
	for i, name := range fields {
		if len(key) == len(name) && equalFoldASCII(key, name) {
			return i
		}
	}
	return -1
}

// equalFoldASCII compares a key with an ASCII name of the same length, ignoring case.
func equalFoldASCII(key []byte, name string) bool {
	for i := range len(name) {
		a, b := key[i], name[i]
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}

// readFeltPtr reads a felt behind a pointer. Null leaves the pointer nil.
func readFeltPtr(r *fast.Reader, dst **felt.Felt) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value := new(felt.Felt)
	if err := fast.Felt(r, value); err != nil {
		return err
	}
	*dst = value
	return nil
}

// readFeltValue reads a felt in place. Null leaves it as it was.
func readFeltValue(r *fast.Reader, dst *felt.Felt) error {
	if r.Null() {
		return nil
	}
	return fast.Felt(r, dst)
}

func readUint64Value(r *fast.Reader, dst *uint64) error {
	if r.Null() {
		return nil
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

func readUint64Ptr(r *fast.Reader, dst **uint64) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value, err := r.Uint64()
	if err != nil {
		return err
	}
	*dst = &value
	return nil
}

// errBloomSize means a stored bloom filter declares more bits than its bytes hold.
var errBloomSize = errors.New("core: bloom filter is shorter than it declares")

// readBloomPtr reads a bloom filter, stored as its binary form in a byte string: m, k and the bit
// count (8 bytes each), then the words of the bit set, big-endian. The bloom package allocates the
// bit count the form declares before it reads the words, so a count the bytes cannot hold is
// rejected first. The words are read here rather than through the package's stream reader, which
// costs more than the rest of the header; bytes after them are ignored, as the package does.
func readBloomPtr(r *fast.Reader, dst **bloom.BloomFilter) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	data, err := r.BytesNoCopy()
	if err != nil {
		return err
	}
	const header = 3 * 8
	if len(data) < header {
		return errBloomSize
	}
	words := data[header:]
	bits := binary.BigEndian.Uint64(data[2*8:])
	if bits > uint64(len(words))*8 {
		return errBloomSize
	}
	count := (bits + 63) / 64
	if count*8 > uint64(len(words)) {
		return errBloomSize
	}
	// A bit count that is not a whole number of words, or a bit set package set to another byte
	// order, takes the package's own reader. UnmarshalBinary copies what it keeps.
	if bits != count*64 || bitset.BinaryOrder() != binary.BigEndian {
		filter := new(bloom.BloomFilter)
		if err := filter.UnmarshalBinary(data); err != nil {
			return err
		}
		*dst = filter
		return nil
	}
	var set []uint64 // the package leaves a filter of no words with a nil slice
	if count != 0 {
		set = make([]uint64, count)
	}
	for i := range set {
		set[i] = binary.BigEndian.Uint64(words[8*i:])
	}
	m, k := binary.BigEndian.Uint64(data), binary.BigEndian.Uint64(data[8:])
	*dst = bloom.FromWithM(set, uint(m), uint(k))
	return nil
}

// The wire names of the fields each reader decodes, in the order its read callback numbers them.
var (
	hashField              = []string{"Hash"}
	globalStateRootField   = []string{"GlobalStateRoot"}
	transactionCountField  = []string{"TransactionCount"}
	timestampField         = []string{"Timestamp"}
	eventsBloomField       = []string{"EventsBloom"}
	hashAndStateRootFields = []string{"Hash", "GlobalStateRoot"}
	executionStatusFields  = []string{"Reverted", "RevertReason"}
	eventsFields           = []string{"Events", "TransactionHash"}
	eventFields            = []string{"From", "Keys", "Data"}
	transactionHashField   = []string{"TransactionHash"}
)

func readHeaderHashProjection(r *fast.Reader, dst *headerHashProjection) error {
	return walkFields(r, hashField, func(int) error {
		return readFeltPtr(r, &dst.Hash)
	})
}

func readHeaderGlobalStateRootProjection(r *fast.Reader, dst *headerGlobalStateRootProjection) error {
	return walkFields(r, globalStateRootField, func(int) error {
		return readFeltPtr(r, &dst.GlobalStateRoot)
	})
}

func readHeaderTransactionCountProjection(
	r *fast.Reader,
	dst *headerTransactionCountProjection,
) error {
	return walkFields(r, transactionCountField, func(int) error {
		return readUint64Value(r, &dst.TransactionCount)
	})
}

func readHeaderTimestampProjection(r *fast.Reader, dst *headerTimestampProjection) error {
	return walkFields(r, timestampField, func(int) error {
		return readUint64Ptr(r, &dst.Timestamp)
	})
}

func readHeaderEventsBloomProjection(r *fast.Reader, dst *headerEventsBloomProjection) error {
	return walkFields(r, eventsBloomField, func(int) error {
		return readBloomPtr(r, &dst.EventsBloom)
	})
}

func readHeaderHashAndStateRootProjection(
	r *fast.Reader,
	dst *headerHashAndStateRootProjection,
) error {
	return walkFields(r, hashAndStateRootFields, func(field int) error {
		if field == 0 {
			return readFeltPtr(r, &dst.Hash)
		}
		return readFeltPtr(r, &dst.GlobalStateRoot)
	})
}

func readReceiptExecutionStatusProjection(
	r *fast.Reader,
	dst *receiptExecutionStatusProjection,
) error {
	return walkFields(r, executionStatusFields, func(field int) error {
		if r.Null() {
			return nil
		}
		if field == 0 {
			value, err := r.Bool()
			dst.Reverted = value
			return err
		}
		value, err := r.String()
		dst.RevertReason = value
		return err
	})
}

func readReceiptEventsProjection(r *fast.Reader, dst *receiptEventsProjection) error {
	return walkFields(r, eventsFields, func(field int) error {
		if field == 0 {
			return readEvents(r, &dst.Events)
		}
		return readFeltPtr(r, &dst.TransactionHash)
	})
}

// readEvents reads an array of events, or null as nil. An element may be null.
func readEvents(r *fast.Reader, dst *[]*Event) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	events := make([]*Event, count)
	for i := range events {
		if r.Null() {
			continue
		}
		event := new(Event)
		if err := readEvent(r, event); err != nil {
			return err
		}
		events[i] = event
	}
	r.Leave()
	*dst = events
	return nil
}

func readEvent(r *fast.Reader, dst *Event) error {
	return walkFields(r, eventFields, func(field int) error {
		if field == 0 {
			return readFeltPtr(r, &dst.From)
		}
		value, err := fast.LimbSlice[felt.Felt](r)
		if err != nil {
			return err
		}
		if field == 1 {
			dst.Keys = value
		} else {
			dst.Data = value
		}
		return nil
	})
}

// readTransactionHashProjection reads the hash of a stored transaction. The record carries the
// tag of its transaction type, which fxamacker looks through since the projection has none.
func readTransactionHashProjection(r *fast.Reader, dst *transactionHashProjection) error {
	if head, err := r.Peek(); err == nil && head&0xe0 == fast.MajorTag {
		// Tags 0-3 hold a time or a bignum, which fxamacker reads differently; so does a chain.
		tag, err := r.Tag()
		if err != nil {
			return err
		}
		if next, err := r.Peek(); tag <= 3 || (err == nil && next&0xe0 == fast.MajorTag) {
			return fast.ErrUnsupported
		}
	}
	return walkFields(r, transactionHashField, func(int) error {
		return readFeltValue(r, &dst.TransactionHash)
	})
}

// readBlockTransactionsIndexes reads the offsets heading a block's transactions and receipts,
// which every partial read of them decodes first. Its fields are keyed by the integers 1 and 2,
// which fxamacker matches only to integer keys; a text key names no field.
func readBlockTransactionsIndexes(r *fast.Reader, dst *BlockTransactionsIndexes) error {
	if r.Null() {
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	var seen uint64
	for ; count > 0; count-- {
		field, err := readIndexesKey(r)
		if err != nil {
			return err
		}
		if field < 0 || seen&(1<<field) != 0 {
			if err := r.Skip(); err != nil {
				return err
			}
			continue
		}
		seen |= 1 << field
		target := &dst.Transactions
		if field == 2 {
			target = &dst.Receipts
		}
		if err := readInts(r, target); err != nil {
			return err
		}
	}
	r.Leave()
	return nil
}

// readIndexesKey reads a key of BlockTransactionsIndexes and returns the field it names, 1 or 2,
// or -1. fxamacker reads integer keys as int64 and checks text keys are UTF-8.
func readIndexesKey(r *fast.Reader) (int, error) {
	if head, err := r.Peek(); err != nil || head&0xe0 != fast.MajorUint {
		_, _, err := r.Key()
		return -1, err
	}
	key, err := r.Uint64()
	switch {
	case err != nil:
		return -1, err
	case key > math.MaxInt64:
		return -1, fast.ErrType
	case key == 1 || key == 2:
		return int(key), nil
	default:
		return -1, nil
	}
}

// readInts reads an array of integers, or null as nil. A null element leaves it zero.
func readInts(r *fast.Reader, dst *[]int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	count, err := r.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]int, count)
	for i := range out {
		// Stored offsets are unsigned; a null or a negative integer takes the second try.
		value, err := r.Uint64()
		if err != nil {
			if r.Null() {
				continue
			}
			signed, err := r.Int64()
			if err != nil {
				return err
			}
			out[i] = int(signed)
			continue
		}
		if value > math.MaxInt64 {
			return fast.ErrType
		}
		out[i] = int(value)
	}
	r.Leave()
	*dst = out
	return nil
}
