package fastcbor

import (
	"bytes"
	"encoding"
	"math"
	"math/big"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

func hasExportedField(t reflect.Type) bool {
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// itemBytes skips the item at the cursor and returns its bytes.
func itemBytes(r *cursor.Cursor) ([]byte, error) {
	before := r.Remaining()
	if err := r.Skip(); err != nil {
		return nil, err
	}
	return before[:len(before)-r.Len()], nil
}

// opaqueReader hands the item to fxamacker. It is for the types that no rule reads, such as
// big.Int, whose fields are not exported.
func opaqueReader(t reflect.Type) fieldReader {
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		item, err := itemBytes(r)
		if err != nil {
			return err
		}
		return fxamacker.Unmarshal(item, reflect.NewAt(t, dst).Interface())
	}
}

// ownEncodingReader reads a type that decodes itself, as fxamacker does: with UnmarshalCBOR, and
// failing that UnmarshalBinary on a byte string.
func (p *planner) ownEncodingReader(t reflect.Type, path string) fieldReader {
	if t.Kind() == reflect.Pointer {
		elem := t.Elem()
		inner := p.ownEncodingReader(elem, path)
		if inner == nil {
			return nil
		}
		return func(r *cursor.Cursor, dst unsafe.Pointer) error {
			if r.Null() {
				*(*unsafe.Pointer)(dst) = nil
				return nil
			}
			v := reflect.New(elem)
			if err := inner(r, v.UnsafePointer()); err != nil {
				return err
			}
			*(*unsafe.Pointer)(dst) = v.UnsafePointer()
			return nil
		}
	}
	ptr := reflect.PointerTo(t)
	switch {
	case ptr.Implements(selfDecoder):
		return func(r *cursor.Cursor, dst unsafe.Pointer) error {
			item, err := itemBytes(r)
			if err != nil {
				return err
			}
			return reflect.NewAt(t, dst).Interface().(fxamacker.SelfDecoder).UnmarshalCBOR(item)
		}
	case ptr.Implements(reflect.TypeFor[encoding.BinaryUnmarshaler]()):
		return func(r *cursor.Cursor, dst unsafe.Pointer) error {
			data, err := r.BytesBorrowed()
			if err != nil {
				return err
			}
			return reflect.NewAt(t, dst).Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(data)
		}
	}
	return p.fail("%s: %v encodes itself but does not decode itself", path, t)
}

// iface is the layout of an interface: the itab, or for an empty interface the type, and the data.
type iface struct{ tab, data unsafe.Pointer }

// itabOf returns the first word of an interface of type t that holds a pointer of type ptr, which
// implements t.
func itabOf(t, ptr reflect.Type) unsafe.Pointer {
	v := reflect.New(t).Elem()
	v.Set(reflect.Zero(ptr))
	return (*iface)(v.Addr().UnsafePointer()).tab
}

// interfaceReader reads an interface stored as the tag of its concrete type and then that type,
// for the types cbor.RegisterType gave a tag that implement it. A read allocates the concrete value
// with the runtime and builds the interface from the itab found when planning, with no reflection.
func (p *planner) interfaceReader(t reflect.Type, path string) fieldReader {
	type variant struct {
		tag  uint64
		typ  *runtimeType
		tab  unsafe.Pointer
		plan *structPlan
	}
	var variants []variant
	for concrete, tag := range fxamacker.RegisteredTypes() {
		ptr := reflect.PointerTo(concrete)
		if concrete.Kind() == reflect.Struct && ptr.Implements(t) {
			variants = append(variants, variant{
				tag: tag, typ: runtimeTypeOf(concrete), tab: itabOf(t, ptr),
				plan: p.structOf(concrete, concrete.String()),
			})
		}
	}
	if len(variants) == 0 {
		return p.fail("%s: no registered type implements %v", path, t)
	}
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*iface)(dst) = iface{}
			return nil
		}
		tag, err := r.Tag()
		if err != nil {
			return err
		}
		for i := range variants {
			v := &variants[i]
			if v.tag != tag {
				continue
			}
			object := unsafe_New(v.typ)
			if err := v.plan.read(r, object); err != nil {
				return err
			}
			*(*iface)(dst) = iface{tab: v.tab, data: object}
			return nil
		}
		return cursor.ErrUnsupported
	}
}

