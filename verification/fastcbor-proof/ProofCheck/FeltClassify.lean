import ProofCheck.FeltFailureSpec

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

/-- Exhaustive expectation outcomes, with no restrictions on the input bytes. -/
theorem expected_classify (want : w8) (wire : List w8) :
    (∃ head payload rest argument, wire = (head :: payload) ++ rest ∧
      HeadArgument head payload argument ∧ head &&& W8 224 = want) ∨
    (∃ reason, ExpectedFailure want wire reason) := by
  cases wire with
  | nil => exact Or.inr ⟨.eof, .eof⟩
  | cons head tail =>
    cases head_classify head tail with
    | fault reason failure => exact Or.inr ⟨.head head reason, .head tail failure⟩
    | truncated size info sizeEq short => exact Or.inr ⟨.incomplete, .incomplete tail info sizeEq short⟩
    | encoded payload rest argument split enc =>
      subst tail
      by_cases major : head &&& W8 224 = want
      · exact Or.inl ⟨head, payload, rest, argument, rfl, enc, major⟩
      by_cases tag : head &&& W8 224 = W8 192
      · exact Or.inr ⟨.tag, .tag rest enc major tag⟩
      · exact Or.inr ⟨.typeMismatch head, .typeMismatch rest enc major tag⟩

/-- Every requested unsigned sequence either supplies all limbs or has a valid
    prefix followed by one of the specified expectation failures. -/
theorem unsigned_items_classify (n : Nat) (wire : List w8) :
    (∃ (items : List UnsignedWire) (rest : List w8), wire = unsignedBytes items ++ rest ∧ items.length = n) ∨
    (∃ (items : List UnsignedWire) (tail : List w8) (reason : ExpectedFailureReason),
      wire = unsignedBytes items ++ tail ∧ items.length < n ∧ ExpectedFailure (W8 0) tail reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, rfl, rfl⟩
  | succ n ih =>
    rcases expected_classify (W8 0) wire with ⟨head, payload, rest, argument, split, enc, major⟩ | ⟨reason, failure⟩
    · let item : UnsignedWire := ⟨head, payload, argument, major, enc⟩
      rcases ih rest with ⟨items, tail, htail, hcount⟩ | ⟨items, tail, reason, htail, hcount, failure⟩
      · refine Or.inl ⟨item :: items, tail, ?_, by simp only [List.length_cons, hcount]⟩
        simpa only [unsignedBytes, UnsignedWire.bytes, item, List.flatMap_cons, List.append_assoc, htail] using split
      · refine Or.inr ⟨item :: items, tail, reason, ?_, by simp only [List.length_cons]; omega, failure⟩
        simpa only [unsignedBytes, UnsignedWire.bytes, item, List.flatMap_cons, List.append_assoc, htail] using split
    · exact Or.inr ⟨[], wire, reason, rfl, by simp, failure⟩

def FeltClassified (d : Nat) (wire : List w8) : Prop :=
  (∃ item rest value, wire = item ++ rest ∧ FeltValid d item value) ∨
  (∃ offset reason, FeltFailure d wire offset reason)

/-- All definite felt arrays are classified. Count determines how many unsigned
    limbs to retain; subsequent values are exhaustively classified by Skip. -/
theorem felt_array_classify (d : Nat) (head : w8) (payload body : List w8) (count : w64)
    (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128)
    (hdepth : d ≤ 32) : FeltClassified d ((head :: payload) ++ body) := by
  by_cases bound : d < 32
  · have hc : 0 ≤ uint.Z count := by word
    let n := min (uint.Z count).toNat 4
    have hn : n ≤ 4 ∧ (n : Int) ≤ uint.Z count := by dsimp only [n]; omega
    rcases unsigned_items_classify n body with ⟨items, rest, split, hcount⟩ | ⟨items, tail, reason, split, hcount, failure⟩
    · by_cases small : uint.Z count ≤ 4
      · have hlen : uint.Z count = items.length := by dsimp only [n] at hcount; omega
        refine Or.inl ⟨(head :: payload) ++ unsignedBytes items, rest, some (paddedLimbs items), ?_,
          .small items enc major hlen (by omega) bound⟩
        rw [split, List.append_assoc]
      · have four : items.length = 4 := by dsimp only [n] at hcount; omega
        let extraCount := (uint.Z count).toNat - 4
        have hextra : (extraCount : Int) = uint.Z count - 4 := by dsimp only [extraCount]; omega
        rcases skip_items_classify (d + 1) extraCount rest (fun tail _ => skip_classify (d + 1) tail (by omega)) with
          ⟨extras, suffix, hrest, hsize, valid⟩ | ⟨extras, tail, offset, reason, hrest, hsize, valid, failure⟩
        · refine Or.inl ⟨(head :: payload) ++ unsignedBytes items ++ extras.flatten, suffix,
            some (paddedLimbs items), ?_, .extended items extras enc major (by omega) four bound valid⟩
          rw [split, hrest, List.append_assoc, List.append_assoc]
        · refine Or.inr ⟨1 + payload.length + (unsignedBytes items).length + extras.flatten.length + offset,
            .skipped reason, ?_⟩
          rw [split, hrest, ← List.append_assoc, ← List.append_assoc]
          exact .extras items extras enc major bound four (by omega) valid failure
    · refine Or.inr ⟨1 + payload.length + (unsignedBytes items).length, .expected (W8 0) reason, ?_⟩
      rw [split, ← List.append_assoc]
      exact .limb items enc major bound (by omega) (by omega) failure
  · have heq : d = 32 := by omega
    subst d
    exact Or.inr ⟨0, .depth, .depth body enc major⟩

/-- Every finite input at a supported depth either begins with a specified felt
    value (including nil versus allocated zero) or has a specified failure.
    Null/undefined are handled before array expectation and depth checking. -/
theorem felt_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) : FeltClassified d wire := by
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
      exact felt_array_classify d head payload rest count enc major hdepth
    · exact Or.inr ⟨0, .expected (W8 128) reason, .outer hn hu failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.expected_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.unsigned_items_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.felt_array_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.felt_classify
