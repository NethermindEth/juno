package fastcbor

import (
	"cmp"
	"encoding"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

// A plan is the reader of one struct type, built from the type when the package registers its
// readers, by the rules fxamacker follows to decode it: the wire name of each field, and for each
// the offset of the field and the function that reads its Go type. The function of a field is
// found once, when the plan is made, so a read does no reflection: it walks the map with
// NextField and calls the function of the field each key names on the field's address.
//
// The function of a type comes from, in this order: the overrides (overrides.go), for the types a
// rule cannot read, by their Go type; the rules for felts; and the rules for each kind of type
// (integers, strings, structs, and pointers to and slices of anything with a reader). A type with
// none, or one with its own CBOR encoding and no override, makes the plan fail, naming the type
// and the field; the record is then left to fxamacker, and a test fails.
//
// A struct with fields of size 0 that decode themselves is a projection: a few fields of a stored
// record, the rest discarded. Its plan reads only the fields it names, with NextWanted.

// fieldReader reads one item into the value that dst points to.
type fieldReader func(r *cursor.Cursor, dst unsafe.Pointer) error

// asFieldReader makes read a fieldReader. A pointer to a T and an unsafe.Pointer are passed the
// same way, so the reader is called directly, without a closure that would only convert the
// pointer.
func asFieldReader[T any](read func(*cursor.Cursor, *T) error) fieldReader {
	return *(*fieldReader)(unsafe.Pointer(&read))
}

// structPlan is the plan of one struct type.
type structPlan struct {
	// typ is the struct type, and alloc the runtime's.
	typ   reflect.Type
	alloc *runtimeType
	// fields has the wire name of each field to read, by field number.
	fields *cursor.Fields
	// reads has the reader of each field, by field number.
	reads []fieldRead
	// partial is whether the struct is a projection, which reads only the keys it names.
	partial bool
	// intKeys has the fields of a struct keyed by integers, which is read by readIntKeys instead.
	intKeys []intKey
	// fused is the plan that reads this struct behind a pointer when the small structs it points to
	// are allocated with it, or nil.
	fused *fusedPlan
	// special is whether the struct is read by readPart or readIntKeys, which one check tells.
	special bool
}

type fieldRead struct {
	// offset is where the field is in the struct, or for a field override where the struct that
	// declares the field is.
	offset uintptr
	op     op
	// read reads the field for opCall, and plan for opStruct and opPtr.
	read fieldReader
	plan *structPlan
}

// op is how a field is read. Most fields are of a few types, which the loop of a plan reads with a
// direct call to their helper, or to the plan of a struct; the others call their reader.
type op uint8

const (
	// opCall calls the field's reader.
	opCall op = iota
	// opFelt is a pointer to a felt.
	opFelt
	opUint64
	// opFeltSlice is a slice of felts.
	opFeltSlice
	opBool
	opString
	// opStruct is a struct held by value, and opPtr a pointer to one.
	opStruct
	opPtr
)

// readAt reads the field of the struct at base. The loop of read has the same switch written out:
// calling readAt there measured 2 to 4% slower on the small records.
func (f *fieldRead) readAt(r *cursor.Cursor, base unsafe.Pointer) error {
	dst := unsafe.Add(base, f.offset)
	switch f.op {
	case opFelt:
		return readFelt(r, (**felt.Felt)(dst))
	case opUint64:
		return readUint64(r, (*uint64)(dst))
	case opFeltSlice:
		return readFeltSlice(r, (*felt.Slice[felt.Felt])(dst))
	case opBool:
		return readBool(r, (*bool)(dst))
	case opString:
		return readString(r, (*string)(dst))
	case opStruct:
		return f.plan.read(r, dst)
	case opPtr:
		return f.plan.readPtr(r, dst)
	default:
		return f.read(r, dst)
	}
}

// readPtr reads a struct behind the pointer at dst. Null leaves the pointer nil.
func (p *structPlan) readPtr(r *cursor.Cursor, dst unsafe.Pointer) error {
	if r.Null() {
		*(*unsafe.Pointer)(dst) = nil
		return nil
	}
	if f := p.fused; f != nil {
		value := unsafe_New(f.alloc)
		if err := f.plan.read(r, value); err != nil {
			return err
		}
		*(*unsafe.Pointer)(dst) = value
		return nil
	}
	value := unsafe_New(p.alloc)
	if err := p.read(r, value); err != nil {
		return err
	}
	*(*unsafe.Pointer)(dst) = value
	return nil
}

// read reads one map into the struct at base.
func (p *structPlan) read(r *cursor.Cursor, base unsafe.Pointer) error {
	if p.special {
		if p.partial {
			return p.readPart(r, base)
		}
		return p.readIntKeys(r, base)
	}
	s, err := r.OpenStruct(p.fields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		if field < 0 {
			if field == cursor.Done {
				return err
			}
			err = r.Skip() // a key that names no field
			continue
		}
		f := &p.reads[field]
		dst := unsafe.Add(base, f.offset)
		switch f.op {
		case opFelt:
			err = readFelt(r, (**felt.Felt)(dst))
		case opUint64:
			err = readUint64(r, (*uint64)(dst))
		case opFeltSlice:
			err = readFeltSlice(r, (*felt.Slice[felt.Felt])(dst))
		case opBool:
			err = readBool(r, (*bool)(dst))
		case opString:
			err = readString(r, (*string)(dst))
		case opStruct:
			err = f.plan.read(r, dst)
		case opPtr:
			err = f.plan.readPtr(r, dst)
		default:
			err = f.read(r, dst)
		}
	}
	return err
}

// readPart reads the fields of a projection. Like fxamacker it looks through the tag of the
// record, as a projection is not a registered type; tags 0-3 hold a time or a bignum, which
// fxamacker reads differently, and so does a chain of tags.
func (p *structPlan) readPart(r *cursor.Cursor, base unsafe.Pointer) error {
	if head, err := r.Peek(); err == nil && head&0xe0 == cursor.MajorTag {
		tag, err := r.Tag()
		if err != nil {
			return err
		}
		if next, err := r.Peek(); tag <= 3 || (err == nil && next&0xe0 == cursor.MajorTag) {
			return cursor.ErrUnsupported
		}
	}
	s, err := r.OpenStruct(p.fields)
	for err == nil {
		var field int
		field, err = r.NextWanted(&s)
		if field == cursor.Done {
			return err
		}
		err = p.reads[field].readAt(r, base)
	}
	return err
}

// overrides holds what the planner cannot work out from a type.
type overrides struct {
	// types gives the reader of a type, made from the planner so that it can use the plans of other
	// types. It is consulted before any rule.
	types map[reflect.Type]func(*planner) fieldReader
	// fields gives the reader of a field, by the struct that declares it and the field's Go name,
	// made from the planner. The reader is given the address of that struct, not of the field.
	fields map[reflect.Type]map[string]func(*planner) fieldReader
	// generic reads the types no rule reads from what the types themselves say, so the planner
	// needs no knowledge of Juno's types.
	generic bool
}

// plans has the plans made so far for the roots planned in full, which later roots share.
type plans struct {
	overrides
	structs map[reflect.Type]*structPlan
	made    map[reflect.Type]fieldReader
}

func newPlans(o overrides) *plans {
	return &plans{
		overrides: o, structs: map[reflect.Type]*structPlan{}, made: map[reflect.Type]fieldReader{},
	}
}

// planner builds the readers of the types one root type needs. A root is planned on its own, from
// the plans of the roots before it, so that a type that cannot be planned fails only the roots that
// hold it.
type planner struct {
	overrides
	// structs has the plan of each struct type seen, which is filled once all its fields are
	// planned; a type that holds itself finds its own plan there, still empty.
	structs map[reflect.Type]*structPlan
	// made has the reader each type override made.
	made map[reflect.Type]fieldReader
	// errs has what could not be planned.
	errs []error
}

// planner returns the reader of the type t, which reads one item into a value of type t, and the
// planner that made it with the plans it made, or an error naming every field the planner could
// not read.
func (s *plans) planner(t reflect.Type) (fieldReader, *planner, error) {
	p := &planner{overrides: s.overrides, structs: maps.Clone(s.structs), made: maps.Clone(s.made)}
	read := p.reader(t, t.String())
	if len(p.errs) > 0 {
		return nil, p, errors.Join(p.errs...)
	}
	s.structs, s.made = p.structs, p.made
	return read, p, nil
}

func (p *planner) fail(format string, args ...any) fieldReader {
	p.errs = append(p.errs, fmt.Errorf(format, args...))
	return nil
}

// planned is the plan of the struct type T, for an override to call.
type planned[T any] struct {
	plan *structPlan
}

func (p planned[T]) read(r *cursor.Cursor, dst *T) error {
	return p.plan.read(r, unsafe.Pointer(dst))
}

// planOf returns the plan of the struct type T.
func planOf[T any](p *planner) planned[T] {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		p.fail("records: planOf needs a struct, not %v", t)
		return planned[T]{plan: &structPlan{fields: cursor.NewFields()}}
	}
	return planned[T]{plan: p.structOf(t, t.String())}
}

