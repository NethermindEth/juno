import ProofCheck.FeltKeyWellFormed
import ProofCheck.MigratedClassesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every stored key and nonnil felt payload has its Go representation length.
    This predicate neither assumes nor defines Go array equality. -/
def MigratedClassesWellFormed : Option (GMap FeltMapKey FeltMapKey) → Prop
  | none => True
  | some values => FeltKeyedMapWellFormed FeltWordsWellFormed values

theorem migratedClassesEntryValid_wellFormed (d : Nat) (item rest : List w8)
    (entry : FeltMapKey × FeltMapKey) (valid : MigratedClassesEntryValid d item rest entry) :
    FeltWordsWellFormed entry.1 ∧ FeltWordsWellFormed entry.2 := by
  cases valid with
  | pair keyValid valueValid =>
    exact ⟨feltMapKeyValid_wellFormed _ _ _ keyValid, feltMapKeyValid_wellFormed _ _ _ valueValid⟩

theorem migratedClassesEntriesValid_wellFormed (d n : Nat) (item rest : List w8)
    (entries : List (FeltMapKey × FeltMapKey))
    (valid : MigratedClassesEntriesValid d n item rest entries) :
    FeltKeyedEntriesWellFormed FeltWordsWellFormed entries := by
  induction valid with
  | nil => intro entry member; simp at member
  | step valid rest ih =>
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact migratedClassesEntryValid_wellFormed _ _ _ _ valid
    · exact ih entry member

/-- Inserting later duplicate keys preserves validity while replacing the value. -/
theorem migratedClassesApply_wellFormed (old : GMap FeltMapKey FeltMapKey)
    (entries : List (FeltMapKey × FeltMapKey))
    (before : FeltKeyedMapWellFormed FeltWordsWellFormed old)
    (valid : FeltKeyedEntriesWellFormed FeltWordsWellFormed entries) :
    FeltKeyedMapWellFormed FeltWordsWellFormed (migratedClassesApply old entries) :=
  feltKeyedMapWellFormed_fold FeltWordsWellFormed old entries before valid

theorem migratedClassesSpec_wellFormed (d : Nat) (wire : List w8)
    (values : Option (GMap FeltMapKey FeltMapKey)) (rest : List w8)
    (spec : MigratedClassesSpec d wire (.success values rest)) : MigratedClassesWellFormed values := by
  cases spec with
  | nullish => trivial
  | map enc major size fit depth valid =>
    exact migratedClassesApply_wellFormed ∅ _ (feltKeyedMapWellFormed_empty FeltWordsWellFormed)
      (migratedClassesEntriesValid_wellFormed _ _ _ _ _ valid)

/-- A failed value can retain a valid key, but never a malformed-length key. -/
theorem migratedClassesEntryRejected_wellFormed (d consumed offset : Nat) (wire : List w8)
    (retained : Option FeltMapKey) (entered : Bool) (reason : MigratedClassesEntryFailureReason)
    (failure : MigratedClassesEntryRejected d wire retained consumed offset entered reason) :
    NullableFeltWellFormed retained := by
  cases failure with
  | key => trivial
  | value valid failure => exact feltMapKeyValid_wellFormed _ _ _ valid

/-- All successfully built private entries and the pending key remain valid
    when any later entry fails. No distinct-key assumption is needed. -/
theorem migratedClassesEntriesRejected_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapKey)) (retained : Option FeltMapKey)
    (entered : Bool) (reason : MigratedClassesEntryFailureReason)
    (failure : MigratedClassesEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedEntriesWellFormed FeltWordsWellFormed done ∧ NullableFeltWellFormed retained := by
  induction failure with
  | here failure =>
    exact ⟨by intro entry member; simp at member,
      migratedClassesEntryRejected_wellFormed _ _ _ _ _ _ _ failure⟩
  | next valid failure ih =>
    refine ⟨?_, ih.2⟩
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact migratedClassesEntryValid_wellFormed _ _ _ _ valid
    · exact ih.1 entry member

theorem migratedClassesRejected_private_map_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapKey)) (retained : Option FeltMapKey)
    (entered : Bool) (reason : MigratedClassesEntryFailureReason)
    (failure : MigratedClassesEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedMapWellFormed FeltWordsWellFormed (migratedClassesOfEntries done) ∧
      NullableFeltWellFormed retained := by
  have valid := migratedClassesEntriesRejected_wellFormed _ _ _ _ _ _ _ _ _ failure
  exact ⟨migratedClassesApply_wellFormed ∅ done (feltKeyedMapWellFormed_empty FeltWordsWellFormed) valid.1, valid.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntryValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntriesValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesApply_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesSpec_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntryRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesEntriesRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesRejected_private_map_wellFormed
