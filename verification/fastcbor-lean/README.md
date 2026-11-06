# Goose → Perennial → Lean 4 assessment of fastcbor

The ordinary parser code is a plausible verification target. The complete fastcbor
library cannot currently be verified unchanged with the upstream toolchain. The
main obstacles are reflection, Go runtime entry points, and memory reinterpretation
of functions, interfaces, pointers and slice headers. They are not a general lack
of generics, closures, loops or maps.

This assessment changes no production source and adds no trusted semantics,
replacement runtime functions, or assumed decoder contracts.

## Revisions and reproduction

- Juno branch `origin/danielntmd/cbor-fastcbor`: `a4a67242cc67f3855915a97bf4e79a97d217fae4`.
- Perennial **lean** branch: `84d5380639a38f56cd517f1e81246fad7be5b21b`.
- Go `go1.27.0`; Lean `leanprover/lean4:v4.34.1`.
- Worktree branch: `danielntmd/cbor-fastcbor-lean`.

From this directory:

```sh
./bootstrap.sh
./check.py
```

`ELAN_HOME` can point to an existing elan installation; otherwise bootstrap installs
one under `.tools/elan`. The assessed run reused the earlier POC's installation:

```sh
ELAN_HOME=/tmp/juno-cbor-lean-poc-608d97/worktree/verification/cbor/.tools/elan ./check.py
```

`check.py` verifies production source byte-for-byte against the pinned Juno commit.
It records source hashes, actual exit codes, and diagnostics in `results/`.
Expected translation failures are checked as failures: a successful assessment run
does **not** mean the whole decoder translated or was verified. Generated Lean
files and downloaded tools are ignored and reproducible.

## What actually ran

1. Existing cursor and compatibility Go tests passed (`-short`); fastcbor itself
   has no package-local tests. All ten standalone feature packages compile as Go.
2. Unmodified full cursor and fastcbor translation failed. Goose emitted one
   cursor diagnostic and ten fastcbor diagnostics.
3. Excluding the eleven affected function declarations makes both packages pass
   Goose and proofgen. Those exclusions are diagnostic only; callees remain
   unresolved, so this is not a closed or verified program. We did not Lean-build
   that full filtered output or its entire dependency graph.
4. Five feature packages pass Goose, proofgen, and Lean elaboration. Five isolate
   translation failures. The passing packages deliberately include reflection and
   opaque-memory casts: **Lean accepts their generated definitions without
   supplying the missing execution rules**.
5. A selection of actual production cursor declarations passes Goose, proofgen,
   and Lean elaboration: cursor construction/access, head parsing, signed/unsigned
   integers, booleans, container lengths, tags, null, and their diagnostic helpers.
   The exact selection is `core-config/.../cursor.toml`. It includes slow-path
   definitions, but does not establish their functional correctness.
6. Two small Lean lemmas establish the 32-byte, alignment-8 plain layout of
   `[4]uint64`, and the semantic step for converting an `unsafe.Pointer` to a
   pointer to that array. Their transitive axiom lists contain only `propext`,
   `Classical.choice`, and `Quot.sound`. The semantic-step lemma assumes
   Perennial's `go.Semantics`; it proves neither memory ownership nor decoding.

**No new decoder correctness theorem was proved.** The build contains upstream
warnings for axiomatized declarations. Our generated files contain no `sorry`,
and the two boundary lemmas do not depend on `sorryAx`; this is not an axiom audit
of every upstream declaration. The earlier POC theorem is about a different
pinned source/toolchain and does not transfer automatically.

## Feature assessment

