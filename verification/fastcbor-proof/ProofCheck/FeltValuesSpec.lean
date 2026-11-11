import ProofCheck.LimbItemsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive FeltValuesFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count (count left : Nat)
  | depth
  | element (reason : FeltFailureReason)

inductive FeltValuesOutcome where
  | success (values : Option (List (GoArray w64 4))) (rest : List w8)
  | failure (offset : Nat) (reason : FeltValuesFailureReason)

/-- The slice allocation guard is deliberately contextual: it counts all
    remaining input bytes, including the suffix following the encoded slice.
    It runs before both depth checks and before any element is decoded. -/
inductive FeltValuesSpec (d : Nat) : List w8 → FeltValuesOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      FeltValuesSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      FeltValuesSpec d wire (.failure 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (short : tail.length < 5 * count) :
      FeltValuesSpec d ((head :: payload) ++ tail) (.failure 0 (.count count tail.length))
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : 5 * count ≤ tail.length) (deep : 32 ≤ d) :
      FeltValuesSpec d ((head :: payload) ++ tail) (.failure 0 .depth)
  | elementDepth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : 5 * count ≤ tail.length)
      (depth : d < 32) (nonempty : 0 < count) (deep : 32 ≤ d + 1) :
      FeltValuesSpec d ((head :: payload) ++ tail) (.failure 0 .depth)
  | array {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : 5 * count ≤ (item ++ rest).length)
      (depth : d < 32) (children : count = 0 ∨ d + 1 < 32)
      (valid : LimbItemsValid (d + 1) count item values) :
      FeltValuesSpec d ((head :: payload) ++ item ++ rest) (.success (some values) rest)
  | element {head payload tail argument done consumed offset reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : 5 * count ≤ tail.length)
      (depth : d < 32) (children : count = 0 ∨ d + 1 < 32)
      (failure : LimbItemsRejected (d + 1) count tail done consumed offset reason) :
      FeltValuesSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + offset) (.element reason))

/-- All byte inputs have a specified value/suffix or diagnostic. Classification
    makes no assumption about the implementation or canonical encoder shapes. -/
theorem feltValues_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, FeltValuesSpec d wire outcome := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact ⟨_, .nullish head tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
  have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
  rcases expected_classify (W8 128) wire with
    ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    by_cases short : tail.length < 5 * count
    · exact ⟨_, .count enc major size short⟩
    have fit : 5 * count ≤ tail.length := by omega
    by_cases deep : 32 ≤ d
    · exact ⟨_, .depth enc major size fit deep⟩
    have outer : d < 32 := by omega
    by_cases children : count = 0 ∨ d + 1 < 32
    · rcases limbItems_classify (d + 1) count tail (by omega) with
        ⟨item, rest, values, split, valid⟩ | ⟨done, consumed, offset, reason, failure⟩
      · subst tail
        refine ⟨.success (some values) rest, ?_⟩
        simpa only [List.append_assoc] using FeltValuesSpec.array enc major size fit outer children valid
      · exact ⟨_, .element enc major size fit outer children failure⟩
    · exact ⟨_, .elementDepth enc major size fit outer (by omega) (by omega)⟩
  · exact ⟨_, .outer hn hu failure⟩

/-- The slice wrapper rolls back its cursor on errors, while an element error
    can retain a diagnostic deeper inside the original input. -/
theorem feltValues_failure_bound (d offset : Nat) (wire : List w8) (reason : FeltValuesFailureReason)
    (spec : FeltValuesSpec d wire (.failure offset reason)) : offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | elementDepth => omega
  | element enc major size fit depth children failure =>
    have bounds := limbItems_rejected_bounds _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

/-- Even the empty slice consumes its array head. Every successful result
    identifies an exact nonempty prefix and the remaining original bytes. -/
theorem feltValues_success_suffix (d : Nat) (wire rest : List w8)
    (values : Option (List (GoArray w64 4))) (spec : FeltValuesSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | array enc major size fit depth children valid => exact ⟨_, rfl, by simp⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_failure_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValues_success_suffix
