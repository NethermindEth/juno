import ProofCheck.HeaderFieldSuccess
import ProofCheck.HeaderGasFresh
import ProofCheck.HeaderString

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A scalar encoding carries either a replacement word or the nullish policy
    of preserving the previous value. This is independent of the Header heap. -/
structure HeaderScalarWire where
  bytes : List w8
  value : Option w64
  valid : ∀ old, ScalarValid old bytes (value.getD old)

structure HeaderSignaturesWire (depth : Nat) where
  bytes : List w8
  value : Option (List SignatureValue)
  valid : SignaturesValid depth bytes value

/-- The currently closed successful price encodings. Child maps start from
    zero because a known Header key is visited at most once. -/
inductive HeaderGasWire (depth : Nat) where
  | nullish (head : w8) (hn : head = W8 246 ∨ head = W8 247)
  | map (head : w8) (payload : List w8) (entries : List (GasPriceEntry (depth + 1)))
      (enc : HeadArgument head payload (W64 entries.length))
      (major : head &&& W8 224 = W8 160) (bound : depth < 32)
      (fresh : GasEntriesFresh (W64 0) entries)

def HeaderGasWire.bytes {d : Nat} : HeaderGasWire d → List w8
  | .nullish head _ => [head]
  | .map head payload entries _ _ _ _ => (head :: payload) ++ entries.flatMap GasPriceEntry.bytes

def HeaderGasWire.value {d : Nat} : HeaderGasWire d → Option GasPriceValue
  | .nullish _ _ => none
  | .map _ _ entries _ _ _ _ => some (entries.foldl GasPriceEntry.apply ⟨none, none⟩)

/-- Entries for the actual Header-loop composition milestone. This relation
    deliberately uses the currently proved key families and nullish protocol
    strings only. It is not the final all-encoding Header specification.
    All fifteen field identities participate, with general felt, unsigned,
    signature, GasPrice and ignored nested values. -/
inductive HeaderEntry (depth : Nat) where
  | felt (field : HeaderFeltField) (key : HeaderKeyWire)
      (choice : key.choice = some (headerFeltKey field)) (item : FeltWire depth)
  | scalar (field : HeaderScalarField) (key : HeaderKeyWire)
      (choice : key.choice = some (headerScalarKey field)) (item : HeaderScalarWire)
  | signatures (key : HeaderKeyWire) (choice : key.choice = some .signatures) (item : HeaderSignaturesWire depth)
  | versionNull (key : HeaderKeyWire) (choice : key.choice = some .protocolVersion)
      (head : w8) (hn : head = W8 246 ∨ head = W8 247)
  | gas (second : Bool) (key : HeaderKeyWire) (choice : key.choice = some (headerGasKey second)) (item : HeaderGasWire depth)
  | ignored (key : HeaderKeyWire) (choice : key.choice = none) (wire : List w8) (valid : SkipValid depth wire)

def HeaderEntry.key {d : Nat} : HeaderEntry d → HeaderKeyWire
  | .felt _ key _ _ => key
  | .scalar _ key _ _ => key
  | .signatures key _ _ => key
  | .versionNull key _ _ _ => key
  | .gas _ key _ _ => key
  | .ignored key _ _ _ => key

def HeaderEntry.valueBytes {d : Nat} : HeaderEntry d → List w8
  | .felt _ _ _ item => item.bytes
  | .scalar _ _ _ item => item.bytes
  | .signatures _ _ item => item.bytes
  | .versionNull _ _ head _ => [head]
  | .gas _ _ _ item => item.bytes
  | .ignored _ _ wire _ => wire

def HeaderEntry.bytes {d : Nat} (entry : HeaderEntry d) : List w8 := entry.key.bytes ++ entry.valueBytes

