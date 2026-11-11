import ProofCheck.IntsReject

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

inductive IntElementFailureReason where
  | unsignedOverflow
  | signed (reason : Int64FailureReason)

/-- An oversized unsigned value consumes its encoded head and returns the bare
    type sentinel. Other failures are reported by the signed fallback at the
    original element position, after the unsigned attempt has rolled back. -/
inductive IntElementRejected : List w8 → Nat → IntElementFailureReason → Prop where
  | overflow {head : w8} {payload : List w8} {argument : w64} (tail : List w8)
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (bound : 2 ^ 63 ≤ uint.Z argument) :
      IntElementRejected ((head :: payload) ++ tail) (1 + payload.length) .unsignedOverflow
  | fallback {wire : List w8} {reason : Int64FailureReason} (unsignedReason : ExpectedFailureReason)
      (unsignedFailure : ExpectedFailure (W8 0) wire unsignedReason)
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (signedFailure : Int64Rejected wire reason) : IntElementRejected wire 0 (.signed reason)

def IntElementFailureError (result : GoInterface) (pos : w64)
    (reason : IntElementFailureReason) (k : BigIntErrorKinds) : IProp GF :=
  match reason with
  | .unsignedOverflow => iprop(⌜result = bigIntCategory k.typeError⌝)
  | .signed reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      Int64FailureError err pos reason k)

/-- Every element input either contains one independently specified value or
    has a specified failure, including the unsigned/signed overflow asymmetry. -/
theorem intElement_classify (wire : List w8) :
    (∃ (item : IntElementWire) (rest : List w8), wire = item.bytes ++ rest) ∨
    (∃ consumed reason, IntElementRejected wire consumed reason) := by
  cases wire with
  | nil => exact Or.inr ⟨0, .signed .eof, .fallback .eof .eof (by simp) (by simp) .eof⟩
  | cons head tail =>
    by_cases hn : head = W8 246 ∨ head = W8 247
    · exact Or.inl ⟨.nullish head hn, tail, rfl⟩
    have notNull : head ≠ W8 246 := fun h => hn (Or.inl h)
    have notUndefined : head ≠ W8 247 := fun h => hn (Or.inr h)
    cases head_classify head tail with
    | fault reason fault =>
      exact Or.inr ⟨0, .signed (.head head reason), .fallback (.head head reason) (.head tail fault)
        (fun h => notNull (Option.some.inj h)) (fun h => notUndefined (Option.some.inj h)) (.head tail fault)⟩
    | truncated size info sizeEq short =>
      exact Or.inr ⟨0, .signed .incomplete, .fallback .incomplete (.incomplete tail info sizeEq short)
        (fun h => notNull (Option.some.inj h)) (fun h => notUndefined (Option.some.inj h)) (.incomplete tail info sizeEq short)⟩
    | encoded payload rest argument split enc =>
      subst tail
      by_cases hu : head &&& W8 224 = W8 0
      · by_cases bound : uint.Z argument < 2 ^ 63
        · exact Or.inl ⟨.unsigned head payload argument enc hu bound, rest, rfl⟩
        · exact Or.inr ⟨_, .unsignedOverflow, .overflow rest enc hu (by omega)⟩
      · by_cases negative : head &&& W8 224 = W8 32
        · have ht : head &&& W8 224 ≠ W8 192 := by rw [negative]; decide
          by_cases bound : uint.Z argument < 2 ^ 63
          · exact Or.inl ⟨.negative head payload argument enc negative bound, rest, rfl⟩
          · exact Or.inr ⟨0, .signed .overflow, .fallback (.typeMismatch head)
              (.typeMismatch rest enc hu ht) (fun h => notNull (Option.some.inj h))
              (fun h => notUndefined (Option.some.inj h)) (.overflow rest enc (Or.inr negative) (by omega))⟩
        · by_cases tag : head &&& W8 224 = W8 192
          · exact Or.inr ⟨0, .signed .tag, .fallback .tag (.tag rest enc hu tag)
              (fun h => notNull (Option.some.inj h)) (fun h => notUndefined (Option.some.inj h)) (.tag rest enc tag)⟩
          · exact Or.inr ⟨0, .signed (.typeMismatch head), .fallback (.typeMismatch head)
              (.typeMismatch rest enc hu tag) (fun h => notNull (Option.some.inj h))
              (fun h => notUndefined (Option.some.inj h)) (.typeMismatch rest enc hu negative tag)⟩

theorem intElementRejected_bounds (wire : List w8) (consumed : Nat) (reason : IntElementFailureReason)
    (failure : IntElementRejected wire consumed reason) : consumed ≤ wire.length := by
  cases failure with
  | overflow _ _ _ _ => simp only [List.length_append, List.length_cons]; omega
  | fallback _ _ _ _ _ => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intElement_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementRejected_bounds
