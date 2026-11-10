import ProofCheck.FeltDeterminism
import ProofCheck.SignaturesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Unique feltPointers contents and byte boundaries preserve nil versus allocated
    empty slices, including all nested nullable elements. -/
theorem feltPointersValid_prefix_unique (d : Nat) (a b : Option (List (Option (GoArray w64 4))))
    (left right restLeft restRight : List w8)
    (first : FeltPointersValid d left a) (second : FeltPointersValid d right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @array other payload count items enc major size bound =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @array head payload count items enc major size bound =>
    cases second with
    | @nullish other ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @array other payload2 count2 items2 enc2 major2 size2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, countEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (items.flatMap FeltWire.bytes ++ restLeft) (items2.flatMap FeltWire.bytes ++ restRight)
        count count2 enc enc2 same
      subst payload2
      subst count2
      have lengths : items.length = items2.length := by omega
      have result := wireSequence_prefix_unique FeltWire.bytes FeltWire.value
        items items2 restLeft restRight lengths
        (fun x _ y _ tailX tailY split => feltValid_prefix_unique (d + 1) x.value y.value
          x.bytes y.bytes tailX tailY x.valid y.valid split) tailsEq
      exact ⟨by rw [result.1], congrArg some result.2.1, result.2.2⟩

/-- Unique signatures contents and byte boundaries preserve nil versus allocated
    empty slices, including all nested nullable elements. -/
theorem signaturesValid_prefix_unique (d : Nat) (a b : Option (List SignatureValue))
    (left right restLeft restRight : List w8)
    (first : SignaturesValid d left a) (second : SignaturesValid d right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @array other payload count items enc major size bound =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @array head payload count items enc major size bound =>
    cases second with
    | @nullish other ho =>
      simp only [List.append_assoc, List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @array other payload2 count2 items2 enc2 major2 size2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, countEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (items.flatMap SignatureWire.bytes ++ restLeft) (items2.flatMap SignatureWire.bytes ++ restRight)
        count count2 enc enc2 same
      subst payload2
      subst count2
      have lengths : items.length = items2.length := by omega
      have result := wireSequence_prefix_unique SignatureWire.bytes SignatureWire.value
        items items2 restLeft restRight lengths
        (fun x _ y _ tailX tailY split => feltPointersValid_prefix_unique (d + 1) x.value y.value
          x.bytes y.bytes tailX tailY x.valid y.valid split) tailsEq
      exact ⟨by rw [result.1], congrArg some result.2.1, result.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesValid_prefix_unique
