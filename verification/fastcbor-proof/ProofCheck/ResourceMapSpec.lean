import ProofCheck.ResourceMapEntriesSpec
import ProofCheck.ResourceMapValues

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive ResourceMapFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : ResourceMapEntryFailureReason)

inductive ResourceMapOutcome where
  | success (values : Option (GMap w32 ResourceBoundsValue)) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : ResourceMapFailureReason)

/-- The map clears on null/undefined and publishes a new map only after all
    entries succeed. MapLen checks twice the pair count against all remaining
    bytes before depth. Entry errors retain consumption and entered depth;
    the old destination is preserved and the private map is not published. -/
inductive ResourceMapSpec (d : Nat) : List w8 → ResourceMapOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      ResourceMapSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 160) wire reason) :
      ResourceMapSpec d wire (.failure 0 0 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (short : tail.length < 2 * count) :
      ResourceMapSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .count)
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ tail.length) (deep : 32 ≤ d) :
      ResourceMapSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .depth)
  | map {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : ResourceMapEntriesValid (d + 1) count item rest values) :
      ResourceMapSpec d ((head :: payload) ++ item ++ rest) (.success (some (resourceMapOfEntries values)) rest)
  | element {head payload tail argument done retained consumed offset entered reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ tail.length) (depth : d < 32)
      (failure : ResourceMapEntriesRejected (d + 1) count tail done retained consumed offset entered reason) :
      ResourceMapSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + consumed) (1 + payload.length + offset)
          (1 + if entered then 1 else 0) (.element reason))

/-- All-input map classification, including allocated empty maps and null
    values that consume no additional record depth. -/
theorem resourceMap_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, ResourceMapSpec d wire outcome := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact ⟨_, .nullish head tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases expected_classify (W8 160) wire with
    ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    by_cases short : tail.length < 2 * count
    · exact ⟨_, .count enc major size short⟩
    have fit : 2 * count ≤ tail.length := by omega
    by_cases deep : 32 ≤ d
    · exact ⟨_, .depth enc major size fit deep⟩
    have outer : d < 32 := by omega
    rcases resourceMapEntries_classify (d + 1) count tail (by omega) with
      ⟨item, rest, values, split, valid⟩ | ⟨done, retained, consumed, offset, entered, reason, failure⟩
    · subst tail
      refine ⟨.success (some (resourceMapOfEntries values)) rest, ?_⟩
      simpa only [List.append_assoc] using ResourceMapSpec.map enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩

theorem resourceMap_success_suffix (d : Nat) (wire rest : List w8) (values : Option (GMap w32 ResourceBoundsValue))
    (spec : ResourceMapSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | map enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

theorem resourceMap_failure_bounds (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : ResourceMapFailureReason) (spec : ResourceMapSpec d wire (.failure consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit depth failure =>
    have bounds := resourceMapEntriesRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem resourceMapEntriesRejected_depth (d n consumed offset : Nat) (wire : List w8)
    (done : List (w32 × ResourceBoundsValue)) (retained : ResourceBoundsValue) (entered : Bool) (reason : ResourceMapEntryFailureReason)
    (depth : d ≤ 32) (failure : ResourceMapEntriesRejected d n wire done retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  induction failure with
  | here failure => exact resourceMapEntryRejected_depth _ _ _ _ _ _ _ depth failure
  | next valid failure ih => exact ih

theorem resourceMap_failure_depth (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : ResourceMapFailureReason) (depth : d ≤ 32)
    (spec : ResourceMapSpec d wire (.failure consumed offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | outer => simpa using depth
  | count => simpa using depth
  | depth => simpa using depth
  | element enc major size fit outer failure =>
    have bound := resourceMapEntriesRejected_depth _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMap_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMap_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMap_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntriesRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMap_failure_depth
