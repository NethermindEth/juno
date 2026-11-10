import ProofCheck.WireDeterminism
import ProofCheck.SkipSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Atomic encodings cannot be confused with recursive container/tag heads. -/
theorem skipAtom_nonrecursive (head : w8) (argument : w64) (body : List w8)
    (valid : SkipAtom head argument body) :
    head &&& W8 224 ≠ W8 128 ∧ head &&& W8 224 ≠ W8 160 ∧ head &&& W8 224 ≠ W8 192 := by
  cases valid <;> word

/-- The atomic head fixes its payload length, including byte/text strings. -/
theorem skipAtom_length_unique (head : w8) (argument : w64) (left right : List w8)
    (first : SkipAtom head argument left) (second : SkipAtom head argument right) :
    left.length = right.length := by
  cases first <;> cases second <;> simp_all <;> word

/-- Arbitrary recursive ignored values have a unique consumed prefix and suffix.
    This preserves Skip's deliberate acceptance of unvalidated text payloads
    and its chained-tag depth policy; it does not strengthen that policy. -/
theorem skipValid_prefix_unique (d : Nat) (left right restLeft restRight : List w8)
    (first : SkipValid d left) (second : SkipValid d right)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ restLeft = restRight := by
  induction first generalizing right restLeft restRight with
  | @atom d head payload body argument enc accepted =>
    cases second with
    | @atom d2 head2 payload2 body2 argument2 enc2 accepted2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (body ++ restLeft) (body2 ++ restRight) argument argument2 enc enc2 same
      subst payload2
      subst argument2
      have lengths := skipAtom_length_unique head argument body body2 accepted accepted2
      have bodies := congrArg (List.take body.length) tailsEq
      simp only [List.take_left] at bodies
      rw [lengths, List.take_left] at bodies
      subst body2
      have tails := List.append_cancel_left tailsEq
      subst restRight
      exact ⟨rfl, rfl⟩
    | @array d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted).1 major2)
    | @map d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted).2.1 major2)
    | @tag d2 head2 next2 payload2 tail2 argument2 enc2 major2 number2 bound2 child2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted).2.2 major2)
  | @array d head payload argument items enc major count bound children ih =>
    cases second with
    | @atom d2 head2 payload2 body2 argument2 enc2 accepted2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted2).1 major)
    | @array d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (items.flatten ++ restLeft) (items2.flatten ++ restRight) argument argument2 enc enc2 same
      subst payload2
      subst argument2
      have lengths : items.length = items2.length := by omega
      have result := wireSequence_prefix_unique (id : List w8 → List w8) (fun _ => ())
        items items2 restLeft restRight lengths (by
          intro a ha b hb tailA tailB split
          obtain ⟨wire, rest⟩ := ih a ha b tailA tailB (children2 b hb) split
          exact ⟨wire, rfl, rest⟩) (by simpa only [List.flatMap_id] using tailsEq)
      have wires : items.flatten = items2.flatten := by simpa only [List.flatMap_id] using result.1
      exact ⟨by rw [wires], result.2.2⟩
    | @map d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @tag d2 head2 next2 payload2 tail2 argument2 enc2 major2 number2 bound2 child2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
  | @map d head payload argument items enc major count bound children ih =>
    cases second with
    | @atom d2 head2 payload2 body2 argument2 enc2 accepted2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted2).2.1 major)
    | @array d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @map d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (items.flatten ++ restLeft) (items2.flatten ++ restRight) argument argument2 enc enc2 same
      subst payload2
      subst argument2
      have lengths : items.length = items2.length := by omega
      have result := wireSequence_prefix_unique (id : List w8 → List w8) (fun _ => ())
        items items2 restLeft restRight lengths (by
          intro a ha b hb tailA tailB split
          obtain ⟨wire, rest⟩ := ih a ha b tailA tailB (children2 b hb) split
          exact ⟨wire, rfl, rest⟩) (by simpa only [List.flatMap_id] using tailsEq)
      have wires : items.flatten = items2.flatten := by simpa only [List.flatMap_id] using result.1
      exact ⟨by rw [wires], result.2.2⟩
    | @tag d2 head2 next2 payload2 tail2 argument2 enc2 major2 number2 bound2 child2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
  | @tag d head next payload tail argument enc major number bound child ih =>
    cases second with
    | @atom d2 head2 payload2 body2 argument2 enc2 accepted2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      exact False.elim ((skipAtom_nonrecursive _ _ _ accepted2).2.2 major)
    | @array d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @map d2 head2 payload2 argument2 items2 enc2 major2 count2 bound2 children2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      rw [major] at major2
      contradiction
    | @tag d2 head2 next2 payload2 tail2 argument2 enc2 major2 number2 bound2 child2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        ((next :: tail) ++ restLeft) ((next2 :: tail2) ++ restRight) argument argument2 enc enc2 same
      subst payload2
      subst argument2
      have heads : next = next2 := (List.cons.inj (by simpa only [List.cons_append] using tailsEq)).1
      subst next2
      obtain ⟨wire, rest⟩ := ih (next :: tail2) restLeft restRight child2 tailsEq
      exact ⟨by rw [wire], rest⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipAtom_nonrecursive
#print axioms Perennial.github_com.NethermindEth.juno.core.skipAtom_length_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_prefix_unique
