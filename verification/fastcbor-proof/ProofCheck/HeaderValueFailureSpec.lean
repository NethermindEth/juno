import ProofCheck.HeaderMapSpec
import ProofCheck.HeaderFieldFailure

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

def HeaderEntriesBefore {d : Nat} (seen : w64) : List (HeaderEntry d) → Option HeaderField → Prop
  | [], last => HeaderKeyFresh seen last
  | entry :: rest, last => HeaderKeyFresh seen entry.key.choice ∧
      HeaderEntriesBefore (headerKeySeen seen entry.key.choice) rest last

inductive HeaderValueFailureReason where
  | felt (reason : FeltFailureReason)
  | string (reason : StringFailureReason)
  | gas (reason : GasPriceFailureReason)

def stringFailureKind (reason : StringFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .expected reason => expectedFailureKind reason kinds
  | .truncated => kinds.truncatedKind

def headerValueFailureKind (reason : HeaderValueFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .felt reason => feltFailureKind reason kinds
  | .string reason => stringFailureKind reason kinds
  | .gas reason => gasPriceFailureKind reason kinds

def HeaderValueFailureError (err : Loc) (pos : w64) (reason : HeaderValueFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason kind
  | .string reason => StringFailureError err pos reason kind
  | .gas reason => GasPriceFailureError err pos reason kind

/-- The selected field's independent failure relation. Consumed bytes and error
    offsets differ when a child rolls back; retained depth counts entered outer
    containers. A fresh GasPrice may keep private partial contents but publishes
    no failed replacement. String and GasPrice relations retain their documented
    coverage limits; no exhaustive Header classification is asserted. -/
inductive HeaderValueFailure (d : Nat) : Option HeaderField → List w8 → Nat → Nat → Nat → HeaderValueFailureReason → Prop where
  | felt (field : HeaderFeltField) {wire : List w8} {offset : Nat} {reason : FeltFailureReason}
      (failure : FeltFailure d wire offset reason) :
      HeaderValueFailure d (some (headerFeltKey field)) wire 0 offset 0 (.felt reason)
  | scalar (field : HeaderScalarField) {wire : List w8} {reason : ExpectedFailureReason}
      (failure : ScalarFailure wire reason) :
      HeaderValueFailure d (some (headerScalarKey field)) wire 0 0 0 (.felt (.expected (W8 0) reason))
  | signatures {wire : List w8} {consumed offset extraDepth : Nat} {reason : FeltFailureReason}
      (failure : SignaturesFailure d wire consumed offset extraDepth reason) :
      HeaderValueFailure d (some .signatures) wire consumed offset extraDepth (.felt reason)
  | string {wire : List w8} {reason : StringFailureReason} (failure : StringFailure wire reason) :
      HeaderValueFailure d (some .protocolVersion) wire 0 0 0 (.string reason)
  | gas (second : Bool) (final : GasPriceValue) {wire : List w8} {consumed offset : Nat} {entered : Bool}
      {reason : GasPriceFailureReason} (failure : GasPriceFailure d wire ⟨none, none⟩ final consumed offset entered reason) :
      HeaderValueFailure d (some (headerGasKey second)) wire consumed offset (if entered then 1 else 0) (.gas reason)
  | ignored {wire : List w8} {offset : Nat} {reason : SkipFailureReason} (failure : SkipFailure d wire offset reason) :
      HeaderValueFailure d none wire 0 offset 0 (.felt (.skipped reason))

theorem stringErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : StringFailureReason) :
    FeltErrorGlobals kinds ⊢ (stringFailureAddr (GF := GF) reason ↦□ stringFailureKind reason kinds : IProp GF) := by
  cases reason with
  | expected reason =>
    exact feltErrorGlobals_lookup kinds (.expected (W8 96) reason)
  | truncated =>
    unfold FeltErrorGlobals SkipErrorGlobals stringFailureAddr stringFailureKind
    iintro ⟨⟨-, #Htruncated, -, -⟩, -⟩
    iexact Htruncated

theorem headerValueFailure_bounds (d consumed offset extraDepth : Nat) (choice : Option HeaderField)
    (wire : List w8) (reason : HeaderValueFailureReason) (hdepth : d ≤ 32)
    (failure : HeaderValueFailure d choice wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | felt field failure =>
    have := feltFailure_offset _ _ _ _ failure
    simp only [Nat.add_zero]; omega
  | scalar field failure => simp; omega
  | signatures failure =>
    have hb := signaturesFailure_bounds _ _ _ _ _ _ failure
    have hd := signaturesFailure_depth _ _ _ _ _ _ hdepth failure
    exact ⟨hb.1, hb.2, hd⟩
  | string failure => simp; omega
  | gas second final failure =>
    have hb := gasPriceFailure_bounds _ _ _ _ _ _ _ _ failure hdepth
    exact hb
  | ignored failure =>
    have := skipFailure_offset _ _ _ _ failure
    simp only [Nat.add_zero]; omega

/-- A whole Header map reaching a specified failing value after a successful
    prefix. Count/depth precedence and key freshness are explicit input facts. -/
structure HeaderValueFailureWire (depth : Nat) where
  head : w8
  payload : List w8
  count : Nat
  entries : List (HeaderEntry (depth + 1))
  key : HeaderKeyWire
  tail : List w8
  consumed : Nat
  offset : Nat
  extraDepth : Nat
  reason : HeaderValueFailureReason
  enc : HeadArgument head payload (W64 count)
  major : head &&& W8 224 = W8 160
  bound : depth < 32
  more : entries.length < count
  fit : 2 * count ≤ (entries.flatMap HeaderEntry.bytes ++ key.bytes ++ tail).length
  fresh : HeaderEntriesBefore (W64 0) entries key.choice
  failure : HeaderValueFailure (depth + 1) key.choice tail consumed offset extraDepth reason

def HeaderValueFailureWire.bytes {d : Nat} (item : HeaderValueFailureWire d) : List w8 :=
  (item.head :: item.payload) ++ item.entries.flatMap HeaderEntry.bytes ++ item.key.bytes ++ item.tail

def HeaderValueFailureWire.valueStart {d : Nat} (item : HeaderValueFailureWire d) : Nat :=
  1 + item.payload.length + (item.entries.flatMap HeaderEntry.bytes).length + item.key.bytes.length

def HeaderValueFailureWire.errorOffset {d : Nat} (item : HeaderValueFailureWire d) : Nat :=
  item.valueStart + item.offset

theorem headerValueFailureWire_bounds (d : Nat) (item : HeaderValueFailureWire d) :
    item.valueStart + item.consumed ≤ item.errorOffset ∧ item.errorOffset ≤ item.bytes.length ∧
      d + 1 + item.extraDepth ≤ 32 := by
  have hb := headerValueFailure_bounds (d + 1) item.consumed item.offset item.extraDepth item.key.choice
    item.tail item.reason (by have := item.bound; omega) item.failure
  simp only [HeaderValueFailureWire.valueStart, HeaderValueFailureWire.errorOffset,
    HeaderValueFailureWire.bytes, List.length_append, List.length_cons]
  omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stringErrorGlobals_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.headerValueFailure_bounds

#print axioms Perennial.github_com.NethermindEth.juno.core.headerValueFailureWire_bounds
