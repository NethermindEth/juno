import ProofCheck.L2MessagePointerItemsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive L2MessagePointersFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : L2MessageFailureReason)

inductive L2MessagePointersOutcome where
  | success (values : Option (List (Option L2MessageValue))) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : L2MessagePointersFailureReason)

/-- The outer pointer-slice reader clears on null/undefined and publishes its
    new slice only after all elements succeed. ArrayLen checks count against
    all remaining bytes before depth. Element errors retain entered depth and
    consumption; unlike the felt-value slice, this reader does not roll back. -/
inductive L2MessagePointersSpec (d : Nat) : List w8 → L2MessagePointersOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      L2MessagePointersSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      L2MessagePointersSpec d wire (.failure 0 0 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (short : tail.length < count) :
      L2MessagePointersSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .count)
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (deep : 32 ≤ d) :
      L2MessagePointersSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .depth)
  | array {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : L2MessagePointerItemsValid (d + 1) count item rest values) :
      L2MessagePointersSpec d ((head :: payload) ++ item ++ rest) (.success (some values) rest)
  | element {head payload tail argument done retained consumed offset entered reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (depth : d < 32)
      (failure : L2MessagePointerItemsRejected (d + 1) count tail done retained consumed offset entered reason) :
      L2MessagePointersSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + consumed) (1 + payload.length + offset)
          (1 + if entered then 1 else 0) (.element reason))

/-- Full pointer-slice input classification, including empty arrays at the last
    available depth and nil elements that consume no additional map depth. -/
theorem l2MessagePointers_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, L2MessagePointersSpec d wire outcome := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact ⟨_, .nullish head tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases expected_classify (W8 128) wire with
    ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    by_cases short : tail.length < count
    · exact ⟨_, .count enc major size short⟩
    have fit : count ≤ tail.length := by omega
    by_cases deep : 32 ≤ d
    · exact ⟨_, .depth enc major size fit deep⟩
    have outer : d < 32 := by omega
    rcases l2MessagePointerItems_classify (d + 1) count tail (by omega) with
      ⟨item, rest, values, split, valid⟩ | ⟨done, retained, consumed, offset, entered, reason, failure⟩
    · subst tail
      refine ⟨.success (some values) rest, ?_⟩
      simpa only [List.append_assoc] using L2MessagePointersSpec.array enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩

theorem l2MessagePointers_success_suffix (d : Nat) (wire rest : List w8) (values : Option (List (Option L2MessageValue)))
    (spec : L2MessagePointersSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | array enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

theorem l2MessagePointers_failure_bounds (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : L2MessagePointersFailureReason) (spec : L2MessagePointersSpec d wire (.failure consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit depth failure =>
    have bounds := l2MessagePointerItemsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem l2MessageRecordRejected_depth (d consumed offset : Nat) (old retained : L2MessageValue) (wire : List w8)
    (entered : Bool) (reason : L2MessageFailureReason) (depth : d ≤ 32)
    (failure : L2MessageRecordRejected d old wire retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  cases failure with
  | opening => simpa using depth
  | map enc major size depth fit failure => simp only [ite_true]; omega

theorem l2MessagePointerItemsRejected_depth (d n consumed offset : Nat) (wire : List w8)
    (done : List (Option L2MessageValue)) (retained : L2MessageValue) (entered : Bool) (reason : L2MessageFailureReason)
    (depth : d ≤ 32) (failure : L2MessagePointerItemsRejected d n wire done retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  induction failure with
  | here failure =>
    cases failure with
    | record hn hu failure => exact l2MessageRecordRejected_depth _ _ _ _ _ _ _ _ depth failure
  | next valid failure ih => exact ih

theorem l2MessagePointers_failure_depth (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : L2MessagePointersFailureReason) (depth : d ≤ 32)
    (spec : L2MessagePointersSpec d wire (.failure consumed offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | outer => simpa using depth
  | count => simpa using depth
  | depth => simpa using depth
  | element enc major size fit outer failure =>
    have bound := l2MessagePointerItemsRejected_depth _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointers_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointers_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointers_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageRecordRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointerItemsRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessagePointers_failure_depth