var (
	selfDecoder = reflect.TypeFor[fxamacker.SelfDecoder]()
	// encodings are the interfaces through which a type takes over its own encoding, among those
	// fxamacker honors with the options Juno gives it (TextMarshaler is off by default).
	encodings = []reflect.Type{
		reflect.TypeFor[fxamacker.SelfEncoder](),
		selfDecoder,
		reflect.TypeFor[encoding.BinaryMarshaler](),
		reflect.TypeFor[encoding.BinaryUnmarshaler](),
	}
)

// ownEncoding reports whether t or a pointer to it encodes or decodes itself.
func ownEncoding(t reflect.Type) bool {
	for _, e := range encodings {
		if t.Implements(e) || reflect.PointerTo(t).Implements(e) {
			return true
		}
	}
	return false
}

// isFelt reports whether t is stored as a felt, an array of four unsigned integers: an array of
// four uint64, and either plain or one of the types whose own encoding writes the same.
func isFelt(t reflect.Type) bool {
	return t.Kind() == reflect.Array && t.Len() == cursor.Limbs && t.Elem().Kind() == reflect.Uint64 &&
		(feltEncoded[t] || !ownEncoding(t))
}

// reader returns the reader of the type t, found at path, or records why there is none and
// returns nil.
func (p *planner) reader(t reflect.Type, path string) fieldReader {
	if read, ok := p.made[t]; ok {
		return read
	}
	if build, ok := p.types[t]; ok {
		read := build(p)
		p.made[t] = read
		return read
	}
	if p.generic {
		probeFeltLike(t)
	}
	if read := feltReader(t); read != nil {
		return read
	}
	if ownEncoding(t) {
		if p.generic {
			return p.ownEncodingReader(t, path)
		}
		return p.fail("%s: %v has its own CBOR encoding but does not decode itself", path, t)
	}

	if p.generic {
		switch {
		case t == reflect.TypeFor[*big.Int]():
			return asFieldReader(readBigIntPtr)
		case t.Kind() == reflect.Interface:
			return p.interfaceReader(t, path)
		case t.Kind() == reflect.Map:
			return p.mapReader(t, path)
		case t.Kind() == reflect.Struct && !hasExportedField(t):
			return opaqueReader(t)
		}
	}

	switch t.Kind() {
	case reflect.Pointer:
		return p.pointerReader(t, path)
	case reflect.Slice:
		return p.sliceReader(t, path)
	case reflect.Struct:
		return p.structOf(t, path).read
	}
	if read := scalarReader(t); read != nil {
		return read
	}
	return p.fail("%s: no rule reads a %v; check how fxamacker stores it and add a rule", path, t)
}