// mapMaxElemBytes is the size above which a map keeps a pointer to each element in its slots
// instead of the element: internal/abi.MapMaxElemBytes, which the compiler goes by too.
const mapMaxElemBytes = 128

// inlineKey is the room a key of a map is read into, in the Cursor: 4 words, enough for a felt.
type inlineKey = [4]uint64

// pointerFree reports whether a value of type t holds no pointer, so that it can be kept where
// the garbage collector does not look.
func pointerFree(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array:
		return pointerFree(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if !pointerFree(t.Field(i).Type) {
				return false
			}
		}
		return true
	}
	return false
}

// selfContained reports whether the reader of a value of type t calls no code outside the planner,
// which could keep the pointer it is given: a scalar, a felt, or an array or struct of them. A
// pointer to the stack must not reach any other, for a stack that grows moves what it holds but not
// the pointers to it that the heap holds.
func selfContained(t reflect.Type) bool {
	if isFelt(t) {
		return true
	}
	if ownEncoding(t) || opaque(t) {
		return false
	}
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Array:
		return selfContained(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() && !selfContained(f.Type) {
				return false
			}
		}
		return true
	}
	return false
}

// mapReader reads a map from the readers of its keys and its values. It makes the map with the
// runtime and reads each value into the place the map keeps it. A key that is an unsigned integer
// of 4 or 8 bytes is read from the cursor and stored by the runtime's faster calls for such keys;
// another key is read into the same place again and again, in the cursor if it has no pointer and
// is small, and the map copies it.
func (p *planner) mapReader(t reflect.Type, path string) fieldReader {
	keyType, valueType := t.Key(), t.Elem()
	readKey := p.reader(keyType, path+"[key]")
	readValue := p.reader(valueType, path+"[value]")
	if readKey == nil || readValue == nil {
		return nil
	}
	mapType := runtimeTypeOf(t)
	keyRuntime, valueRuntime := runtimeTypeOf(keyType), runtimeTypeOf(valueType)
	// The runtime's calls for integer keys are the ones the compiler uses for a map whose elements
	// are kept in its slots, which they are up to mapMaxElemBytes; beyond it the slot holds a pointer
	// to the element, which only the general call knows.
	plain := !ownEncoding(keyType) && p.types[keyType] == nil && valueType.Size() <= mapMaxElemBytes
	if plain {
		switch {
		case keyType.Kind() == reflect.Uint32:
			return integerKeyedMap(mapType, valueRuntime, readValue, 32)
		case keyType.Kind() == reflect.Uint64 || keyType.Kind() == reflect.Uint && keyType.Size() == 8:
			return integerKeyedMap(mapType, valueRuntime, readValue, 64)
		}
	}
	inCursor := pointerFree(keyType) && selfContained(keyType) &&
		keyType.Size() <= unsafe.Sizeof(inlineKey{}) && keyType.Align() <= 8
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*unsafe.Pointer)(dst) = nil
			return nil
		}
		count, err := r.MapLen()
		if err != nil {
			return err
		}
		out := makemap(mapType, count)
		key := r.Scratch()
		if !inCursor {
			key = unsafe_New(keyRuntime)
		}
		for ; count > 0; count-- {
			if r.Null() {
				// fxamacker keeps the previous key for a null one; it is not worth copying.
				return cursor.ErrUnsupported
			}
			if inCursor {
				*(*inlineKey)(key) = inlineKey{}
			} else {
				typedmemclr(keyRuntime, key)
			}
			if err := readKey(r, key); err != nil {
				return err
			}
			slot := mapassign(mapType, out, key)
			// The key may have been there already.
			typedmemclr(valueRuntime, slot)
			if err := readValue(r, slot); err != nil {
				return err
			}
		}
		r.Leave()
		*(*unsafe.Pointer)(dst) = out
		return nil
	}
}

