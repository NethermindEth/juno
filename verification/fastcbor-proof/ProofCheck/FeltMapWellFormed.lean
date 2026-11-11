import ProofCheck.FeltKeyWellFormed
import ProofCheck.FeltMapSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every stored key and nonnil felt payload has its Go representation length.
    This predicate neither assumes nor defines Go array equality. -/
def FeltMapWellFormed : Option (GMap FeltMapKey FeltMapValue) → Prop
  | none => True
  | some values => FeltKeyedMapWellFormed NullableFeltWellFormed values

theorem feltMapEntryValid_wellFormed (d : Nat) (item rest : List w8)
    (entry : FeltMapKey × FeltMapValue) (valid : FeltMapEntryValid d item rest entry) :
    FeltWordsWellFormed entry.1 ∧ NullableFeltWellFormed entry.2 := by
  cases valid with
  | pair keyValid valueValid =>
    exact ⟨feltMapKeyValid_wellFormed _ _ _ keyValid, feltValid_wellFormed _ _ _ valueValid⟩

theorem feltMapEntriesValid_wellFormed (d n : Nat) (item rest : List w8)
    (entries : List (FeltMapKey × FeltMapValue))
    (valid : FeltMapEntriesValid d n item rest entries) :
    FeltKeyedEntriesWellFormed NullableFeltWellFormed entries := by
  induction valid with
  | nil => intro entry member; simp at member
  | step valid rest ih =>
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact feltMapEntryValid_wellFormed _ _ _ _ valid
    · exact ih entry member

/-- Inserting later duplicate keys preserves validity while replacing the value. -/
theorem feltMapApply_wellFormed (old : GMap FeltMapKey FeltMapValue)
    (entries : List (FeltMapKey × FeltMapValue))
    (before : FeltKeyedMapWellFormed NullableFeltWellFormed old)
    (valid : FeltKeyedEntriesWellFormed NullableFeltWellFormed entries) :
    FeltKeyedMapWellFormed NullableFeltWellFormed (feltMapApply old entries) :=
  feltKeyedMapWellFormed_fold NullableFeltWellFormed old entries before valid

theorem feltMapSpec_wellFormed (d : Nat) (wire : List w8)
    (values : Option (GMap FeltMapKey FeltMapValue)) (rest : List w8)
    (spec : FeltMapSpec d wire (.success values rest)) : FeltMapWellFormed values := by
  cases spec with
  | nullish => trivial
  | map enc major size fit depth valid =>
    exact feltMapApply_wellFormed ∅ _ (feltKeyedMapWellFormed_empty NullableFeltWellFormed)
      (feltMapEntriesValid_wellFormed _ _ _ _ _ valid)

/-- A failed value can retain a valid key, but never a malformed-length key. -/
theorem feltMapEntryRejected_wellFormed (d consumed offset : Nat) (wire : List w8)
    (retained : Option FeltMapKey) (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntryRejected d wire retained consumed offset entered reason) :
    NullableFeltWellFormed retained := by
  cases failure with
  | key => trivial
  | value valid failure => exact feltMapKeyValid_wellFormed _ _ _ valid

/-- All successfully built private entries and the pending key remain valid
    when any later entry fails. No distinct-key assumption is needed. -/
theorem feltMapEntriesRejected_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapValue)) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedEntriesWellFormed NullableFeltWellFormed done ∧ NullableFeltWellFormed retained := by
  induction failure with
  | here failure =>
    exact ⟨by intro entry member; simp at member,
      feltMapEntryRejected_wellFormed _ _ _ _ _ _ _ failure⟩
  | next valid failure ih =>
    refine ⟨?_, ih.2⟩
    intro entry member
    rcases List.mem_cons.mp member with rfl | member
    · exact feltMapEntryValid_wellFormed _ _ _ _ valid
    · exact ih.1 entry member

theorem feltMapRejected_private_map_wellFormed (d n consumed offset : Nat) (wire : List w8)
    (done : List (FeltMapKey × FeltMapValue)) (retained : Option FeltMapKey)
    (entered : Bool) (reason : FeltMapEntryFailureReason)
    (failure : FeltMapEntriesRejected d n wire done retained consumed offset entered reason) :
    FeltKeyedMapWellFormed NullableFeltWellFormed (feltMapOfEntries done) ∧
      NullableFeltWellFormed retained := by
  have valid := feltMapEntriesRejected_wellFormed _ _ _ _ _ _ _ _ _ failure
  exact ⟨feltMapApply_wellFormed ∅ done (feltKeyedMapWellFormed_empty NullableFeltWellFormed) valid.1, valid.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntriesValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapApply_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapSpec_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntriesRejected_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapRejected_private_map_wellFormed
