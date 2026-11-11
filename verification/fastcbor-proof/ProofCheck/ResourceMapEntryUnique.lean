import ProofCheck.ResourceMapEntrySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem resourceMapEntry_prefix_unique (d : Nat) (left right restA restB : List w8)
    (a b : w32 × ResourceBoundsValue)
    (first : ResourceMapEntryValid d left restA a) (second : ResourceMapEntryValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @pair keyItem valueItem rest key value keyValid valueValid =>
    cases second with
    | @pair keyItem2 valueItem2 rest2 key2 value2 keyValid2 valueValid2 =>
      obtain ⟨rfl, rfl, tailsEq⟩ := resourceMapKeyValid_prefix_unique keyItem keyItem2
        (valueItem ++ restA) (valueItem2 ++ restB) key key2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      obtain ⟨rfl, rfl, rfl⟩ := resourceBoundsRecord_prefix_unique _ _ _ _ _ _ _ _ valueValid valueValid2 tailsEq
      exact ⟨rfl, rfl, rfl⟩

theorem resourceMapEntry_not_rejected (d consumed offset : Nat) (item rest : List w8)
    (value : w32 × ResourceBoundsValue) (retained : ResourceBoundsValue) (entered : Bool)
    (reason : ResourceMapEntryFailureReason) (valid : ResourceMapEntryValid d item rest value)
    (failure : ResourceMapEntryRejected d (item ++ rest) retained consumed offset entered reason) : False := by
  generalize same : item ++ rest = wire at failure
  cases valid with
  | @pair keyItem valueItem suffix key value keyValid valueValid =>
    cases failure with
    | key failure =>
      apply resourceMapKeyValid_not_rejected keyItem (valueItem ++ rest) key _ _ keyValid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @value keyItem2 wire2 key2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := resourceMapKeyValid_prefix_unique keyItem keyItem2 (valueItem ++ rest) wire2
        key key2 keyValid valid2 (by simpa only [List.append_assoc] using same)
      rw [← tailEq] at failure2
      exact resourceBoundsRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valueValid failure2

theorem resourceMapEntryRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : ResourceBoundsValue) (enteredA enteredB : Bool) (reasonA reasonB : ResourceMapEntryFailureReason)
    (first : ResourceMapEntryRejected d wire a consumedA offsetA enteredA reasonA)
    (second : ResourceMapEntryRejected d wire b consumedB offsetB enteredB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ enteredA = enteredB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  cases first with
  | key failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl⟩ := resourceMapKeyRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
    | value valid2 failure2 =>
      rw [same] at failure
      exact False.elim (resourceMapKeyValid_not_rejected _ _ _ _ _ valid2 failure)
  | @value keyItem wire key retained consumed offset entered reason valid failure =>
    cases second with
    | key failure2 =>
      rw [← same] at failure2
      exact False.elim (resourceMapKeyValid_not_rejected _ _ _ _ _ valid failure2)
    | @value keyItem2 wire2 key2 retained2 consumed2 offset2 entered2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := resourceMapKeyValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := resourceBoundsRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntry_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntry_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapEntryRejected_unique