// integerKeyedMap reads a map whose keys are unsigned integers of the given width in bits. The
// slot of a key not seen before is already zero; of the small keys seen so far a bitmap says which
// were, and the slot of a large key is always cleared.
func integerKeyedMap(
	mapType, valueRuntime *runtimeType, readValue fieldReader, bits int,
) fieldReader {
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*unsafe.Pointer)(dst) = nil
			return nil
		}
		count, err := r.MapLen()
		if err != nil {
			return err
		}
		out := makemap(mapType, count)
		var seen uint64
		for ; count > 0; count-- {
			if r.Null() {
				// fxamacker keeps the previous key for a null one; it is not worth copying.
				return cursor.ErrUnsupported
			}
			key, err := r.Uint64()
			if err != nil {
				return err
			}
			var slot unsafe.Pointer
			if bits == 32 {
				if key > math.MaxUint32 {
					return cursor.ErrType
				}
				slot = mapassign_fast32(mapType, out, uint32(key))
			} else {
				slot = mapassign_fast64(mapType, out, key)
			}
			if key >= 64 || seen&(1<<key) != 0 {
				typedmemclr(valueRuntime, slot)
			}
			if key < 64 {
				seen |= 1 << key
			}
			if err := readValue(r, slot); err != nil {
				return err
			}
		}
		r.Leave()
		*(*unsafe.Pointer)(dst) = out
		return nil
	}
}

// opaque reports whether t is read without a struct plan: big.Int, and a struct with no exported
// field.
func opaque(t reflect.Type) bool {
	return t == reflect.TypeFor[big.Int]() || t.Kind() == reflect.Struct && !hasExportedField(t)
}

// probed has what probeFeltLike decided of each type.
var probed = map[reflect.Type]bool{}

// probeFeltLike finds out whether t, or the type t points to, is a felt that encodes itself in the
// way a felt does, and if so tells the planner to read it as one. It is found out, not assumed: a
// few values are written by fxamacker as the type and as a felt and the bytes must be the same, and
// the bytes of the felt must read back into the type as the same value.
func probeFeltLike(t reflect.Type) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if feltEncoded[t] {
		return
	}
	if _, done := probed[t]; done {
		return
	}
	probed[t] = false
	if t.Kind() != reflect.Array || t.Len() != cursor.Limbs || t.Elem().Kind() != reflect.Uint64 ||
		t.Size() != reflect.TypeFor[felt.Felt]().Size() || !ownEncoding(t) {
		return
	}
	defer func() {
		if recover() != nil {
			probed[t] = false
			delete(feltEncoded, t)
		}
	}()
	for _, limbs := range [][cursor.Limbs]uint64{
		{},
		{1},
		{0, 1},
		{0, 0, 0, 1},
		{0x1234567890abcdef, 0x0fedcba098765432, 0x0123456789abcdef, 0x00ff00ff00ff00ff},
	} {
		own, plain, back := reflect.New(t), reflect.New(reflect.TypeFor[felt.Felt]()), reflect.New(t)
		*(*[cursor.Limbs]uint64)(own.UnsafePointer()) = limbs
		*(*[cursor.Limbs]uint64)(plain.UnsafePointer()) = limbs
		ownBytes, err := fxamacker.Marshal(own.Interface())
		if err != nil {
			return
		}
		plainBytes, err := fxamacker.Marshal(plain.Interface())
		if err != nil || !bytes.Equal(ownBytes, plainBytes) {
			return
		}
		if err := fxamacker.Unmarshal(plainBytes, back.Interface()); err != nil ||
			*(*[cursor.Limbs]uint64)(back.UnsafePointer()) != limbs {
			return
		}
	}
	probed[t] = true
	feltEncoded[t] = true
}

// maxSharedSize is the largest struct that sibling fields share an allocation of: one of them may
// be absent, and then its room is wasted.
const maxSharedSize = 128

// siblingGroup is the fields of a struct that are pointers to the same struct type, whose values
// are allocated together, when the first of them is read, as an array.
type siblingGroup struct {
	elem   reflect.Type
	fields []int // indexes into the fields of the struct
}

