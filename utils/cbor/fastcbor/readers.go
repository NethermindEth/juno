package fastcbor

import (
	"math"
	"math/big"
	"reflect"
	"sync"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
)

// A rootReader reads one item from the start of data into the value at dst, which must be the zero
// value of the type it was planned for, and returns the bytes after the item.
type rootReader = func(data []byte, dst unsafe.Pointer) (rest []byte, err error)

// feltEncoded are the types that RegisterFelt declared as written as a felt, or a slice of felts.
var feltEncoded = map[reflect.Type]bool{}

// special are the types that a reader written for them reads, before any rule applies.
var special = map[reflect.Type]fieldReader{
	reflect.TypeFor[[]int](): asFieldReader(readInts),
}

// isSpecial reports whether a reader written for t reads it.
func isSpecial(t reflect.Type) bool {
	_, ok := specialReader(t)
	return ok
}

var (
	shared = &plans{structs: map[reflect.Type]*structPlan{}}
	// readers has the rootReader of each type asked for, or nil if there is none.
	readers sync.Map // reflect.Type -> *entry
	mu      sync.Mutex
)

type entry struct {
	read rootReader
	err  error
}

// readerFor returns the rootReader of t, planning it the first time. It returns nil if t cannot be
// planned, and then err says why.
func readerFor(t reflect.Type) (read rootReader, err error) {
	if e, ok := readers.Load(t); ok {
		return e.(*entry).read, e.(*entry).err
	}
	mu.Lock()
	defer mu.Unlock()
	if e, ok := readers.Load(t); ok {
		return e.(*entry).read, e.(*entry).err
	}
	read, err = shared.reader(t)
	readers.Store(t, &entry{read: read, err: err})
	return read, err
}

func readBigInt(r *cursor.Cursor, dst *big.Int) error {
	if r.Null() {
		return nil
	}
	value, err := r.BigInt()
	if err != nil {
		return err
	}
	*dst = *value
	return nil
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
		// Stored offsets are unsigned. A null leaves the element zero, and a negative integer is read as
		// signed.
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

// reader returns the reader of the type t, or why there is none.
func (s *plans) reader(t reflect.Type) (rootReader, error) {
	read, p, err := s.planner(t)
	if err != nil {
		return nil, err
	}
	if structPlan := p.structs[t]; structPlan != nil {
		// A struct calls its own plan here, with no indirect call.
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

// borrow returns a pooled Cursor positioned at the start of data. It and giveBack are functions of
// their own, as NewCursor and Remaining do not inline into the closures of the readers.
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
