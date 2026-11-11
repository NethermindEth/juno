import ProofCheck.NullableFailure
import ProofCheck.IntegerKey

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- The typed field accepts the two Boolean bytes and preserves its prior value
    for null and undefined. The wire relation is independent of the reader. -/
inductive BoolValid (old : Bool) : List w8 → Bool → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : BoolValid old [head] old
  | boolean (value : Bool) : BoolValid old [if value then W8 245 else W8 244] value

inductive BoolFailureReason where
  | eof
  | mismatch (head : w8)
  deriving DecidableEq

/-- Bool uses Peek rather than the generic head parser: even an incomplete tag
    or a reserved simple-value byte is a mismatch at the original offset. -/
inductive BoolCursorRejected : List w8 → BoolFailureReason → Prop where
  | eof : BoolCursorRejected [] .eof
  | mismatch (head : w8) (tail : List w8)
      (hf : head ≠ W8 244) (ht : head ≠ W8 245) :
      BoolCursorRejected (head :: tail) (.mismatch head)

def BoolRejected (wire : List w8) (reason : BoolFailureReason) : Prop :=
  wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) ∧ BoolCursorRejected wire reason

theorem bool_classify (old : Bool) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ BoolValid old item value) ∨
    (∃ reason, BoolRejected wire reason) := by
  cases wire with
  | nil => exact Or.inr ⟨.eof, by simp, by simp, .eof⟩
  | cons head tail =>
    by_cases hn : head = W8 246 ∨ head = W8 247
    · exact Or.inl ⟨[head], tail, old, rfl, .nullish hn⟩
    by_cases hf : head = W8 244
    · subst head; exact Or.inl ⟨[W8 244], tail, false, rfl, .boolean false⟩
    by_cases ht : head = W8 245
    · subst head; exact Or.inl ⟨[W8 245], tail, true, rfl, .boolean true⟩
    exact Or.inr ⟨.mismatch head, by simpa using fun h => hn (Or.inl h),
      by simpa using fun h => hn (Or.inr h), .mismatch head tail hf ht⟩

theorem bool_valid_length (old value : Bool) (item : List w8) (valid : BoolValid old item value) :
    item.length = 1 := by cases valid <;> rfl

theorem bool_valid_prefix_unique (old a b : Bool) (left right xs ys : List w8)
    (ha : BoolValid old left a) (hb : BoolValid old right b)
    (heq : left ++ xs = right ++ ys) : left = right ∧ a = b ∧ xs = ys := by
  cases ha with
  | nullish hn =>
    cases hb with
    | nullish hm => simpa using heq
    | boolean => cases b <;> rcases hn with rfl | rfl <;> simp at heq
  | boolean =>
    cases hb with
    | nullish hm => cases a <;> rcases hm with rfl | rfl <;> simp at heq
    | boolean => cases a <;> cases b <;> simpa using heq

theorem bool_valid_not_rejected (old value : Bool) (item rest : List w8)
    (reason : BoolFailureReason) (valid : BoolValid old item value) :
    ¬ BoolRejected (item ++ rest) reason := by
  intro ⟨hn, hu, rejected⟩
  cases valid with
  | nullish h => rcases h with rfl | rfl <;> simp at hn hu
  | boolean value =>
    cases value <;> cases rejected with
    | mismatch head tail hf ht => simp_all

theorem bool_rejected_unique (wire : List w8) (a b : BoolFailureReason)
    (ha : BoolRejected wire a) (hb : BoolRejected wire b) : a = b := by
  rcases ha with ⟨_, _, ha⟩
  rcases hb with ⟨_, _, hb⟩
  cases ha <;> cases hb <;> rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.bool_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.bool_valid_length
#print axioms Perennial.github_com.NethermindEth.juno.core.bool_valid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bool_valid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bool_rejected_unique
