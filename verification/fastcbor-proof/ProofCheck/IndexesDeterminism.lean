import ProofCheck.IntegerKeyDeterminism
import ProofCheck.IntsDeterminism
import ProofCheck.IndexesAPISpec
import ProofCheck.SkipDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Once dispatch has selected a field or Skip, the value and byte boundary
    are unique, including nullable arrays and duplicate values. -/
theorem indexesItemValid_prefix_unique (d : Nat) (choice : Option Bool) (initial a b : IndexesValue)
    (left right restA restB : List w8) (first : IndexesItemValid d choice initial left a)
    (second : IndexesItemValid d choice initial right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | ignored valid =>
    cases second with
    | ignored valid2 =>
      obtain ⟨rfl, rfl⟩ := skipValid_prefix_unique d _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | selected valid =>
    cases second with
    | selected valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := intsValid_prefix_unique d _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

/-- Deterministic keys and values compose across arbitrary order, duplicates
    and unknown entries without changing the final suffix boundary. -/
theorem indexesPairsValid_prefix_unique (d n : Nat) (seen : List Bool) (initial a b : IndexesValue)
    (left right restA restB : List w8) (first : IndexesPairsValid d n seen initial left a)
    (second : IndexesPairsValid d n seen initial right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  induction first generalizing right b restA restB with
  | nil fields initial =>
    cases second
    simpa using same
  | @step n seen decoded initial middle final key value tail keyValid valueValid rest ih =>
    cases second with
    | @step n2 seen2 decoded2 initial2 middle2 final2 key2 value2 tail2 keyValid2 valueValid2 rest2 =>
      obtain ⟨keyEq, decodedEq, valuesEq⟩ := integerKeyValid_prefix_unique key key2
        (value ++ tail ++ restA) (value2 ++ tail2 ++ restB) decoded decoded2 keyValid keyValid2
        (by simpa only [List.append_assoc] using same)
      subst key2
      subst decoded2
      obtain ⟨valueEq, middleEq, tailsEq⟩ := indexesItemValid_prefix_unique d _ initial middle middle2
        value value2 (tail ++ restA) (tail2 ++ restB) valueValid valueValid2
        (by simpa only [List.append_assoc] using valuesEq)
      subst value2
      subst middle2
      obtain ⟨tailEq, finalEq, suffixEq⟩ := ih _ _ _ _ rest2 tailsEq
      exact ⟨by rw [tailEq], finalEq, suffixEq⟩

/-- Complete record success determines all values and exactly one input prefix. -/
theorem indexesRecordValid_prefix_unique (d : Nat) (initial a b : IndexesValue)
    (left right restA restB : List w8) (first : IndexesRecordValid d initial left a)
    (second : IndexesRecordValid d initial right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @map other payload body argument count final enc major size depth valid =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
  | @map head payload body argument count final enc major size depth valid =>
    cases second with
    | @nullish other hn =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
    | @map other payload2 body2 argument2 count2 final2 enc2 major2 size2 depth2 valid2 =>
      obtain ⟨rfl, rfl, rfl, tailsEq⟩ := encodedHead_prefix_unique head other payload payload2
        (body ++ restA) (body2 ++ restB) argument argument2 enc enc2 (by simpa only [List.append_assoc] using same)
      have counts : count = count2 := by omega
      subst count2
      obtain ⟨bodyEq, valueEq, restEq⟩ := indexesPairsValid_prefix_unique (d + 1) count [] _ _ _
        body body2 restA restB valid valid2 tailsEq
      exact ⟨by rw [bodyEq], valueEq, restEq⟩

theorem indexesFirstSpec_success_unique (old a b : IndexesValue) (wire restA restB : List w8)
    (first : IndexesFirstSpec old wire (.success a restA)) (second : IndexesFirstSpec old wire (.success b restB)) :
    a = b ∧ restA = restB := by
  obtain ⟨_, left, split, valid⟩ := indexesFirstSpec_success old a wire restA first
  obtain ⟨_, right, split2, valid2⟩ := indexesFirstSpec_success old b wire restB second
  exact (indexesRecordValid_prefix_unique 0 indexesZeroValue a b left right restA restB valid valid2 (split.symm.trans split2)).2

theorem indexesFullSpec_success_unique (old a b : IndexesValue) (wire : List w8)
    (first : IndexesFullSpec old wire (.success a)) (second : IndexesFullSpec old wire (.success b)) : a = b := by
  obtain ⟨_, valid⟩ := indexesFullSpec_success old a wire first
  obtain ⟨_, valid2⟩ := indexesFullSpec_success old b wire second
  exact (indexesRecordValid_prefix_unique 0 indexesZeroValue a b wire wire [] [] valid valid2 rfl).2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItemValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairsValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFirstSpec_success_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesFullSpec_success_unique