// scalarReader returns the reader of a boolean, a string, an unsigned integer or an array of bytes,
// or nil if t is none of these.
func scalarReader(t reflect.Type) fieldReader {
	switch t.Kind() {
	case reflect.Bool:
		return asFieldReader(readBool)
	case reflect.String:
		return asFieldReader(readString)
	case reflect.Uint64:
		return asFieldReader(readUint64)
	case reflect.Uint:
		return asFieldReader(readUint[uint])
	case reflect.Uint32:
		return asFieldReader(readUint32[uint32])
	case reflect.Uint16:
		return asFieldReader(readUint16)
	case reflect.Uint8:
		return asFieldReader(readByte)
	case reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 && !ownEncoding(t.Elem()) {
			return readByteArray(t.Len())
		}
	}
	return nil
}

// feltReader returns the reader of a felt, of a pointer to one or of a slice of felts or of
// pointers to them, or nil if t is none of these.
func feltReader(t reflect.Type) fieldReader {
	switch {
	case isFelt(t):
		return asFieldReader(readFeltInPlace)
	case t.Kind() == reflect.Pointer && isFelt(t.Elem()):
		// Every felt type has the layout and the pointer-free memory of a felt.Felt.
		return asFieldReader(readFelt)
	case t.Kind() == reflect.Slice && isFelt(t.Elem()) && (feltEncoded[t] || !ownEncoding(t)):
		return asFieldReader(readFeltSlice)
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Pointer &&
		isFelt(t.Elem().Elem()) && !ownEncoding(t):
		return asFieldReader(readFeltPtrSlice)
	}
	return nil
}

