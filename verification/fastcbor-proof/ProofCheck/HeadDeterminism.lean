import ProofCheck.HeadClassification

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

/-- The additional-information bits fix the exact number of following head
    bytes, including non-preferred integer-width encodings. -/
theorem headArgument_same_width (head : w8) (left right : List w8) (a b : w64)
    (first : HeadArgument head left a) (second : HeadArgument head right b) :
    left.length = right.length := by
  cases first <;> cases second <;> simp_all <;> word

/-- A complete head byte sequence has one argument value. -/
theorem headArgument_value_unique (head : w8) (payload : List w8) (a b : w64)
    (first : HeadArgument head payload a) (second : HeadArgument head payload b) : a = b := by
  cases first <;> cases second <;> simp_all <;> word

/-- Two head decodings of the same input agree on the consumed head bytes,
    decoded argument and exact remaining suffix. -/
theorem headArgument_prefix_unique (head : w8) (left right restLeft restRight : List w8) (a b : w64)
    (first : HeadArgument head left a) (second : HeadArgument head right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  have width := headArgument_same_width head left right a b first second
  have prefixes := congrArg (List.take left.length) same
  simp only [List.take_left] at prefixes
  rw [width, List.take_left] at prefixes
  subst right
  have tails := List.append_cancel_left same
  exact ⟨rfl, headArgument_value_unique head left a b first second, tails⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_same_width
#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_value_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_prefix_unique
