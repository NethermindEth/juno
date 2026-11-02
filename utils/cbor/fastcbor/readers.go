package fastcbor

import (
	"math"
	"math/big"
	"reflect"
	"sync"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
)

// A Reader reads one item from the start of data into the value at dst, which must be the zero
// value of the type it was planned for, and returns the bytes after the item.
type Reader = func(data []byte, dst unsafe.Pointer) (rest []byte, err error)

// feltEncoded are the types with their own CBOR encoding that write what fxamacker writes for a
// felt, or for a slice of felts, so the planner reads them as one. Other felt-like types are found
// by probing what fxamacker writes for them.
var feltEncoded = map[reflect.Type]bool{
	reflect.TypeFor[felt.Felt]():             true,
	reflect.TypeFor[felt.Slice[felt.Felt]](): true,
}

// newGenericPlans makes the planner: the generic rules, and the readers of the maps whose keys and
// values are types of the felt package.
func newGenericPlans() *plans {
	return newPlans(overrides{generic: true, types: map[reflect.Type]func(*planner) fieldReader{
		reflect.TypeFor[map[felt.Felt]*felt.Felt]():                    leaf(readFeltMap),
		reflect.TypeFor[map[felt.Felt]map[felt.Felt]*felt.Felt]():      leaf(readStorageDiffs),
		reflect.TypeFor[map[felt.SierraClassHash]felt.CasmClassHash](): leaf(readMigratedClasses),
		reflect.TypeFor[[]int]():                                       leaf(readInts),
	}})
}

var (
	generic = newGenericPlans()
	// readers has the Reader of each type asked for, or nil if there is none.
	readers sync.Map // reflect.Type -> *entry
	mu      sync.Mutex
)

type entry struct {
	read Reader
	err  error
}

// readerFor returns the Reader of t, planning it the first time it is asked for. It returns nil if
// t cannot be planned, and then err is why, on the first call only.
func readerFor(t reflect.Type) (read Reader, err error) {
	if e, ok := readers.Load(t); ok {
		return e.(*entry).read, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if e, ok := readers.Load(t); ok {
		return e.(*entry).read, nil
	}
	read, err = generic.reader(t)
	readers.Store(t, &entry{read: read, err: err})
	return read, err
}

// leaf makes a reader that needs no plan an override.
func leaf[T any](read func(*cursor.Cursor, *T) error) func(*planner) fieldReader {
	return func(*planner) fieldReader { return asFieldReader(read) }
}

func readBigIntPtr(r *cursor.Cursor, dst **big.Int) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	value, err := r.BigInt()
	if err != nil {
		return err
	}
	*dst = value
	return nil
}

// readInts reads an array of integers, or null as nil. A null element leaves it zero.
func readInts(c *cursor.Cursor, dst *[]int) error {
	if c.Null() {
		*dst = nil
		return nil
	}
	count, err := c.ArrayLen()
	if err != nil {
		return err
	}
	out := make([]int, count)
	for i := range out {
		// Stored offsets are unsigned; a null or a negative integer takes the second try.
		value, err := c.Uint64()
		if err != nil {
			if c.Null() {
				continue
			}
			signed, err := c.Int64()
			if err != nil {
				return err
			}
			out[i] = int(signed)
			continue
		}
		if value > math.MaxInt64 {
			return cursor.ErrType
		}
		out[i] = int(value)
	}
	c.Leave()
	*dst = out
	return nil
}

// reader returns the reader of the type t, or why there is none, which leaves t to fxamacker.
func (s *plans) reader(t reflect.Type) (Reader, error) {
	read, p, err := s.planner(t)
	if err != nil {
		return nil, err
	}
	if structPlan := p.structs[t]; structPlan != nil {
		// The plan of a struct is called directly, not through its method value.
		return func(data []byte, dst unsafe.Pointer) ([]byte, error) {
			c := borrow(data)
			err := structPlan.read(c, dst)
			rest := giveBack(c)
			if err != nil {
				return nil, err
			}
			return rest, nil
		}, nil
	}
	return func(data []byte, dst unsafe.Pointer) ([]byte, error) {
		c := borrow(data)
		err := read(c, dst)
		rest := giveBack(c)
		if err != nil {
			return nil, err
		}
		return rest, nil
	}, nil
}

// cursorPool keeps Cursors, which would otherwise escape to the heap on every decode.
var cursorPool = sync.Pool{New: func() any { return new(cursor.Cursor) }}

// borrow returns a pooled Cursor positioned at the start of data. These two are not in the readers
// because the compiler did not inline NewCursor and Remaining into their closures, which cost a
// few nanoseconds in each decode.
func borrow(data []byte) *cursor.Cursor {
	c := cursorPool.Get().(*cursor.Cursor)
	*c = cursor.NewCursor(data)
	return c
}

// giveBack returns what c has not read, and puts c back in the pool.
func giveBack(c *cursor.Cursor) []byte {
	rest := c.Remaining()
	*c = cursor.Cursor{} // the pool must not hold on to data
	cursorPool.Put(c)
	return rest
}