// pointerReader returns the reader of the pointer type t: null leaves the pointer nil, anything
// else is read into a new value.
func (p *planner) pointerReader(t reflect.Type, path string) fieldReader {
	elem := t.Elem()
	read := p.reader(elem, path)
	if read == nil {
		return nil
	}
	if elem.Kind() == reflect.Struct && p.types[elem] == nil {
		return p.structOf(elem, path).readPtr
	}
	alloc := runtimeTypeOf(elem)
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*unsafe.Pointer)(dst) = nil
			return nil
		}
		value := unsafe_New(alloc)
		if err := read(r, value); err != nil {
			return err
		}
		*(*unsafe.Pointer)(dst) = value
		return nil
	}
}

// sliceReader returns the reader of the slice type t.
func (p *planner) sliceReader(t reflect.Type, path string) fieldReader {
	elem := t.Elem()
	if elem.Kind() == reflect.Uint8 && !ownEncoding(elem) {
		return asFieldReader(readRaw)
	}
	read := p.reader(elem, path)
	if read == nil {
		return nil
	}
	if elem.Kind() == reflect.Pointer && elem.Elem().Kind() == reflect.Struct &&
		p.types[elem] == nil && p.types[elem.Elem()] == nil {
		return readPtrSliceOf(p.structOf(elem.Elem(), path), t, reflect.SliceOf(elem.Elem()))
	}
	if elem.Kind() == reflect.Struct && p.types[elem] == nil {
		read = nil
	}
	return readSliceOf(read, p.structs[elem], t)
}

// structOf returns the plan of the struct type t, making it if it is new. Until the fields of a
// struct that holds itself are planned, its plan is empty.
func (p *planner) structOf(t reflect.Type, path string) *structPlan {
	if plan, ok := p.structs[t]; ok {
		return plan
	}
	plan := &structPlan{typ: t, alloc: runtimeTypeOf(t)}
	p.structs[t] = plan
	if !p.fillStruct(plan, t, path) {
		// Keep an empty plan, so that the readers already made of it stay valid; the error is in errs.
		plan.fields = cursor.NewFields()
	}
	plan.special = plan.partial || plan.intKeys != nil
	return plan
}

// fillStruct plans the fields of the struct type t, as fxamacker decodes it: the visible fields, by
// wire name.
func (p *planner) fillStruct(plan *structPlan, t reflect.Type, path string) bool {
	if ownEncoding(t) {
		p.fail("%s: %v has its own CBOR encoding; check what it writes and give it an override", path, t)
		return false
	}
	if p.generic && hasIntKeys(t) {
		return p.fillIntKeyed(plan, t, path)
	}
	if f, ok := t.FieldByName("_"); ok && f.Tag.Get("cbor") != "" && f.Tag.Get("cbor") != "-" {
		p.fail("%s: %v has options for the whole struct, such as toarray", path, t)
		return false
	}
	all, err := collectFields(t, 0, 0, map[reflect.Type]bool{t: true}, nil)
	if err != nil {
		p.fail("%s: %w", path, err)
		return false
	}
	all = visibleFields(all)

	var (
		names []string
		reads []fieldRead
		// skipped has the names of the fields not read.
		skipped []string
		ok      = true
	)
	groups := p.siblingGroups(all)
	readIndex := map[int]int{} // the index in reads of each field read as an ordinary field
	for idx, f := range all {
		fieldPath := path + "." + f.goName
		if group, shared := groups[idx]; shared {
			names = append(names, f.name)
			read := p.siblingReader(group, all, idx, fieldPath)
			reads = append(reads, fieldRead{offset: f.offset, op: opCall, read: read})
			continue
		}
		if build, overridden := p.fields[f.owner][f.goName]; overridden {
			names = append(names, f.name)
			reads = append(reads, fieldRead{offset: f.ownerOffset, read: build(p)})
			continue
		}
		if f.typ.Size() == 0 && reflect.PointerTo(f.typ).Implements(selfDecoder) {
			skipped = append(skipped, f.name)
			continue
		}
		read := p.reader(f.typ, fieldPath)
		if read == nil {
			ok = false
			continue
		}
		names = append(names, f.name)
		readIndex[idx] = len(reads)
		reads = append(reads, p.fieldRead(&f, read))
	}
	if !ok {
		return false
	}
	if len(names) > 64 || !distinctInCase(names, skipped) {
		p.fail("%s: %v has more than 64 fields, or names that are not ASCII or differ only in case",
			path, t)
		return false
	}
	plan.fields, plan.reads, plan.partial = cursor.NewFields(names...), reads, len(skipped) > 0
	if p.generic && !plan.partial {
		p.fuseChildren(plan, t, all, groups, readIndex, path)
	}
	return true
}

