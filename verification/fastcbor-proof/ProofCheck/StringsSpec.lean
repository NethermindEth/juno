import ProofCheck.StringsItemsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive StringsFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : StringFailureReason)

inductive StringsOutcome where
  | success (values : Option (List GoString)) (rest : List w8)
  | failure (offset extraDepth : Nat) (reason : StringsFailureReason)

/-- The outer value-slice reader clears on null/undefined and publishes its
    new slice only after all elements succeed. ArrayLen checks count against
    all remaining bytes before depth. Element errors preserve the entered array
    depth and consumed successful prefix; the failed string itself rolls back. -/
inductive StringsSpec (d : Nat) : List w8 → StringsOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      StringsSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      StringsSpec d wire (.failure 0 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (short : tail.length < count) :
      StringsSpec d ((head :: payload) ++ tail) (.failure 0 0 .count)
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (deep : 32 ≤ d) :
      StringsSpec d ((head :: payload) ++ tail) (.failure 0 0 .depth)
  | array {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : StringsItemsValid count item values) :
      StringsSpec d ((head :: payload) ++ item ++ rest) (.success (some values) rest)
  | element {head payload tail argument done offset reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (depth : d < 32)
      (failure : StringsItemsRejected count tail done offset reason) :
      StringsSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + offset) 1 (.element reason))

/-- Full value-slice input classification, including empty arrays at the last
    available depth and nullish elements that become empty strings. -/
theorem strings_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, StringsSpec d wire outcome := by
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
    rcases stringsItems_classify count tail with
      ⟨item, rest, values, split, valid⟩ | ⟨done, offset, reason, failure⟩
    · subst tail
      refine ⟨.success (some values) rest, ?_⟩
      simpa only [List.append_assoc] using StringsSpec.array enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩

theorem strings_success_suffix (d : Nat) (wire rest : List w8) (values : Option (List GoString))
    (spec : StringsSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | array enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

theorem strings_failure_bounds (d offset extraDepth : Nat) (wire : List w8)
    (reason : StringsFailureReason) (spec : StringsSpec d wire (.failure offset extraDepth reason)) :
    offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit depth failure =>
    have bounds := stringsItemsRejected_bounds _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem strings_failure_depth (d offset extraDepth : Nat) (wire : List w8)
    (reason : StringsFailureReason) (depth : d ≤ 32)
    (spec : StringsSpec d wire (.failure offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | outer => simpa using depth
  | count => simpa using depth
  | depth => simpa using depth
  | element enc major size fit outer failure => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.strings_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.strings_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.strings_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.strings_failure_depth
