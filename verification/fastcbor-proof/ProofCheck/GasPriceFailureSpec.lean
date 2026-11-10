import ProofCheck.GasPriceMapDuplicate
import ProofCheck.GasPriceMapFailure

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

inductive GasPriceFailureReason where
  | inner (reason : FeltFailureReason)
  | duplicate (wei : Bool)

def gasPriceFailureAddr (reason : GasPriceFailureReason) : Loc :=
  match reason with
  | .inner reason => feltFailureAddr (GF := GF) reason
  | .duplicate _ => globalAddr cursor.ErrUnsupported

def GasPriceFailureError (err : Loc) (pos : w64) (reason : GasPriceFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .inner reason => FeltFailureError err pos reason kind
  | .duplicate wei => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(gasPriceFieldName wei)])

/-- Currently closed GasPrice failure families. This relation retains the final
    abstract value because successful prefix fields remain in the destination.
    General key rejection and wider/non-ASCII keys still need additional cases;
    no exhaustive-decoding claim is made for this relation yet. -/
inductive GasPriceFailure : Nat → List w8 → GasPriceValue → GasPriceValue → Nat → Nat → Bool → GasPriceFailureReason → Prop where
  | opening {d : Nat} {wire : List w8} {reason : FeltFailureReason} (initial : GasPriceValue)
      (failure : GasPriceOpenFailure d wire reason) :
      GasPriceFailure d wire initial initial 0 0 false (.inner reason)
  | value {d count offset : Nat} {head : w8} {payload tail : List w8} {reason : FeltFailureReason}
      (initial : GasPriceValue) (entries : List (GasPriceEntry (d + 1))) (key : GasKeyWire)
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
      (depth : d < 32) (more : entries.length < count)
      (fit : 2 * count ≤ (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail).length)
      (fresh : GasEntriesBefore (W64 0) entries key.choice)
      (failure : GasPriceValueFailure (d + 1) key.choice tail offset reason) :
      GasPriceFailure d ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail)
        initial (entries.foldl GasPriceEntry.apply initial)
        (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length)
        (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length + offset)
        true (.inner reason)
  | duplicate {d count : Nat} {head : w8} {payload tail : List w8} {wei : Bool}
      (initial : GasPriceValue) (entries : List (GasPriceEntry (d + 1))) (key : GasKeyWire)
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
      (depth : d < 32) (more : entries.length < count)
      (fit : 2 * count ≤ (entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail).length)
      (beforeDuplicate : GasEntriesBeforeDuplicate (W64 0) entries wei) (choice : key.choice = some wei) :
      GasPriceFailure d ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ key.bytes ++ tail)
        initial (entries.foldl GasPriceEntry.apply initial)
        (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length)
        (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length + key.bytes.length)
        true (.duplicate wei)

/-- Retained progress precedes the diagnostic; both lie inside the unread input
    or at its end. Only the map's own depth entry survives these failures. -/
theorem gasPriceFailure_bounds (d consumed offset : Nat) (entered : Bool) (wire : List w8)
    (initial final : GasPriceValue) (reason : GasPriceFailureReason)
    (failure : GasPriceFailure d wire initial final consumed offset entered reason) (hdepth : d ≤ 32) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + (if entered then 1 else 0) ≤ 32 := by
  cases failure with
  | opening initial failure => simp; omega
  | value initial entries key enc major depth more fit fresh failure =>
    have hoff := gasPriceValueFailure_offset _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons, ite_true]
    omega
  | duplicate initial entries key enc major depth more fit beforeDuplicate choice =>
    simp only [List.length_append, List.length_cons, ite_true]
    omega

/-- Specified errors cannot be intercepted by the nullable slot wrapper. -/
theorem gasPriceFailure_not_nullish (d consumed offset : Nat) (entered : Bool) (wire : List w8)
    (initial final : GasPriceValue) (reason : GasPriceFailureReason)
    (failure : GasPriceFailure d wire initial final consumed offset entered reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | opening initial failure =>
    cases failure with
    | outer hn hu failure => exact ⟨hn, hu⟩
    | count enc major short =>
      constructor <;> intro h <;> have hh := Option.some.inj h <;> rw [hh] at major <;> contradiction
    | depth enc major fit =>
      constructor <;> intro h <;> have hh := Option.some.inj h <;> rw [hh] at major <;> contradiction
  | value initial entries key enc major depth more fit fresh failure =>
    constructor <;> intro h <;> have hh := Option.some.inj h <;> rw [hh] at major <;> contradiction
  | duplicate initial entries key enc major depth more fit beforeDuplicate choice =>
    constructor <;> intro h <;> have hh := Option.some.inj h <;> rw [hh] at major <;> contradiction

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceFailure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceFailure_not_nullish