// fieldRead returns how to read the field f, whose type read reads: through one of the ops for
// the common types, or by calling read.
func (p *planner) fieldRead(f *wireField, read fieldReader) fieldRead {
	out := fieldRead{offset: f.offset, op: opCall, read: read}
	t := f.typ
	switch {
	case p.types[t] != nil:
	case p.generic && (opaque(t) || t.Kind() == reflect.Pointer && opaque(t.Elem())):
	case t.Kind() == reflect.Struct:
		out.op, out.plan = opStruct, p.structs[t]
	case t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct && p.types[t.Elem()] == nil:
		out.op, out.plan = opPtr, p.structs[t.Elem()]
	default:
		out.op = scalarOp(t)
	}
	return out
}

// scalarOp returns the op of a field of type t that holds no struct.
func scalarOp(t reflect.Type) op {
	switch {
	case t.Kind() == reflect.Pointer && isFelt(t.Elem()):
		return opFelt
	case t.Kind() == reflect.Slice && isFelt(t.Elem()) && (feltEncoded[t] || !ownEncoding(t)):
		return opFeltSlice
	case ownEncoding(t):
		return opCall
	}
	switch t.Kind() {
	case reflect.Uint64:
		return opUint64
	case reflect.Bool:
		return opBool
	case reflect.String:
		return opString
	}
	return opCall
}

// wireField is a field of a struct as fxamacker sees it, with the embedded structs promoted.
type wireField struct {
	name   string
	offset uintptr
	typ    reflect.Type
	// goName is the field's name in Go, owner the struct that declares it and ownerOffset where
	// that struct is.
	goName      string
	owner       reflect.Type
	ownerOffset uintptr
	// depth is the number of embedded structs the field is in, and tagged whether it has a tag.
	depth  int
	tagged bool
}