| Feature used here | Observed support | Consequence |
| --- | --- | --- |
| Generic functions, `~uint` / `~uint32` / `~[4]uint64` constraints, callbacks, closures | Used production forms translate; probes Lean-build | Suitable building blocks; concrete proof obligations remain |
| Structs, arrays, slices, ordinary maps, integer range loops, recursion, masks/shifts/wrapping arithmetic | Production translation accepts them | No syntax-level reason to rewrite parser/state-machine logic |
| Plain felt-pointer casts; `unsafe.Add` and `unsafe.Slice` over plain elements | Translation plus byte-layout semantics; small felt-layout lemma checked | Much better than our older assessment; ownership and bounds still need proofs |
| `unsafe.SliceData` | Rejected | Affects `cursor.LimbSlice` and `readPtrSliceOf` |
| `uintptr` `i++` | Rejected | Affects `isZero`; ordinary uintptr arithmetic itself has semantics |
| `unsafe.Sizeof` / `Alignof` of generic `T` | Rejected in isolated probes | Additional blocker inside `isZero`, hidden by the first diagnostic; concrete constant sizes do translate |
| Indexing a type parameter constrained by `~[4]uint64` | Rejected in isolated probe | A limitation to consider when refactoring; current felt code casts to a concrete array instead |
| `defer` / direct deferred-literal `recover()` | Actual `structOfOrNil` and probe translate | Translation is not full panic/recovery fidelity; upstream documents limitations |
| `sync.Mutex`, `RWMutex`, `sync.Pool` | Models/rules exist; Pool probe Lean-builds | Pool is no longer simply missing; a client proof must use its ownership invariant and model assumptions |
| `sync.Map.Load/Store` | Names/types exist, but implementations are not selected by `sync.toml` | Reader cache needs additional support or a different implementation |
| `reflect.TypeFor`, field inspection, `StructOf`, `NewAt`, `IsZero` | Calls translate; reflection implementation excluded upstream | Planner and API precheck cannot execute/prove through those calls as supplied |
| Eight `go:linkname` runtime functions | Rejected as bodyless external functions | Must verify/provide justified contracts for allocation, clearing and map-slot access, or remove their use |
| Function, interface, slice-header and pointer-memory reinterpretation | Often translates and even Lean-builds; non-null unsafe conversion to a non-plain target has no rule | Fundamental semantic boundary, not fixed by completing the translator |
| Big integers, UTF-8 and some collection/string helpers | Needed implementations are absent/incomplete in bundled dependency configurations | Additional dependency translation/proofs or explicitly scoped contracts required |
| `fmt.Errorf("%w", ...)` | Bundled model does not implement error wrapping | Current model cannot establish the decoder's `errors.Is` behavior |

The ten feature probes are not a statistical sample and their pass ratio is not
"verification coverage." Likewise, eleven rejected declarations are not the full
list of unverified behavior. A single unresolved operation can block the public
entry point.

## Where the full API gets stuck

`UnmarshalFirst[T]` starts with `isZero`: raw reads across a generic value followed
by reflection. Raw bytes cannot be read from the model's opaque pointer/string/
slice cells as though they were ordinary bytes. Next `readerFor` uses reflection
and a `sync.Map` cache. Planned readers then use runtime allocation/map calls and
unsafe field writes. These barriers exist even if the cursor is proved correct.

Concrete sites:

- `fastcbor.go:65`: generic raw-memory zero test and reflection fallback.
- `alloc.go:17`: extracting a runtime type pointer from `reflect.Type`'s representation.
- `alloc.go:24–64`: eight runtime/reflect entry points with no Go body.
- `plan.go:34`: `asFieldReader` reinterprets one function type as another.
- `generic.go:85`: writing interface table/data words directly.
- `plan.go:679` and its readers: manufacturing a slice header, including pointer arrays.
- `cursor/felt.go:142`: `unsafe.SliceData` prevents whole-cursor translation.

Perennial's new plain-memory model handles integers, floats, nonempty arrays of
plain elements, and restricted structs without padding. It keeps pointers,
functions, interfaces, strings, slices, maps and bools opaque. Merely adding a
model for the eight external functions would not solve all these casts.

## Recommended scope

Start with the cursor's integer/head parser and the typed four-limb reader. Prove
consumption, decoded values, bounds, and destination behavior against an explicit
CBOR subset contract. Fixing/adapting `SliceData` can then open the limb-slice
reader; UTF-8 and error-model support are needed to extend to all skip/text paths.
The proof can leave formatting out only if that exclusion is explicit; it must
not claim error-wrapping compatibility from the current `fmt` model.

For eventual whole-record proofs, typed generated readers would avoid much of the
runtime/reflection boundary. Keeping this planner unchanged would instead require
a substantial extension of Perennial's memory and runtime models. A proved cursor
with a tested planner is a useful intermediate result, but it does not establish
`UnmarshalFirst`/`Unmarshal` equivalence with fxamacker.

## Pinned upstream evidence

- [Plain-memory execution rules](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/Golang/Defn/Plain.lean): unsafe conversion requires a plain target (or nil).
- [Reflection selection](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/Code/reflect.toml): all implementations excluded.
- [Sync selection and Pool modeling](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/Code/sync.toml).
- [Pool proof](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/Proof/sync/Pool__Get.lean).
- [Formatting model](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/TrustedCode/fmt.lean).
- [Big integer selection](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/Perennial/Code/math/big.toml).
- [Known model limitations](https://github.com/mit-pdos/perennial/blob/84d5380639a38f56cd517f1e81246fad7be5b21b/docs/KNOWN_LIMITATIONS.md).
