import ProofCheck.StorageDiffsUnique
import ProofCheck.MigratedClassesUnique
import ProofCheck.SliceFailureUnique
import ProofCheck.SignaturesDisjoint
import ProofCheck.OpeningFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored StateDiff fields, including nil/empty distinctions at every map
    and slice level. This is a decoding schema, not state-transition validation. -/
structure StateDiffValue where
  storage : Option (GMap FeltMapKey StorageDiffsValue)
  nonces : Option (GMap FeltMapKey FeltMapValue)
  deployed : Option (GMap FeltMapKey FeltMapValue)
  declaredV0 : Option (List (Option (GoArray w64 4)))
  declaredV1 : Option (GMap FeltMapKey FeltMapValue)
  replaced : Option (GMap FeltMapKey FeltMapValue)
  migrated : Option (GMap FeltMapKey FeltMapKey)

def stateDiffZero : StateDiffValue := ⟨none, none, none, none, none, none, none⟩

inductive StateDiffField where
  | storage | nonces | deployed | declaredV0 | declaredV1 | replaced | migrated
  deriving DecidableEq

def stateDiffFields : List StateDiffField := [.storage, .nonces, .deployed, .declaredV0, .declaredV1, .replaced, .migrated]
def stateDiffFieldName : StateDiffField → GoString
  | .storage => go!"StorageDiffs"
  | .nonces => go!"Nonces"
  | .deployed => go!"DeployedContracts"
  | .declaredV0 => go!"DeclaredV0Classes"
  | .declaredV1 => go!"DeclaredV1Classes"
  | .replaced => go!"ReplacedClasses"
  | .migrated => go!"MigratedClasses"
def stateDiffNames : List GoString := [go!"StorageDiffs", go!"Nonces", go!"DeployedContracts", go!"DeclaredV0Classes", go!"DeclaredV1Classes", go!"ReplacedClasses", go!"MigratedClasses"]

theorem stateDiffField_names : stateDiffFields.map stateDiffFieldName = stateDiffNames := rfl