// collectFields lists the fields of the struct type t, which is at offset base in the struct being
// planned, and those of the structs embedded in it. It follows fxamacker's rules for which fields
// there are, and gives up on the unusual: an embedded pointer, a struct embedded twice, integer
// keys.
func collectFields(
	t reflect.Type,
	base uintptr,
	depth int,
	embedded map[reflect.Type]bool,
	out []wireField,
) ([]wireField, error) {
	for i := range t.NumField() {
		f := t.Field(i)
		elem := f.Type
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		if !f.IsExported() && (!f.Anonymous || elem.Kind() != reflect.Struct) {
			continue
		}
		tag := fieldTag(f.Tag)
		if tag == "-" {
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		if slices.Contains(strings.Split(options, ","), "keyasint") {
			return nil, fmt.Errorf("%v.%s has an integer key", t, f.Name)
		}

		if f.Anonymous && elem.Kind() == reflect.Struct && name == "" {
			if f.Type.Kind() == reflect.Pointer || embedded[elem] {
				return nil, fmt.Errorf("%v.%s is an embedded pointer, or a struct embedded twice", t, f.Name)
			}
			embedded[elem] = true
			var err error
			if out, err = collectFields(elem, base+f.Offset, depth+1, embedded, out); err != nil {
				return nil, err
			}
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, wireField{
			name: name, offset: base + f.Offset, typ: f.Type,
			goName: f.Name, owner: t, ownerOffset: base,
			depth: depth, tagged: tag != "",
		})
	}
	return out, nil
}

// fieldTag is the tag fxamacker reads from a field: the cbor one, or failing that the json one.
func fieldTag(tag reflect.StructTag) string {
	if cborTag := tag.Get("cbor"); cborTag != "" {
		return cborTag
	}
	return tag.Get("json")
}

// visibleFields keeps, of the fields of a name, the one fxamacker decodes into, if any: the least
// nested, and of those a tagged one over untagged ones. Several of the same depth and tag hide
// one another.
func visibleFields(fields []wireField) []wireField {
	sorted := slices.Clone(fields)
	slices.SortStableFunc(sorted, func(a, b wireField) int {
		return cmp.Or(
			strings.Compare(a.name, b.name),
			cmp.Compare(a.depth, b.depth),
			-compareBool(a.tagged, b.tagged),
		)
	})
	var out []wireField
	for i := 0; i < len(sorted); {
		f := sorted[i]
		j := i + 1
		for j < len(sorted) && sorted[j].name == f.name {
			j++
		}
		if j == i+1 || f.depth < sorted[i+1].depth || (f.tagged && !sorted[i+1].tagged) {
			out = append(out, f)
		}
		i = j
	}
	return out
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// distinctInCase reports whether the names to read can be matched as fxamacker does, which takes
// the field of an exact name before one of a name that differs in case, with only these names. It
// is so if they are ASCII, short enough, and differ from each other and from the names of the
// fields not read by more than case.
func distinctInCase(names, skipped []string) bool {
	seen := map[string]bool{}
	for _, name := range names {
		nonASCII := strings.ContainsFunc(name, func(c rune) bool { return c >= utf8.RuneSelf })
		if len(name) >= 1<<8 || nonASCII {
			return false
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			return false
		}
		seen[lower] = true
	}
	for _, name := range skipped {
		if seen[strings.ToLower(name)] {
			return false
		}
	}
	return true
}

// sliceHeader is the layout of a slice.
type sliceHeader struct {
	data     unsafe.Pointer
	len, cap int
}

// empty is where an empty slice points, which must not be nil.
var empty [0]uint64

// newArray returns a new zeroed array of n elements of type elem, which is not nil when n is 0.
func newArray(elem *runtimeType, n int) unsafe.Pointer {
	if n == 0 {
		return unsafe.Pointer(&empty)
	}
	return unsafe_NewArray(elem, n)
}

// readSliceOf returns the reader of the slice type t, whose elements read reads, or plan if read is
// nil: an array, or null as nil.
func readSliceOf(read fieldReader, plan *structPlan, t reflect.Type) fieldReader {
	size, elem := t.Elem().Size(), runtimeTypeOf(t.Elem())
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*sliceHeader)(dst) = sliceHeader{}
			return nil
		}
		count, err := r.ArrayLen()
		if err != nil {
			return err
		}
		out := newArray(elem, count)
		for i := range count {
			value := unsafe.Add(out, uintptr(i)*size)
			if read != nil {
				err = read(r, value)
			} else {
				err = plan.read(r, value)
			}
			if err != nil {
				return err
			}
		}
		r.Leave()
		*(*sliceHeader)(dst) = sliceHeader{data: out, len: count, cap: count}
		return nil
	}
}

// readPtrSliceOf returns the reader of the slice type t of pointers to the structs that plan reads,
// which values is a slice type of: an array, or null as nil, whose elements may be null. The
// elements share one array, as long as the bytes left could hold them: an array of nulls or empty
// maps gets no more memory than fxamacker would give it, give or take a small factor.
func readPtrSliceOf(plan *structPlan, t, values reflect.Type) fieldReader {
	size, pointer, elem := values.Elem().Size(), runtimeTypeOf(t.Elem()), runtimeTypeOf(values.Elem())
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			*(*sliceHeader)(dst) = sliceHeader{}
			return nil
		}
		count, err := r.ArrayLen()
		if err != nil {
			return err
		}
		out := unsafe.Slice((*unsafe.Pointer)(newArray(pointer, count)), count)
		shared := min(count, r.Len()/minPtrSliceElement)
		array := newArray(elem, shared)
		used := 0
		for i := range out {
			if r.Null() {
				continue
			}
			var value unsafe.Pointer
			if used < shared {
				value = unsafe.Add(array, uintptr(used)*size)
				used++
			} else {
				value = unsafe_New(elem)
			}
			if err := plan.read(r, value); err != nil {
				return err
			}
			out[i] = value
		}
		r.Leave()
		data := unsafe.Pointer(unsafe.SliceData(out))
		*(*sliceHeader)(dst) = sliceHeader{data: data, len: count, cap: count}
		return nil
	}
}

// readByteArray returns the reader of a byte array of n bytes, stored as a byte string of exactly n
// bytes. Null leaves it as it was.
func readByteArray(n int) fieldReader {
	return func(r *cursor.Cursor, dst unsafe.Pointer) error {
		if r.Null() {
			return nil
		}
		return r.BytesInto(unsafe.Slice((*byte)(dst), n))
	}
}

// readFeltInPlace reads a felt held by value. Null leaves it as it was.
func readFeltInPlace(r *cursor.Cursor, dst *felt.Felt) error {
	if r.Null() {
		return nil
	}
	return r.Limbs((*[cursor.Limbs]uint64)(dst))
}