// siblingGroups finds the groups of two or more fields of a struct that point to the same small
// struct type, and returns the group of each field by its index.
func (p *planner) siblingGroups(all []wireField) map[int]*siblingGroup {
	if !p.generic {
		return nil
	}
	byElem := map[reflect.Type][]int{}
	for i, f := range all {
		if _, overridden := p.fields[f.owner][f.goName]; overridden || f.typ.Kind() != reflect.Pointer {
			continue
		}
		elem := f.typ.Elem()
		if elem.Kind() != reflect.Struct || elem.Size() == 0 || elem.Size() > maxSharedSize ||
			p.types[f.typ] != nil || p.types[elem] != nil || opaque(elem) ||
			ownEncoding(f.typ) || ownEncoding(elem) {
			continue
		}
		byElem[elem] = append(byElem[elem], i)
	}
	groups := map[int]*siblingGroup{}
	for elem, fields := range byElem {
		if len(fields) < 2 {
			continue
		}
		g := &siblingGroup{elem: elem, fields: fields}
		for _, i := range fields {
			groups[i] = g
		}
	}
	return groups
}

// siblingReader reads the pointer field self of a group. The first of the group read allocates the
// array of all their values; the others find it from the pointer of a sibling that was read, as
// only these readers set the fields of a group.
func (p *planner) siblingReader(
	g *siblingGroup, all []wireField, self int, path string,
) fieldReader {
	plan := p.structOf(g.elem, path)
	size, runtime, count := int(g.elem.Size()), runtimeTypeOf(g.elem), len(g.fields)
	type sibling struct{ delta, slot int }
	var (
		others []sibling
		mine   int
	)
	for slot, i := range g.fields {
		if i == self {
			mine = slot
			continue
		}
		others = append(others, sibling{delta: int(all[i].offset) - int(all[self].offset), slot: slot})
	}
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*unsafe.Pointer)(dst) = nil
			return nil
		}
		var array unsafe.Pointer
		for _, o := range others {
			if sibling := *(*unsafe.Pointer)(unsafe.Add(dst, o.delta)); sibling != nil {
				array = unsafe.Add(sibling, -o.slot*size)
				break
			}
		}
		if array == nil {
			array = unsafe_NewArray(runtime, count)
		}
		value := unsafe.Add(array, mine*size)
		if err := plan.read(r, value); err != nil {
			return err
		}
		*(*unsafe.Pointer)(dst) = value
		return nil
	}
}

// intKey is a field of a struct keyed by integers.
type intKey struct {
	key    uint64
	offset uintptr
	read   fieldReader
}

// hasIntKeys reports whether a field of the struct type t is keyed by an integer.
func hasIntKeys(t reflect.Type) bool {
	for i := range t.NumField() {
		f := t.Field(i)
		if _, options, _ := strings.Cut(fieldTag(f.Tag), ","); f.IsExported() &&
			slices.Contains(strings.Split(options, ","), "keyasint") {
			return true
		}
	}
	return false
}

// fillIntKeyed plans a struct all of whose fields are keyed by integers, as
// BlockTransactionsIndexes is. Embedded structs and fields keyed by names are not supported.
func (p *planner) fillIntKeyed(plan *structPlan, t reflect.Type, path string) bool {
	var (
		keys []intKey
		ok   = true
	)
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := fieldTag(f.Tag)
		if tag == "-" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		key, err := strconv.ParseUint(name, 10, 63)
		if f.Anonymous || !slices.Contains(strings.Split(options, ","), "keyasint") || err != nil {
			p.fail("%s: %v mixes integer keys with other fields", path, t)
			return false
		}
		read := p.reader(f.Type, path+"."+f.Name)
		if read == nil {
			ok = false
			continue
		}
		keys = append(keys, intKey{key: key, offset: f.Offset, read: read})
	}
	if !ok {
		return false
	}
	if len(keys) == 0 || len(keys) > 64 {
		p.fail("%s: %v has no field keyed by an integer, or more than 64", path, t)
		return false
	}
	plan.fields, plan.intKeys = cursor.NewFields(), keys
	return true
}

