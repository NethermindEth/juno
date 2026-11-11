import ProofCheck.MigratedClassesEntriesSpec
import ProofCheck.MigratedClassesValues

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

inductive MigratedClassesFailureReason where
  | expected (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : MigratedClassesEntryFailureReason)

inductive MigratedClassesOutcome where
  | success (values : Option (GMap FeltMapKey FeltMapKey)) (rest : List w8)
  | failure (consumed offset extraDepth : Nat) (reason : MigratedClassesFailureReason)

/-- The map clears on null/undefined and publishes a new map only after all
    entries succeed. MapLen checks twice the pair count against all remaining
    bytes before depth. Entry errors retain consumption and entered depth;
    the old destination is preserved and the private map is not published. -/
inductive MigratedClassesSpec (d : Nat) : List w8 → MigratedClassesOutcome → Prop where
  | nullish (head : w8) (tail : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      MigratedClassesSpec d (head :: tail) (.success none tail)
  | outer {wire reason}
      (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 160) wire reason) :
      MigratedClassesSpec d wire (.failure 0 0 0 (.expected reason))
  | count {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (short : tail.length < 2 * count) :
      MigratedClassesSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .count)
  | depth {head payload tail argument} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ tail.length) (deep : 32 ≤ d) :
      MigratedClassesSpec d ((head :: payload) ++ tail) (.failure 0 0 0 .depth)
  | map {head payload item rest argument values} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ (item ++ rest).length) (depth : d < 32)
      (valid : MigratedClassesEntriesValid (d + 1) count item rest values) :
      MigratedClassesSpec d ((head :: payload) ++ item ++ rest) (.success (some (migratedClassesOfEntries values)) rest)
  | element {head payload tail argument done retained consumed offset entered reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (fit : 2 * count ≤ tail.length) (depth : d < 32)
      (failure : MigratedClassesEntriesRejected (d + 1) count tail done retained consumed offset entered reason) :
      MigratedClassesSpec d ((head :: payload) ++ tail)
        (.failure (1 + payload.length + consumed) (1 + payload.length + offset)
          (1 + if entered then 1 else 0) (.element reason))

/-- All-input map classification, including allocated empty maps and null
    maps; neither entry keys nor entry values accept null/undefined. -/
theorem migratedClasses_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, MigratedClassesSpec d wire outcome := by
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
    rcases migratedClassesEntries_classify (d + 1) count tail (by omega) with
      ⟨item, rest, values, split, valid⟩ | ⟨done, retained, consumed, offset, entered, reason, failure⟩
    · subst tail
      refine ⟨.success (some (migratedClassesOfEntries values)) rest, ?_⟩
      simpa only [List.append_assoc] using MigratedClassesSpec.map enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩

theorem migratedClasses_success_suffix (d : Nat) (wire rest : List w8) (values : Option (GMap FeltMapKey FeltMapKey))
    (spec : MigratedClassesSpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | map enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

theorem migratedClasses_failure_bounds (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : MigratedClassesFailureReason) (spec : MigratedClassesSpec d wire (.failure consumed offset extraDepth reason)) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases spec with
  | outer => omega
  | count => omega
  | depth => omega
  | element enc major size fit depth failure =>
    have bounds := migratedClassesEntriesRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem migratedClassesEntriesRejected_depth (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapKey)) (retained : Option FeltMapKey) (entered : Bool) (reason : MigratedClassesEntryFailureReason)
    (depth : d ≤ 32) (failure : MigratedClassesEntriesRejected d n wire done retained consumed offset entered reason) :
    d + (if entered then 1 else 0) ≤ 32 := by
  induction failure with
  | here failure => exact migratedClassesEntryRejected_depth _ _ _ _ _ _ _ depth failure
  | next valid failure ih => exact ih

theorem migratedClasses_failure_depth (d consumed offset extraDepth : Nat) (wire : List w8)
    (reason : MigratedClassesFailureReason) (depth : d ≤ 32)
    (spec : MigratedClassesSpec d wire (.failure consumed offset extraDepth reason)) : d + extraDepth ≤ 32 := by
  cases spec with
  | outer => simpa using depth
  | count => simpa using depth
  | depth => simpa using depth
  | element enc major size fit outer failure =>
    have bound := migratedClassesEntriesRejected_depth _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_failure_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntriesRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_failure_depth