inductive StateDiffFieldValid (d : Nat) : StateDiffField → StateDiffValue → List w8 → List w8 → StateDiffValue → Prop where
  | storage {old : StateDiffValue} {item rest value} (valid : StorageDiffsSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .storage old item rest {old with storage := value}
  | nonces {old : StateDiffValue} {item rest value} (valid : FeltMapSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .nonces old item rest {old with nonces := value}
  | deployed {old : StateDiffValue} {item rest value} (valid : FeltMapSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .deployed old item rest {old with deployed := value}
  | declaredV0 {old : StateDiffValue} {item rest value} (valid : FeltPointersValid d item value) :
      StateDiffFieldValid d .declaredV0 old item rest {old with declaredV0 := value}
  | declaredV1 {old : StateDiffValue} {item rest value} (valid : FeltMapSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .declaredV1 old item rest {old with declaredV1 := value}
  | replaced {old : StateDiffValue} {item rest value} (valid : FeltMapSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .replaced old item rest {old with replaced := value}
  | migrated {old : StateDiffValue} {item rest value} (valid : MigratedClassesSpec d (item ++ rest) (.success value rest)) :
      StateDiffFieldValid d .migrated old item rest {old with migrated := value}

inductive StateDiffFieldFailure where
  | storage (reason : StorageDiffsFailureReason)
  | map (reason : FeltMapFailureReason)
  | felts (reason : FeltFailureReason)
  | migrated (reason : MigratedClassesFailureReason)

/-- Failed child readers preserve the old field. Their private partial outputs
    are not published, but consumption and entered depth are retained. -/
inductive StateDiffFieldRejected (d : Nat) : StateDiffField → List w8 →
    Nat → Nat → Nat → StateDiffFieldFailure → Prop where
  | storage {wire consumed offset extraDepth reason} (failure : StorageDiffsSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .storage wire consumed offset extraDepth (.storage reason)
  | nonces {wire consumed offset extraDepth reason} (failure : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .nonces wire consumed offset extraDepth (.map reason)
  | deployed {wire consumed offset extraDepth reason} (failure : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .deployed wire consumed offset extraDepth (.map reason)
  | declaredV0 {wire consumed offset entered reason} (failure : FeltPointersFailure d wire consumed offset entered reason) :
      StateDiffFieldRejected d .declaredV0 wire consumed offset (if entered then 1 else 0) (.felts reason)
  | declaredV1 {wire consumed offset extraDepth reason} (failure : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .declaredV1 wire consumed offset extraDepth (.map reason)
  | replaced {wire consumed offset extraDepth reason} (failure : FeltMapSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .replaced wire consumed offset extraDepth (.map reason)
  | migrated {wire consumed offset extraDepth reason} (failure : MigratedClassesSpec d wire (.failure consumed offset extraDepth reason)) :
      StateDiffFieldRejected d .migrated wire consumed offset extraDepth (.migrated reason)

theorem stateDiffField_classify (d : Nat) (field : StateDiffField) (old : StateDiffValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateDiffFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, StateDiffFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | storage =>
    obtain ⟨outcome, spec⟩ := storageDiffs_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := storageDiffs_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .storage spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .storage spec⟩
  | nonces =>
    obtain ⟨outcome, spec⟩ := feltMap_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltMap_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .nonces spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .nonces spec⟩
  | deployed =>
    obtain ⟨outcome, spec⟩ := feltMap_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltMap_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .deployed spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .deployed spec⟩
  | declaredV0 =>
    rcases feltPointers_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, entered, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .declaredV0 valid⟩
    · exact Or.inr ⟨consumed, offset, (if entered then 1 else 0), _, .declaredV0 failure⟩
  | declaredV1 =>
    obtain ⟨outcome, spec⟩ := feltMap_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltMap_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .declaredV1 spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .declaredV1 spec⟩
  | replaced =>
    obtain ⟨outcome, spec⟩ := feltMap_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltMap_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .replaced spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .replaced spec⟩
  | migrated =>
    obtain ⟨outcome, spec⟩ := migratedClasses_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := migratedClasses_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .migrated spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .migrated spec⟩

theorem stateDiffField_nonempty (d : Nat) (field : StateDiffField) (old final : StateDiffValue)
    (item rest : List w8) (valid : StateDiffFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | storage valid =>
    obtain ⟨other, same, positive⟩ := storageDiffs_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | nonces valid =>
    obtain ⟨other, same, positive⟩ := feltMap_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | deployed valid =>
    obtain ⟨other, same, positive⟩ := feltMap_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | declaredV0 valid => cases valid <;> simp
  | declaredV1 valid =>
    obtain ⟨other, same, positive⟩ := feltMap_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | replaced valid =>
    obtain ⟨other, same, positive⟩ := feltMap_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | migrated valid =>
    obtain ⟨other, same, positive⟩ := migratedClasses_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive

theorem stateDiffField_prefix_unique (d : Nat) (field : StateDiffField) (old a b : StateDiffValue)
    (left right restA restB : List w8)
    (first : StateDiffFieldValid d field old left restA a) (second : StateDiffFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | storage valid => cases second with
    | storage valid2 =>
      rw [← same] at valid2
      have eq := storageDiffsSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | nonces valid => cases second with
    | nonces valid2 =>
      rw [← same] at valid2
      have eq := feltMapSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | deployed valid => cases second with
    | deployed valid2 =>
      rw [← same] at valid2
      have eq := feltMapSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | declaredV0 valid => cases second with
    | declaredV0 valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltPointersValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | declaredV1 valid => cases second with
    | declaredV1 valid2 =>
      rw [← same] at valid2
      have eq := feltMapSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | replaced valid => cases second with
    | replaced valid2 =>
      rw [← same] at valid2
      have eq := feltMapSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | migrated valid => cases second with
    | migrated valid2 =>
      rw [← same] at valid2
      have eq := migratedClassesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩

theorem stateDiffField_not_rejected (d : Nat) (field : StateDiffField) (old final : StateDiffValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : StateDiffFieldFailure)
    (valid : StateDiffFieldValid d field old item rest final)
    (failure : StateDiffFieldRejected d field (item ++ rest) consumed offset extraDepth reason) (bound : (item ++ rest).length < 2^64) : False := by
  cases valid with
  | storage valid => cases failure with
    | storage failure => have eq := storageDiffsSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | nonces valid => cases failure with
    | nonces failure => have eq := feltMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | deployed valid => cases failure with
    | deployed failure => have eq := feltMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | declaredV0 valid => cases failure with
    | declaredV0 failure => exact feltPointersValid_not_failure _ _ _ _ _ _ _ _ bound valid failure
  | declaredV1 valid => cases failure with
    | declaredV1 failure => have eq := feltMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | replaced valid => cases failure with
    | replaced failure => have eq := feltMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | migrated valid => cases failure with
    | migrated failure => have eq := migratedClassesSpec_unique d (item ++ rest) _ _ valid failure; cases eq

theorem stateDiffField_rejected_unique (d : Nat) (field : StateDiffField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : StateDiffFieldFailure)
    (first : StateDiffFieldRejected d field wire consumedA offsetA depthA a)
    (second : StateDiffFieldRejected d field wire consumedB offsetB depthB b) (bound : wire.length < 2^64) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | storage failure => cases second with
    | storage failure2 =>
      have eq := storageDiffsSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | nonces failure => cases second with
    | nonces failure2 =>
      have eq := feltMapSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | deployed failure => cases second with
    | deployed failure2 =>
      have eq := feltMapSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | declaredV0 failure => cases second with
    | declaredV0 failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl⟩ := feltPointersFailure_unique _ _ _ _ _ _ _ _ _ _ bound failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | declaredV1 failure => cases second with
    | declaredV1 failure2 =>
      have eq := feltMapSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | replaced failure => cases second with
    | replaced failure2 =>
      have eq := feltMapSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | migrated failure => cases second with
    | migrated failure2 =>
      have eq := migratedClassesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem stateDiffField_rejected_bounds (d : Nat) (field : StateDiffField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : StateDiffFieldFailure)
    (failure : StateDiffFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | storage failure => exact storageDiffs_failure_bounds _ _ _ _ _ _ failure
  | nonces failure => exact feltMap_failure_bounds _ _ _ _ _ _ failure
  | deployed failure => exact feltMap_failure_bounds _ _ _ _ _ _ failure
  | declaredV0 failure => exact feltPointersFailure_bounds _ _ _ _ _ _ failure
  | declaredV1 failure => exact feltMap_failure_bounds _ _ _ _ _ _ failure
  | replaced failure => exact feltMap_failure_bounds _ _ _ _ _ _ failure
  | migrated failure => exact migratedClasses_failure_bounds _ _ _ _ _ _ failure

theorem stateDiffField_rejected_depth (d : Nat) (field : StateDiffField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : StateDiffFieldFailure) (depth : d ≤ 32)
    (failure : StateDiffFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | storage failure => exact storageDiffs_failure_depth _ _ _ _ _ _ depth failure
  | nonces failure => exact feltMap_failure_depth _ _ _ _ _ _ depth failure
  | deployed failure => exact feltMap_failure_depth _ _ _ _ _ _ depth failure
  | declaredV0 failure => exact feltPointersFailure_depth _ _ _ _ _ _ depth failure
  | declaredV1 failure => exact feltMap_failure_depth _ _ _ _ _ _ depth failure
  | replaced failure => exact feltMap_failure_depth _ _ _ _ _ _ depth failure
  | migrated failure => exact migratedClasses_failure_depth _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffField_rejected_depth