// readIntKeys reads one map into the struct at base, by the integer keys of its fields. A key of
// another integer, or a text key, names no field, and neither does a key that comes twice.
func (p *structPlan) readIntKeys(r *cursor.Cursor, base unsafe.Pointer) error {
	if r.Null() {
		return nil
	}
	count, err := r.MapLen()
	if err != nil {
		return err
	}
	var seen uint64
	for ; count > 0; count-- {
		field, err := p.intKeyField(r)
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
		key := &p.intKeys[field]
		if err := key.read(r, unsafe.Add(base, key.offset)); err != nil {
			return err
		}
	}
	r.Leave()
	return nil
}

// intKeyField reads a key and returns the field it names, or -1. fxamacker reads integer keys as
// int64 and checks that text keys are UTF-8.
func (p *structPlan) intKeyField(r *cursor.Cursor) (int, error) {
	if head, err := r.Peek(); err != nil || head&0xe0 != cursor.MajorUint {
		_, _, err := r.Key()
		return -1, err
	}
	key, err := r.Uint64()
	if err != nil {
		return -1, err
	}
	if key > math.MaxInt64 {
		return -1, cursor.ErrType
	}
	for i := range p.intKeys {
		if p.intKeys[i].key == key {
			return i, nil
		}
	}
	return -1, nil
}

// maxFusedChildren is the most room that the small structs a struct points to may take together
// for them to be allocated with it, in bytes: when some are absent their room is wasted.
const (
	maxFusedChild    = 32
	maxFusedChildren = 64
)

// fusedPlan reads a struct behind a pointer, from a block that holds the struct and the small
// structs it points to, so that one allocation serves all of them.
type fusedPlan struct {
	alloc *runtimeType
	plan  *structPlan
}

// fuseChildren finds the fields of the struct t that point to small structs of their own, and if
// there are some makes the plan that allocates them with the struct, as the struct behind a
// pointer. A pointer to one such child keeps the whole block alive.
func (p *planner) fuseChildren(
	plan *structPlan, t reflect.Type, all []wireField,
	groups map[int]*siblingGroup, readIndex map[int]int, path string,
) {
	type child struct {
		field wireField
		read  int
		elem  reflect.Type
	}
	var (
		children []child
		total    uintptr
	)
	for idx, f := range all {
		read, ordinary := readIndex[idx]
		if groups[idx] != nil || !ordinary || f.typ.Kind() != reflect.Pointer {
			continue
		}
		elem := f.typ.Elem()
		if elem.Kind() != reflect.Struct || elem == t || elem.Size() == 0 ||
			elem.Size() > maxFusedChild || p.types[f.typ] != nil || p.types[elem] != nil ||
			opaque(elem) || ownEncoding(f.typ) || ownEncoding(elem) {
			continue
		}
		children = append(children, child{field: f, read: read, elem: elem})
		total += elem.Size()
	}
	if len(children) == 0 || total > maxFusedChildren {
		return
	}
	fields := []reflect.StructField{{Name: "V", Type: t}}
	for i, c := range children {
		fields = append(fields, reflect.StructField{Name: "C" + strconv.Itoa(i), Type: c.elem})
	}
	block, ok := structOfOrNil(fields)
	if !ok {
		return
	}
	reads := slices.Clone(plan.reads)
	for i, c := range children {
		childPlan := p.structOf(c.elem, path+"."+c.field.goName)
		// dst is the field, which is at its offset in the struct, which is at the start of the block.
		delta := int(block.Field(i+1).Offset) - int(c.field.offset)
		read := func(r *cursor.Cursor, dst unsafe.Pointer) error {
			if r.Null() {
				*(*unsafe.Pointer)(dst) = nil
				return nil
			}
			value := unsafe.Add(dst, delta)
			if err := childPlan.read(r, value); err != nil {
				return err
			}
			*(*unsafe.Pointer)(dst) = value
			return nil
		}
		reads[c.read] = fieldRead{offset: c.field.offset, op: opCall, read: read}
	}
	fused := *plan
	fused.reads, fused.fused = reads, nil
	plan.fused = &fusedPlan{alloc: runtimeTypeOf(block), plan: &fused}
}

// structOfOrNil is reflect.StructOf, which panics for the types it cannot lay out.
func structOfOrNil(fields []reflect.StructField) (t reflect.Type, ok bool) {
	defer func() {
		if recover() != nil {
			t, ok = nil, false
		}
	}()
	return reflect.StructOf(fields), true
}
