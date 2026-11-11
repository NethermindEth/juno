import ProofCheck.FeltValuesUnique
import ProofCheck.SierraEntryValuesUnique
import ProofCheck.DAModeSpec
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent constructor, external and L1-handler entry-point groups.
    No function-count, instruction-range or selector-validity condition is added. -/
structure SierraEntriesValue where
  constructors : Option (List SierraEntryPointValue)
  external : Option (List SierraEntryPointValue)
  l1Handler : Option (List SierraEntryPointValue)

def sierraEntriesZero : SierraEntriesValue := ⟨none, none, none⟩

inductive SierraEntriesField where
  | constructors | external | l1Handler
  deriving DecidableEq

def sierraEntriesFields : List SierraEntriesField := [.constructors, .external, .l1Handler]
def sierraEntriesFieldName : SierraEntriesField → GoString
  | .constructors => go!"Constructor"
  | .external => go!"External"
  | .l1Handler => go!"L1Handler"
def sierraEntriesNames : List GoString := [go!"Constructor", go!"External", go!"L1Handler"]

theorem sierraEntriesField_names : sierraEntriesFields.map sierraEntriesFieldName = sierraEntriesNames := rfl

inductive SierraEntriesFieldValid (d : Nat) : SierraEntriesField → SierraEntriesValue → List w8 → List w8 → SierraEntriesValue → Prop where
  | constructors {old : SierraEntriesValue} {item rest value} (valid : SierraEntryValuesSpec d (item ++ rest) (.success value rest)) :
      SierraEntriesFieldValid d .constructors old item rest {old with constructors := value}
  | external {old : SierraEntriesValue} {item rest value} (valid : SierraEntryValuesSpec d (item ++ rest) (.success value rest)) :
      SierraEntriesFieldValid d .external old item rest {old with external := value}
  | l1Handler {old : SierraEntriesValue} {item rest value} (valid : SierraEntryValuesSpec d (item ++ rest) (.success value rest)) :
      SierraEntriesFieldValid d .l1Handler old item rest {old with l1Handler := value}

inductive SierraEntriesFieldFailure where
  | entries (reason : SierraEntryValuesFailureReason)

/-- Each failed array preserves the old field while retaining its successful
    prefix's consumption and any entered array/record depth. -/
inductive SierraEntriesFieldRejected (d : Nat) : SierraEntriesField → List w8 → Nat → Nat → Nat → SierraEntriesFieldFailure → Prop where
  | constructors {wire consumed offset extraDepth reason} (failure : SierraEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      SierraEntriesFieldRejected d .constructors wire consumed offset extraDepth (.entries reason)
  | external {wire consumed offset extraDepth reason} (failure : SierraEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      SierraEntriesFieldRejected d .external wire consumed offset extraDepth (.entries reason)
  | l1Handler {wire consumed offset extraDepth reason} (failure : SierraEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      SierraEntriesFieldRejected d .l1Handler wire consumed offset extraDepth (.entries reason)

theorem sierraEntriesField_classify (d : Nat) (field : SierraEntriesField) (old : SierraEntriesValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SierraEntriesFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, SierraEntriesFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | constructors =>
    obtain ⟨outcome, spec⟩ := sierraEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := sierraEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .constructors spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .constructors spec⟩
  | external =>
    obtain ⟨outcome, spec⟩ := sierraEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := sierraEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .external spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .external spec⟩
  | l1Handler =>
    obtain ⟨outcome, spec⟩ := sierraEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := sierraEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .l1Handler spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .l1Handler spec⟩

theorem sierraEntriesField_nonempty (d : Nat) (field : SierraEntriesField) (old final : SierraEntriesValue)
    (item rest : List w8) (valid : SierraEntriesFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | constructors valid =>
    obtain ⟨other, same, positive⟩ := sierraEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | external valid =>
    obtain ⟨other, same, positive⟩ := sierraEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | l1Handler valid =>
    obtain ⟨other, same, positive⟩ := sierraEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive

theorem sierraEntriesField_prefix_unique (d : Nat) (field : SierraEntriesField) (old a b : SierraEntriesValue)
    (left right restA restB : List w8)
    (first : SierraEntriesFieldValid d field old left restA a) (second : SierraEntriesFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | constructors valid => cases second with
    | constructors valid2 =>
      rw [← same] at valid2
      have eq := sierraEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | external valid => cases second with
    | external valid2 =>
      rw [← same] at valid2
      have eq := sierraEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | l1Handler valid => cases second with
    | l1Handler valid2 =>
      rw [← same] at valid2
      have eq := sierraEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩

theorem sierraEntriesField_not_rejected (d : Nat) (field : SierraEntriesField) (old final : SierraEntriesValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : SierraEntriesFieldFailure)
    (valid : SierraEntriesFieldValid d field old item rest final)
    (failure : SierraEntriesFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | constructors valid => cases failure with
    | constructors failure => have eq := sierraEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | external valid => cases failure with
    | external failure => have eq := sierraEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | l1Handler valid => cases failure with
    | l1Handler failure => have eq := sierraEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq

theorem sierraEntriesField_rejected_unique (d : Nat) (field : SierraEntriesField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : SierraEntriesFieldFailure)
    (first : SierraEntriesFieldRejected d field wire consumedA offsetA depthA a)
    (second : SierraEntriesFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | constructors failure => cases second with
    | constructors failure2 =>
      have eq := sierraEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | external failure => cases second with
    | external failure2 =>
      have eq := sierraEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | l1Handler failure => cases second with
    | l1Handler failure2 =>
      have eq := sierraEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem sierraEntriesField_rejected_bounds (d : Nat) (field : SierraEntriesField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : SierraEntriesFieldFailure)
    (failure : SierraEntriesFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | constructors failure => exact sierraEntryValues_failure_bounds _ _ _ _ _ _ failure
  | external failure => exact sierraEntryValues_failure_bounds _ _ _ _ _ _ failure
  | l1Handler failure => exact sierraEntryValues_failure_bounds _ _ _ _ _ _ failure

theorem sierraEntriesField_rejected_depth (d : Nat) (field : SierraEntriesField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : SierraEntriesFieldFailure) (depth : d ≤ 32)
    (failure : SierraEntriesFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | constructors failure => exact sierraEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | external failure => exact sierraEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | l1Handler failure => exact sierraEntryValues_failure_depth _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesField_rejected_depth
