import ProofCheck.BoolSpec
import ProofCheck.StringSpec
import ProofCheck.OpeningFailureUnique
import ProofCheck.ProjectionState

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

structure ReceiptStatusValue where
  reverted : Bool
  reason : GoString
  deriving DecidableEq

def receiptStatusZero : ReceiptStatusValue := ⟨false, []⟩

inductive ReceiptStatusField where
  | reverted | reason

def receiptStatusFields : List ReceiptStatusField := [.reverted, .reason]

def receiptStatusFieldName : ReceiptStatusField → GoString
  | .reverted => go!"Reverted"
  | .reason => go!"RevertReason"

def receiptStatusNames : List GoString := projectionNames .receiptExecutionStatus

/-- The independent field identities agree with the actual projection table. -/
theorem receiptStatusField_names : receiptStatusFields.map receiptStatusFieldName = receiptStatusNames := by rfl

theorem receiptStatusField_exists (index : Nat) (bound : index < receiptStatusNames.length) :
    ∃ field, receiptStatusFields[index]? = some field := by
  have inside : index < receiptStatusFields.length := bound
  exact ⟨receiptStatusFields[index], List.getElem?_eq_getElem inside⟩

/-- Nullish fields preserve their prior value. Each successful selected field
    leaves the other field unchanged, including arbitrary pre-existing strings. -/
inductive ReceiptStatusFieldValid : ReceiptStatusField → ReceiptStatusValue →
    List w8 → ReceiptStatusValue → Prop where
  | reverted {old : ReceiptStatusValue} {wire value} (valid : BoolValid old.reverted wire value) :
      ReceiptStatusFieldValid .reverted old wire {old with reverted := value}
  | reason {old : ReceiptStatusValue} {wire value} (valid : StringValid old.reason wire value) :
      ReceiptStatusFieldValid .reason old wire {old with reason := value}

inductive ReceiptStatusFieldFailure where
  | boolean (reason : BoolFailureReason)
  | string (reason : StringFailureReason)

/-- Both selected readers preserve the field and cursor on failure, and report
    the original field offset rather than an offset inside the string payload. -/
inductive ReceiptStatusFieldRejected : ReceiptStatusField → List w8 → Nat →
    ReceiptStatusFieldFailure → Prop where
  | reverted {wire reason} (failure : BoolRejected wire reason) :
      ReceiptStatusFieldRejected .reverted wire 0 (.boolean reason)
  | reason {wire reason} (failure : StringRejected wire reason) :
      ReceiptStatusFieldRejected .reason wire 0 (.string reason)

theorem receiptStatusField_classify (field : ReceiptStatusField) (old : ReceiptStatusValue) (wire : List w8) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptStatusFieldValid field old item final) ∨
    (∃ offset reason, ReceiptStatusFieldRejected field wire offset reason) := by
  cases field with
  | reverted =>
    rcases bool_classify old.reverted wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .reverted valid⟩
    · exact Or.inr ⟨0, _, .reverted failure⟩
  | reason =>
    rcases string_classify old.reason wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .reason valid⟩
    · exact Or.inr ⟨0, _, .reason failure⟩

theorem receiptStatusField_nonempty (field : ReceiptStatusField) (old final : ReceiptStatusValue)
    (wire : List w8) (valid : ReceiptStatusFieldValid field old wire final) : 0 < wire.length := by
  cases valid with
  | reverted valid => rw [bool_valid_length _ _ _ valid]; decide
  | reason valid => exact stringValid_nonempty _ _ _ valid

theorem receiptStatusField_prefix_unique (field : ReceiptStatusField) (old a b : ReceiptStatusValue)
    (left right restA restB : List w8) (ha : ReceiptStatusFieldValid field old left a)
    (hb : ReceiptStatusFieldValid field old right b) (same : left ++ restA = right ++ restB) :
    left = right ∧ a = b ∧ restA = restB := by
  cases ha with
  | reverted valid =>
    cases hb with
    | reverted valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bool_valid_prefix_unique old.reverted _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | reason valid =>
    cases hb with
    | reason valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique old.reason _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem receiptStatusField_not_rejected (field : ReceiptStatusField) (old final : ReceiptStatusValue)
    (item rest : List w8) (offset : Nat) (reason : ReceiptStatusFieldFailure)
    (valid : ReceiptStatusFieldValid field old item final) :
    ¬ ReceiptStatusFieldRejected field (item ++ rest) offset reason := by
  intro rejected
  cases valid with
  | reverted valid => cases rejected with
    | reverted failure => exact bool_valid_not_rejected _ _ _ _ _ valid failure
  | reason valid => cases rejected with
    | reason failure => exact stringValid_not_rejected _ _ _ _ _ valid failure

theorem receiptStatusField_rejected_unique (field : ReceiptStatusField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : ReceiptStatusFieldFailure)
    (ha : ReceiptStatusFieldRejected field wire offsetA a) (hb : ReceiptStatusFieldRejected field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases ha with
  | reverted failure => cases hb with
    | reverted failure2 => exact ⟨rfl, congrArg ReceiptStatusFieldFailure.boolean (bool_rejected_unique _ _ _ failure failure2)⟩
  | reason failure => cases hb with
    | reason failure2 => exact ⟨rfl, congrArg ReceiptStatusFieldFailure.string (stringRejected_reason_unique _ _ _ failure failure2)⟩

theorem receiptStatusField_rejected_offset (field : ReceiptStatusField) (wire : List w8)
    (offset : Nat) (reason : ReceiptStatusFieldFailure) (failure : ReceiptStatusFieldRejected field wire offset reason) :
    offset = 0 := by cases failure <;> rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_exists
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusField_rejected_offset
