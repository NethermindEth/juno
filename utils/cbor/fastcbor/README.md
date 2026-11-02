# Fast CBOR reader

Reads the records Juno stores (headers, transactions, receipts, state updates, classes, ...) into Go
values in one pass. It is a decoder only: [fxamacker/cbor](https://github.com/fxamacker/cbor) still
writes every record, in the same format, so the fast path can be removed without migrating anything.

```go
err := fastcbor.Unmarshal(data, &value)          // data holds one item
rest, err := fastcbor.UnmarshalFirst(data, &value) // the bytes after the first item
```

## How it relates to fxamacker

fxamacker is the **writer**, the **fallback** and the **oracle**. A read that fails, a type the planner
cannot read, and a destination that is not the zero value all go to it, and a failure is reported
through `fastcbor.SetLogger`. The tests decode the same bytes both ways and require the same result.

## How a type is read

The first read of a type plans its reader from the type, by the rules fxamacker follows, and caches it.
Nothing about Juno's types is written down here except the felts: a struct is read by the wire names of
its fields (or integer keys), an interface by the tag `cbor.RegisterType` gave its concrete type, a
type that decodes itself through its own `UnmarshalCBOR` or `UnmarshalBinary`, and maps, slices and
pointers by their elements. Types that are felts, or are probed to encode as one, are read in place.

So the package imports only `core/felt`, the fxamacker adapter and the `utils/cbor/internal/cursor`
primitives, and `core` can call it, for the class inside a `DeclaredClassDefinition`, without a cycle.

## Rules

1. A reader never keeps a reference to the input: it copies what it keeps.
2. It writes only the destination, which is the zero value; the caller clears it on a failure.
3. It counts nesting through the cursor, for fxamacker's limit.
4. It never panics or hangs, and never allocates more than the input can back.
5. It matches fxamacker or gives up. A form it does not read is an error that falls back.

## Sharp edges

- The planner uses `unsafe` and `//go:linkname` to the runtime's allocator and map calls
  (`unsafe_New`, `makemap`, `mapassign`, `typedmemclr`). The runtime keeps them for the packages
  that link to them (go.dev/issue/67401), but a new Go version may need a fallback.
- The fast map calls are used only where the compiler would use them: integer keys of 4 or 8 bytes and
  elements of at most 128 bytes (`internal/abi.MapMaxElemBytes`).
- Structs read behind a pointer share one allocation with the small structs they point to, and
  pointers to the same small struct share one. A pointer to one of them keeps the whole block alive.

## Tests

`go test ./utils/cbor/fastcbor/...` compares mainnet records and every rule with fxamacker. Run the
sweep against a mainnet database before a release and when fxamacker is upgraded:
`utils/cbor/fastcbor/sweep/run.sh quick`.
