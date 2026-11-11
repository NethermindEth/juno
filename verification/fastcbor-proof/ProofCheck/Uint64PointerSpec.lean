import ProofCheck.ScalarComplete
import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Nullable integer fields clear on null/undefined and replace their pointer on
    an unsigned value, including an allocated zero. They do not preserve the old
    scalar on null as plain uint64 fields do. -/
inductive Uint64PointerValid : List w8 → Option w64 → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : Uint64PointerValid [head] none
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0) :
      Uint64PointerValid (head :: payload) (some argument)

theorem uint64Pointer_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ Uint64PointerValid item value) ∨
    (∃ reason, ScalarFailure wire reason) := by
  rcases scalar_classify (W64 0) wire with ⟨item, rest, value, split, valid⟩ | failure
  · cases valid with
    | nullish hn => exact Or.inl ⟨_, rest, none, split, .nullish hn⟩
    | unsigned enc major => exact Or.inl ⟨_, rest, some value, split, .unsigned enc major⟩
  · exact Or.inr failure

/-- Successful nullable decoding is nonempty and has a unique value and boundary. -/
theorem uint64PointerValid_nonempty (wire : List w8) (value : Option w64)
    (valid : Uint64PointerValid wire value) : 0 < wire.length := by
  cases valid <;> simp

theorem uint64PointerValid_prefix_unique (left right restA restB : List w8) (a b : Option w64)
    (first : Uint64PointerValid left a) (second : Uint64PointerValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | @nullish head hn =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | @unsigned other payload argument enc major =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | @unsigned head payload argument enc major =>
    cases second with
    | @nullish other ho =>
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases ho with rfl | rfl <;> contradiction
    | @unsigned other payload2 argument2 enc2 major2 =>
      simp only [List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨rfl, rfl, rfl⟩ := headArgument_prefix_unique head payload payload2 restA restB argument argument2 enc enc2 same
      exact ⟨rfl, rfl, rfl⟩

theorem uint64PointerValid_not_failure (item rest : List w8) (value : Option w64)
    (reason : ExpectedFailureReason) (valid : Uint64PointerValid item value)
    (failure : ScalarFailure (item ++ rest) reason) : False := by
  cases valid with
  | nullish hn =>
    rcases hn with rfl | rfl
    · exact failure.1 rfl
    · exact failure.2.1 rfl
  | unsigned enc major =>
    exact expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major rfl failure.2.2

inductive Uint64PointerOutcome where
  | success (value : Option w64) (rest : List w8)
  | failure (reason : ExpectedFailureReason)

def Uint64PointerSpec (wire : List w8) : Uint64PointerOutcome → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ Uint64PointerValid item value
  | .failure reason => ScalarFailure wire reason

theorem uint64PointerSpec_unique (wire : List w8) (a b : Uint64PointerOutcome)
    (first : Uint64PointerSpec wire a) (second : Uint64PointerSpec wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := uint64PointerValid_prefix_unique item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (uint64PointerValid_not_failure item rest value reason valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (uint64PointerValid_not_failure item rest value reason valid first)
    | failure reason2 =>
      have hr := expectedFailure_reason_unique (W8 0) wire reason reason2 first.2.2 second.2.2
      exact congrArg Uint64PointerOutcome.failure hr

theorem uint64PointerSpec_total_unique (wire : List w8) :
    ∃ outcome, Uint64PointerSpec wire outcome ∧
      ∀ other, Uint64PointerSpec wire other → other = outcome := by
  have existsOutcome : ∃ outcome, Uint64PointerSpec wire outcome := by
    rcases uint64Pointer_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => uint64PointerSpec_unique wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64Pointer_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.uint64PointerSpec_total_unique
