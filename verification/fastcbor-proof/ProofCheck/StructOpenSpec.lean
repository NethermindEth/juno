import ProofCheck.MapExpectedFailure

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

/-- Failure before entering a typed record map. Null/undefined precede expectation;
    impossible pair counts precede the depth check. -/
inductive StructOpenFailure : Nat → List w8 → FeltFailureReason → Prop where
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 160) wire reason) :
      StructOpenFailure d wire (.expected (W8 160) reason)
  | count {d : Nat} {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (short : tail.length < 2 * uint.Z argument) :
      StructOpenFailure d ((head :: payload) ++ tail) (.skipped .count)
  | depth {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (fit : 2 * uint.Z argument ≤ tail.length) :
      StructOpenFailure 32 ((head :: payload) ++ tail) .depth

/-- Exhaustive classification of opening, not of the map entries. The latter
    still need complete key/value classification and the general loop theorem. -/
theorem struct_open_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) :
    (∃ head tail, wire = head :: tail ∧ (head = W8 246 ∨ head = W8 247)) ∨
    (∃ head payload body count, wire = (head :: payload) ++ body ∧
      HeadArgument head payload count ∧ head &&& W8 224 = W8 160 ∧ d < 32 ∧
      2 * uint.Z count ≤ body.length) ∨
    (∃ reason, StructOpenFailure d wire reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨head, tail, rfl, by simpa using nullish⟩
  · have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
    have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
    rcases expected_classify (W8 160) wire with ⟨head, payload, body, count, split, enc, major⟩ | ⟨reason, failure⟩
    · rw [split]
      by_cases short : (body.length : Int) < 2 * uint.Z count
      · exact Or.inr (Or.inr ⟨.skipped .count, .count enc major short⟩)
      have fit : 2 * uint.Z count ≤ body.length := by omega
      by_cases depth : d < 32
      · exact Or.inr (Or.inl ⟨head, payload, body, count, rfl, enc, major, depth, fit⟩)
      · have heq : d = 32 := by omega
        subst d
        exact Or.inr (Or.inr ⟨.depth, .depth enc major fit⟩)
    · exact Or.inr (Or.inr ⟨.expected (W8 160) reason, .outer hn hu failure⟩)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.struct_open_classify
