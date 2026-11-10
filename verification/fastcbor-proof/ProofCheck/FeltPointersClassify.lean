import ProofCheck.FeltPointersFailureSound

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

/-- Any requested sequence of nullable felts either supplies all elements or
    has a valid prefix followed by a specified child failure. -/
theorem felt_items_classify (d n : Nat) (wire : List w8) (hdepth : d ≤ 32) :
    (∃ (items : List (FeltWire d)) (rest : List w8),
      wire = items.flatMap FeltWire.bytes ++ rest ∧ items.length = n) ∨
    (∃ (items : List (FeltWire d)) (tail : List w8) (offset : Nat) (reason : FeltFailureReason),
      wire = items.flatMap FeltWire.bytes ++ tail ∧ items.length < n ∧ FeltFailure d tail offset reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, rfl, rfl⟩
  | succ n ih =>
    rcases felt_classify d wire hdepth with ⟨bytes, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · let item : FeltWire d := ⟨bytes, value, valid⟩
      rcases ih rest with ⟨items, tail, htail, hcount⟩ | ⟨items, tail, offset, reason, htail, hcount, failure⟩
      · refine Or.inl ⟨item :: items, tail, ?_, by simp only [List.length_cons, hcount]⟩
        simpa only [item, List.flatMap_cons, List.append_assoc, htail] using split
      · refine Or.inr ⟨item :: items, tail, offset, reason, ?_, by simp only [List.length_cons]; omega, failure⟩
        simpa only [item, List.flatMap_cons, List.append_assoc, htail] using split
    · exact Or.inr ⟨[], wire, offset, reason, rfl, by simp, failure⟩

def FeltPointersClassified (d : Nat) (wire : List w8) : Prop :=
  (∃ item rest value, wire = item ++ rest ∧ FeltPointersValid d item value) ∨
  (∃ consumed offset entered reason, FeltPointersFailure d wire consumed offset entered reason)

/-- All array inputs are classified with count rejection taking precedence over
    exhausted depth, followed by exhaustive nullable-child classification. -/
theorem feltPointers_array_classify (d : Nat) (head : w8) (payload body : List w8) (count : w64)
    (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128)
    (hdepth : d ≤ 32) : FeltPointersClassified d ((head :: payload) ++ body) := by
  by_cases short : (body.length : Int) < uint.Z count
  · exact Or.inr ⟨0, 0, false, .skipped .count, .count enc major short⟩
  have fit : uint.Z count ≤ body.length := by omega
  by_cases depth : d < 32
  · have hc : 0 ≤ uint.Z count := by word
    let n := (uint.Z count).toNat
    have hn : (n : Int) = uint.Z count := by dsimp only [n]; omega
    have hnword : W64 n = count := by word
    rcases felt_items_classify (d + 1) n body (by omega) with
      ⟨items, rest, split, hcount⟩ | ⟨items, tail, offset, reason, split, hcount, failure⟩
    · refine Or.inl ⟨(head :: payload) ++ items.flatMap FeltWire.bytes, rest, some (items.map FeltWire.value), ?_,
        .array items enc major (by omega) depth⟩
      rw [split, List.append_assoc]
    · refine Or.inr ⟨1 + payload.length + (items.flatMap FeltWire.bytes).length,
        1 + payload.length + (items.flatMap FeltWire.bytes).length + offset, true, reason, ?_⟩
      rw [split, ← List.append_assoc]
      exact .child items (by simpa only [hnword] using enc) major depth hcount
        (by rw [← split]; omega) failure
  · have heq : d = 32 := by omega
    subst d
    exact Or.inr ⟨0, 0, false, .depth, .depth enc major fit⟩

/-- Every finite input at supported nesting depth has a specified nullable-array
    success with an exact suffix, or a specified failure and retained cursor.
    Null/undefined precede array expectation and count/depth validation. -/
theorem feltPointers_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) :
    FeltPointersClassified d wire := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail =>
      have hn : head = W8 246 ∨ head = W8 247 := by simpa using nullish
      exact Or.inl ⟨[head], tail, none, rfl, .nullish hn⟩
  · have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
    have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
    rcases expected_classify (W8 128) wire with ⟨head, payload, rest, count, split, enc, major⟩ | ⟨reason, failure⟩
    · rw [split]
      exact feltPointers_array_classify d head payload rest count enc major hdepth
    · exact Or.inr ⟨0, 0, false, .expected (W8 128) reason, .outer hn hu failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.felt_items_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointers_array_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointers_classify
