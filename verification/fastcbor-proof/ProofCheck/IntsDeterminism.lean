import ProofCheck.IntsSpec
import ProofCheck.WireDeterminism
import ProofCheck.SkipDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Null/undefined and signed/unsigned integer elements each have a unique
    mathematical value and byte boundary, for every supported head width. -/
theorem intElementWire_prefix_unique (a b : IntElementWire) (restA restB : List w8)
    (same : a.bytes ++ restA = b.bytes ++ restB) :
    a.bytes = b.bytes ∧ a.number = b.number ∧ restA = restB := by
  cases a with
  | nullish head hn =>
    cases b with
    | nullish other ho =>
      simp only [IntElementWire.bytes, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | unsigned other payload argument enc major bound | negative other payload argument enc major bound =>
      simp only [IntElementWire.bytes, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
  | unsigned head payload argument enc major bound =>
    cases b with
    | nullish other hn =>
      simp only [IntElementWire.bytes, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
    | unsigned other payload2 argument2 enc2 major2 bound2 =>
      exact int64Valid_prefix_unique _ _ restA restB _ _ (.unsigned enc major bound) (.unsigned enc2 major2 bound2) same
    | negative other payload2 argument2 enc2 major2 bound2 =>
      exact int64Valid_prefix_unique _ _ restA restB _ _ (.unsigned enc major bound) (.negative enc2 major2 bound2) same
  | negative head payload argument enc major bound =>
    cases b with
    | nullish other hn =>
      simp only [IntElementWire.bytes, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
    | unsigned other payload2 argument2 enc2 major2 bound2 =>
      exact int64Valid_prefix_unique _ _ restA restB _ _ (.negative enc major bound) (.unsigned enc2 major2 bound2) same
    | negative other payload2 argument2 enc2 major2 bound2 =>
      exact int64Valid_prefix_unique _ _ restA restB _ _ (.negative enc major bound) (.negative enc2 major2 bound2) same

theorem intElements_prefix_unique (a b : List IntElementWire) (restA restB : List w8)
    (count : a.length = b.length) (same : a.flatMap IntElementWire.bytes ++ restA = b.flatMap IntElementWire.bytes ++ restB) :
    a.flatMap IntElementWire.bytes = b.flatMap IntElementWire.bytes ∧
    a.map IntElementWire.number = b.map IntElementWire.number ∧ restA = restB := by
  exact wireSequence_prefix_unique IntElementWire.bytes IntElementWire.number a b restA restB count
    (fun x _ y _ tailA tailB split => intElementWire_prefix_unique x y tailA tailB split) same

/-- Nullable arrays have a unique value and suffix, including nil versus an
    allocated empty array and mixtures of signed, unsigned and null elements. -/
theorem intsValid_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : Option (List Int))
    (first : IntsValid d left a) (second : IntsValid d right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @array other payload argument items enc major count depth =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
  | @array head payload argument items enc major count depth =>
    cases second with
    | @nullish other hn =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> word
    | @array other payload2 argument2 items2 enc2 major2 count2 depth2 =>
      obtain ⟨rfl, rfl, rfl, tails⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap IntElementWire.bytes ++ restA) (items2.flatMap IntElementWire.bytes ++ restB) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      have length : items.length = items2.length := by omega
      obtain ⟨wires, values, suffix⟩ := intElements_prefix_unique items items2 restA restB length tails
      exact ⟨by rw [wires], congrArg some values, suffix⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intElementWire_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.intElements_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.intsValid_prefix_unique
