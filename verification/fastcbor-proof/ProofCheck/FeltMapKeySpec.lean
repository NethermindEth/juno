import ProofCheck.LimbItemsUnique
import Perennial.Golang.Theory.Map

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Keys require a nonnullable limb array. Pointer values separately preserve
    nil versus a nonnil zero felt. These are mathematical keys, not an assumed
    implementation of Go array equality. -/
abbrev FeltMapKey := GoArray w64 4
abbrev FeltMapValue := Option (GoArray w64 4)

abbrev FeltMapKeyValid (d : Nat) (wire : List w8) (key : FeltMapKey) := FeltValid d wire (some key)

theorem feltMapKey_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest key, wire = item ++ rest ∧ FeltMapKeyValid d item key) ∨
    (∃ offset reason, LimbsFailure d wire offset reason) :=
  limbs_classify d wire depth

theorem feltMapKeyValid_nonempty (d : Nat) (item : List w8) (key : FeltMapKey)
    (valid : FeltMapKeyValid d item key) : 0 < item.length := by
  cases valid <;> simp

theorem feltMapKeyValid_prefix_unique (d : Nat) (left right restA restB : List w8) (a b : FeltMapKey)
    (first : FeltMapKeyValid d left a) (second : FeltMapKeyValid d right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  simpa only [Option.some.injEq] using feltValid_prefix_unique d (some a) (some b) left right restA restB first second same

theorem feltMapKeyValid_not_failure (d offset : Nat) (item rest : List w8) (key : FeltMapKey)
    (reason : FeltFailureReason) (valid : FeltMapKeyValid d item key)
    (failure : LimbsFailure d (item ++ rest) offset reason) : False :=
  limbsFailure_not_valid _ _ _ _ _ _ valid failure

/-- Null/undefined key input produces an array type error at entry start. -/
theorem feltMapKey_nullish_rejected (d : Nat) (head : w8) (rest : List w8)
    (hn : head = W8 246 ∨ head = W8 247) :
    LimbsFailure d (head :: rest) 0 (.expected (W8 128) (.typeMismatch head)) := by
  rcases hn with rfl | rfl
  all_goals exact .outer (.typeMismatch rest (.inline _ (by decide)) (by decide) (by decide))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKey_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKeyValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKeyValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKeyValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKey_nullish_rejected