def HeaderEntry.apply {d : Nat} (old : HeaderValue) : HeaderEntry d → HeaderValue
  | .felt field _ _ item => headerSetFeltValue field old item.value
  | .scalar field _ _ item => headerSetScalarValue field old (item.value.getD (headerScalarValue field old))
  | .signatures _ _ item => {old with signatures := item.value}
  | .versionNull _ _ _ _ => old
  | .gas second _ _ item => headerSetGasValue second old item.value
  | .ignored _ _ _ _ => old

def HeaderEntriesFresh {d : Nat} (seen : w64) : List (HeaderEntry d) → Prop
  | [] => True
  | entry :: rest => HeaderKeyFresh seen entry.key.choice ∧ HeaderEntriesFresh (headerKeySeen seen entry.key.choice) rest

/-- A complete successful map in the current composition family. The explicit
    depth, freshness and encoding witnesses are application facts, not helper
    correctness assumptions. -/
structure HeaderMapWire (depth : Nat) where
  head : w8
  payload : List w8
  entries : List (HeaderEntry (depth + 1))
  enc : HeadArgument head payload (W64 entries.length)
  major : head &&& W8 224 = W8 160
  bound : depth < 32
  fresh : HeaderEntriesFresh (W64 0) entries

def HeaderMapWire.bytes {d : Nat} (item : HeaderMapWire d) : List w8 :=
  (item.head :: item.payload) ++ item.entries.flatMap HeaderEntry.bytes

def HeaderMapWire.value {d : Nat} (item : HeaderMapWire d) : HeaderValue :=
  item.entries.foldl HeaderEntry.apply headerZeroValue

theorem headerMapWire_length (d : Nat) (item : HeaderMapWire d) :
    item.bytes.length = 1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length := by
  simp only [HeaderMapWire.bytes, List.length_append, List.length_cons]
  omega

theorem headerScalarWire_nonempty (item : HeaderScalarWire) : 0 < item.bytes.length := by
  have nonempty (old value : w64) (bytes : List w8) (valid : ScalarValid old bytes value) : 0 < bytes.length := by
    cases valid <;> simp only [List.length_cons, List.length_nil] <;> omega
  exact nonempty _ _ _ (item.valid (W64 0))

theorem headerSignaturesWire_nonempty (d : Nat) (item : HeaderSignaturesWire d) : 0 < item.bytes.length := by
  rcases item with ⟨bytes, value, valid⟩
  cases valid <;> simp only [List.length_cons, List.length_nil, List.length_append] <;> omega

theorem headerGasWire_nonempty (d : Nat) (item : HeaderGasWire d) : 0 < item.bytes.length := by
  cases item <;> simp only [HeaderGasWire.bytes, List.length_cons, List.length_nil, List.length_append] <;> omega

theorem headerEntry_length (d : Nat) (entry : HeaderEntry d) : 2 ≤ entry.bytes.length := by
  have hk := headerKey_nonempty entry.key
  cases entry with
  | felt field key choice item =>
    have hv := feltWire_nonempty d item
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append] at *
    omega
  | scalar field key choice item =>
    have hv := headerScalarWire_nonempty item
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append] at *
    omega
  | signatures key choice item =>
    have hv := headerSignaturesWire_nonempty d item
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append] at *
    omega
  | versionNull key choice head hn =>
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append,
      List.length_cons, List.length_nil] at *
    omega
  | gas second key choice item =>
    have hv := headerGasWire_nonempty d item
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append] at *
    omega
  | ignored key choice wire valid =>
    have hv := skipValid_nonempty d wire valid
    simp only [HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes, List.length_append] at *
    omega

theorem headerEntries_count_bound (d : Nat) (entries : List (HeaderEntry d)) :
    2 * entries.length ≤ (entries.flatMap HeaderEntry.bytes).length := by
  induction entries with
  | nil => simp
  | cons entry entries ih =>
    have := headerEntry_length d entry
    simp only [List.length_cons, List.flatMap_cons, List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerScalarWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerSignaturesWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerGasWire_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntry_length
#print axioms Perennial.github_com.NethermindEth.juno.core.headerEntries_count_bound

#print axioms Perennial.github_com.NethermindEth.juno.core.headerMapWire_length
