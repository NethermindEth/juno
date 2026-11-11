import ProofCheck.FeltMapWellFormed
import ProofCheck.StorageDiffsSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every stored key and nonnil felt payload has its Go representation length.
    This predicate neither assumes nor defines Go array equality. -/
def StorageDiffsWellFormed : Option (GMap FeltMapKey StorageDiffsValue) → Prop
  | none => True
  | some values => FeltKeyedMapWellFormed FeltMapWellFormed values

theorem storageDiffsEntryValid_wellFormed (d : Nat) (item rest : List w8)
    (entry : FeltMapKey × StorageDiffsValue) (valid : StorageDiffsEntryValid d item rest entry) :
    FeltWordsWellFormed entry.1 ∧ FeltMapWellFormed entry.2 := by
  cases valid with
  | pair keyValid valueValid =>
    exact ⟨feltMapKeyValid_wellFormed _ _ _ keyValid, feltMapSpec_wellFormed _ _ _ _ valueValid⟩

theorem storageDiffsEntriesValid_wellFormed (d n : Nat) (item rest : List w8)
    (entries : List (FeltMapKey × StorageDiffsValue))
    (valid : StorageDiffsEntriesValid d n item rest entries) :
    FeltKeyedEntriesWellFormed FeltMapWellFormed entries := by
  induction valid with
  | nil => intro entry member; simp at member
  | step valid rest ih =>
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact storageDiffsEntryValid_wellFormed _ _ _ _ valid
    · exact ih entry member

/-- Inserting later duplicate keys preserves validity while replacing the value. -/
theorem storageDiffsApply_wellFormed (old : GMap FeltMapKey StorageDiffsValue)
    (entries : List (FeltMapKey × StorageDiffsValue))
    (before : FeltKeyedMapWellFormed FeltMapWellFormed old)
    (valid : FeltKeyedEntriesWellFormed FeltMapWellFormed entries) :
    FeltKeyedMapWellFormed FeltMapWellFormed (storageDiffsApply old entries) :=
  feltKeyedMapWellFormed_fold FeltMapWellFormed old entries before valid

theorem storageDiffsSpec_wellFormed (d : Nat) (wire : List w8)
    (values : Option (GMap FeltMapKey StorageDiffsValue)) (rest : List w8)
    (spec : StorageDiffsSpec d wire (.success values rest)) : StorageDiffsWellFormed values := by
  cases spec with
  | nullish => trivial
  | map enc major size fit depth valid =>
    exact storageDiffsApply_wellFormed ∅ _ (feltKeyedMapWellFormed_empty FeltMapWellFormed)
      (storageDiffsEntriesValid_wellFormed _ _ _ _ _ valid)

/-- A failed value can retain a valid key, but never a malformed-length key. -/
theorem storageDiffsEntryRejected_wellFormed (d consumed offset : Nat) (wire : List w8)
    (retained : Option FeltMapKey) (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntryRejected d wire retained consumed offset entered reason) :
    NullableFeltWellFormed retained := by
  cases failure with
  | key => trivial
  | value valid failure => exact feltMapKeyValid_wellFormed _ _ _ valid

/-- All successfully built private entries and the pending key remain valid
    when any later entry fails. No distinct-key assumption is needed. -/
theorem storageDiffsEntriesRejected_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × StorageDiffsValue)) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedEntriesWellFormed FeltMapWellFormed done ∧ NullableFeltWellFormed retained := by
  induction failure with
  | here failure =>
    exact ⟨by intro entry member; simp at member,
      storageDiffsEntryRejected_wellFormed _ _ _ _ _ _ _ failure⟩
  | next valid failure ih =>
    refine ⟨?_, ih.2⟩
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact storageDiffsEntryValid_wellFormed _ _ _ _ valid
    · exact ih.1 entry member

theorem storageDiffsRejected_private_map_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × StorageDiffsValue)) (retained : Option FeltMapKey)
    (entered : Nat) (reason : StorageDiffsEntryFailureReason)
    (failure : StorageDiffsEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedMapWellFormed FeltMapWellFormed (storageDiffsOfEntries done) ∧
      NullableFeltWellFormed retained := by
  have valid := storageDiffsEntriesRejected_wellFormed _ _ _ _ _ _ _ _ _ failure
  exact ⟨storageDiffsApply_wellFormed ∅ done (feltKeyedMapWellFormed_empty FeltMapWellFormed) valid.1, valid.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntryValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntriesValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsApply_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsSpec_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntryRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntriesRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsRejected_private_map_wellFormed
