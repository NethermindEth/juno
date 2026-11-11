import ProofCheck.FeltMapEntrySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem feltMapEntry_prefix_unique (d : Nat) (left right restA restB : List w8)
    (a b : FeltMapKey × FeltMapValue)
    (first : FeltMapEntryValid d left restA a) (second : FeltMapEntryValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @pair keyItem valueItem rest key value keyValid valueValid =>
    cases second with
    | @pair keyItem2 valueItem2 rest2 key2 value2 keyValid2 valueValid2 =>
      obtain ⟨rfl, rfl, tailsEq⟩ := feltMapKeyValid_prefix_unique d keyItem keyItem2
        (valueItem ++ restA) (valueItem2 ++ restB) key key2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valueValid valueValid2 tailsEq
      exact ⟨rfl, rfl, rfl⟩

theorem feltMapEntry_not_rejected (d consumed offset : Nat) (item rest : List w8)
    (value : FeltMapKey × FeltMapValue) (retained : Option FeltMapKey) (entered : Bool)
    (reason : FeltMapEntryFailureReason) (valid : FeltMapEntryValid d item rest value)
    (failure : FeltMapEntryRejected d (item ++ rest) retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases valid with
  | @pair keyItem valueItem suffix key value keyValid valueValid =>
    cases failure with
    | key failure =>
      apply feltMapKeyValid_not_failure d _ keyItem (valueItem ++ rest) key _ keyValid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @value keyItem2 wire2 key2 offset2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := feltMapKeyValid_prefix_unique d keyItem keyItem2 (valueItem ++ rest) wire2
        key key2 keyValid valid2 (by simpa only [List.append_assoc] using same)
      rw [← tailEq] at failure2
      exact feltValid_not_failure _ _ _ _ _ _ valueValid failure2

theorem feltMapEntryRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : Option FeltMapKey) (enteredA enteredB : Bool) (reasonA reasonB : FeltMapEntryFailureReason)
    (first : FeltMapEntryRejected d wire a consumedA offsetA enteredA reasonA)
    (second : FeltMapEntryRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  cases first with
  | key failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl⟩ := limbsFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | value valid2 failure2 =>
      rw [same] at failure
      exact False.elim (feltMapKeyValid_not_failure _ _ _ _ _ _ valid2 failure)
  | @value keyItem wire key offset reason valid failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      exact False.elim (feltMapKeyValid_not_failure _ _ _ _ _ _ valid failure2)
    | @value keyItem2 wire2 key2 offset2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltMapKeyValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntry_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntry_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapEntryRejected_unique
