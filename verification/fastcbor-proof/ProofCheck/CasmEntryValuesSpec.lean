import ProofCheck.CasmEntryItemsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive CasmEntryValuesFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : CasmEntryPointFailureReason)

inductive CasmEntryValuesOutcome where
  | success (values : Option (List CasmEntryPointValue)) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : CasmEntryValuesFailureReason)

/-- The outer value-slice reader clears on null/undefined and publishes its
    new slice only after all elements succeed. ArrayLen checks count against
    all remaining bytes before depth. Element errors retain entered depth and
    consumption; unlike the felt-value slice, this reader does not roll back. -/
inductive CasmEntryValuesSpec (d : Nat) : List w8 → CasmEntryValuesOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      CasmEntryValuesSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      CasmEntryValuesSpec d wire (.failure 0 0 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (short : tail.length < count) :
      CasmEntryValuesSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .count)
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (deep : 32 ≤ d) :
      CasmEntryValuesSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .depth)
  | array {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : CasmEntryItemsValid (d + 1) count item rest values) :
      CasmEntryValuesSpec d ((head :: payload) ++ item ++ rest) (.success (some values) rest)
  | element {head payload tail argument done retained consumed offset childDepth reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (size : uint.Z argument = (count : Int)) (fit : count ≤ tail.length) (depth : d < 32)
      (failure : CasmEntryItemsRejected (d + 1) count tail done retained consumed offset childDepth reason) :
      CasmEntryValuesSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + consumed) (1 + payload.length + offset)
          (1 + childDepth) (.element reason))

/-- Full value-slice input classification, including empty arrays at the last
    available depth and nullish zero elements that consume no additional map depth. -/
theorem casmEntryValues_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, CasmEntryValuesSpec d wire outcome := by
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
    rcases casmEntryItems_classify (d + 1) count tail (by omega) with
      ⟨item, rest, values, split, valid⟩ | ⟨done, retained, consumed, offset, childDepth, reason, failure⟩
    · subst tail
      refine ⟨.success (some values) rest, ?_⟩
      simpa only [List.append_assoc] using CasmEntryValuesSpec.array enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩

theorem casmEntryValues_success_suffix (d : Nat) (wire rest : List w8) (values : Option (List CasmEntryPointValue))
    (spec : CasmEntryValuesSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | array enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

theorem casmEntryValues_failure_bounds (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : CasmEntryValuesFailureReason) (spec : CasmEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit depth failure =>
    have bounds := casmEntryItemsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem casmEntryItemsRejected_depth (d n consumed offset : Nat) (wire : List w8)
    (done : List CasmEntryPointValue) (retained : CasmEntryPointValue) (childDepth : Nat) (reason : CasmEntryPointFailureReason)
    (depth : d ≤ 32) (failure : CasmEntryItemsRejected d n wire done retained consumed offset childDepth reason) :
    d + childDepth ≤ 32 := by
  induction failure with
  | here failure =>
    exact casmEntryPointRecordRejected_depth _ _ _ _ _ _ _ _ depth failure
  | next valid failure ih => exact ih

theorem casmEntryValues_failure_depth (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : CasmEntryValuesFailureReason) (depth : d ≤ 32)
    (spec : CasmEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | outer => simpa using depth
  | count => simpa using depth
  | depth => simpa using depth
  | element enc major size fit outer failure =>
    have bound := casmEntryItemsRejected_depth _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValues_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValues_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValues_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItemsRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryValues_failure_depth
