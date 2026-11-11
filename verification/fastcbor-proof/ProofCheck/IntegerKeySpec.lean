import ProofCheck.IntegerKey

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

def integerKeyMajor (wire : List w8) : Option w8 := wire.head?.map (fun b => b &&& W8 224)

/-- Numeric unsigned keys select fields; supported negative/text keys are consumed
    as unknown. Their values are handled separately by the enclosing record. -/
inductive IntegerKeyValid : List w8 → Option w64 → Prop where
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (bound : uint.Z argument < 2 ^ 63) : IntegerKeyValid (head :: payload) (some argument)
  | unknown {wire : List w8} {decoded : Option GoString}
      (valid : KeyValid wire decoded) (major : integerKeyMajor wire ≠ some (W8 0)) :
      IntegerKeyValid wire none

inductive IntegerKeyFailureReason where
  | unsigned (reason : ExpectedFailureReason)
  | overflow
  | other (reason : KeyFailureReason)

inductive IntegerKeyRejected : List w8 → Nat → IntegerKeyFailureReason → Prop where
  | unsigned {head : w8} {tail : List w8} {reason : ExpectedFailureReason}
      (major : head &&& W8 224 = W8 0) (failure : ExpectedFailure (W8 0) (head :: tail) reason) :
      IntegerKeyRejected (head :: tail) 0 (.unsigned reason)
  | overflow {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
      (bound : 2 ^ 63 ≤ uint.Z argument) :
      IntegerKeyRejected ((head :: payload) ++ tail) (head :: payload).length .overflow
  | other {wire : List w8} {reason : KeyFailureReason}
      (major : integerKeyMajor wire ≠ some (W8 0)) (failure : KeyRejected wire reason) :
      IntegerKeyRejected wire 0 (.other reason)

theorem integerKeyMajor_prefix (item rest : List w8) (nonempty : 0 < item.length) :
    integerKeyMajor (item ++ rest) = integerKeyMajor item := by
  cases item with
  | nil => simp at nonempty
  | cons _ _ => rfl

theorem integerKey_classify (wire : List w8) :
    (∃ item rest key, wire = item ++ rest ∧ IntegerKeyValid item key) ∨
    (∃ consumed reason, IntegerKeyRejected wire consumed reason) := by
  by_cases unsigned : integerKeyMajor wire = some (W8 0)
  · cases wire with
    | nil => simp [integerKeyMajor] at unsigned
    | cons head tail =>
      have major : head &&& W8 224 = W8 0 := Option.some.inj unsigned
      cases head_classify head tail with
      | fault reason fault => exact Or.inr ⟨0, .unsigned (.head head reason), .unsigned major (.head tail fault)⟩
      | truncated size info sizeEq short => exact Or.inr ⟨0, .unsigned .incomplete, .unsigned major (.incomplete tail info sizeEq short)⟩
      | encoded payload rest argument split enc =>
        subst tail
        by_cases bound : uint.Z argument < 2 ^ 63
        · exact Or.inl ⟨head :: payload, rest, some argument, rfl, .unsigned enc major bound⟩
        · exact Or.inr ⟨_, .overflow, .overflow enc major (by omega)⟩
  · rcases key_classify wire with ⟨item, rest, decoded, split, valid⟩ | ⟨reason, failure⟩
    · refine Or.inl ⟨item, rest, none, split, .unknown valid ?_⟩
      rw [split, integerKeyMajor_prefix item rest (keyValid_nonempty item decoded valid)] at unsigned
      exact unsigned
    · exact Or.inr ⟨0, .other reason, .other unsigned failure⟩

theorem integerKeyRejected_bounds (wire : List w8) (consumed : Nat) (reason : IntegerKeyFailureReason)
    (failure : IntegerKeyRejected wire consumed reason) : consumed ≤ wire.length := by
  cases failure with
  | unsigned _ _ | other _ _ => omega
  | overflow _ _ _ => simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyMajor_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKey_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.integerKeyRejected_bounds
