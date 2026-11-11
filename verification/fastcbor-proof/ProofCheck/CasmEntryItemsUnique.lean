import ProofCheck.CasmEntryValuesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem casmEntryItem_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : CasmEntryPointValue)
    (first : CasmEntryItemValid d left restA a) (second : CasmEntryItemValid d right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB :=
  casmEntryPointRecord_prefix_unique d casmEntryPointZero a b left right restA restB first second same

theorem casmEntryItem_not_rejected (d consumed offset : Nat) (item rest : List w8) (value retained : CasmEntryPointValue)
    (childDepth : Nat) (reason : CasmEntryPointFailureReason)
    (valid : CasmEntryItemValid d item rest value)
    (failure : CasmEntryItemRejected d (item ++ rest) retained consumed offset childDepth reason) : False :=
  casmEntryPointRecord_not_rejected d consumed offset casmEntryPointZero value retained item rest childDepth reason valid failure

theorem casmEntryItemRejected_unique (d consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (a b : CasmEntryPointValue) (depthA depthB : Nat) (reasonA reasonB : CasmEntryPointFailureReason)
    (first : CasmEntryItemRejected d wire a consumedA offsetA depthA reasonA)
    (second : CasmEntryItemRejected d wire b consumedB offsetB depthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB :=
  casmEntryPointRecordRejected_unique d consumedA offsetA consumedB offsetB casmEntryPointZero a b wire
    depthA depthB reasonA reasonB first second

theorem casmEntryItems_prefix_unique (d n : Nat) (left right restA restB : List w8)
    (a b : List CasmEntryPointValue)
    (first : CasmEntryItemsValid d n left restA a) (second : CasmEntryItemsValid d n right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right restB b with
  | nil => cases second; simpa using same
  | @step n item tail suffix value values valid rest ih =>
    cases second with
    | @step n2 item2 tail2 suffix2 value2 values2 valid2 rest2 =>
      obtain ⟨itemEq, valueEq, tailsEq⟩ := casmEntryItem_prefix_unique d item item2 (tail ++ suffix) (tail2 ++ restB)
        value value2 valid valid2 (by simpa only [List.append_assoc] using same)
      obtain ⟨tailEq, valuesEq, suffixEq⟩ := ih _ _ _ rest2 tailsEq
      exact ⟨by rw [itemEq, tailEq], by rw [valueEq, valuesEq], suffixEq⟩

theorem casmEntryItems_not_rejected (d n consumed offset : Nat) (item rest : List w8)
    (values done : List CasmEntryPointValue) (retained : CasmEntryPointValue) (childDepth : Nat) (reason : CasmEntryPointFailureReason)
    (valid : CasmEntryItemsValid d n item rest values)
    (failure : CasmEntryItemsRejected d n (item ++ rest) done retained consumed offset childDepth reason) : False := by
  generalize same : item ++ rest = wire at failure
  induction valid generalizing wire done retained consumed offset childDepth reason with
  | nil => cases failure
  | @step n item tail suffix value values valid restValid ih =>
    cases failure with
    | here failure =>
      apply casmEntryItem_not_rejected d consumed offset item (tail ++ suffix) value retained childDepth reason valid
      rw [← same] at failure
      simpa only [List.append_assoc] using failure
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨_, _, tailEq⟩ := casmEntryItem_prefix_unique d item item2 (tail ++ suffix) tail2 value value2 valid valid2
        (by simpa only [List.append_assoc] using same)
      exact ih _ _ _ _ _ _ _ tailEq failure2

/-- Private completed elements, the failed partial CasmEntryPoint, and all position/depth
    diagnostics are uniquely determined, even though they are not published. -/
theorem casmEntryItemsRejected_unique (d n consumedA offsetA consumedB offsetB : Nat) (wire : List w8)
    (doneA doneB : List CasmEntryPointValue) (a b : CasmEntryPointValue) (depthA depthB : Nat)
    (reasonA reasonB : CasmEntryPointFailureReason)
    (first : CasmEntryItemsRejected d n wire doneA a consumedA offsetA depthA reasonA)
    (second : CasmEntryItemsRejected d n wire doneB b consumedB offsetB depthB reasonB) :
    doneA = doneB ∧ a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB := by
  generalize same : wire = wire2 at second
  induction first generalizing wire2 doneB b consumedB offsetB depthB reasonB with
  | @here n wire retained consumed offset childDepth reason failure =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := casmEntryItemRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      rw [same] at failure
      exact False.elim (casmEntryItem_not_rejected _ _ _ _ _ _ _ _ _ valid2 failure)
  | @next n item tail value done retained consumed offset childDepth reason valid failure ih =>
    cases second with
    | here failure2 =>
      rw [← same] at failure2
      exact False.elim (casmEntryItem_not_rejected _ _ _ _ _ _ _ _ _ valid failure2)
    | @next n2 item2 tail2 value2 done2 retained2 consumed2 offset2 depth2 reason2 valid2 failure2 =>
      obtain ⟨rfl, rfl, rfl⟩ := casmEntryItem_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      obtain ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩ := ih _ _ _ _ _ _ _ rfl failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItem_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItem_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItemRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItems_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItems_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryItemsRejected_unique
