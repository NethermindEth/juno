import ProofCheck.EventPointersUnique
import ProofCheck.ProjectionState

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Receipt-event projection values are independent of allocation layout. -/
structure ReceiptEventsValue where
  events : Option (List (Option EventValue))
  hash : Option (GoArray w64 4)

def receiptEventsZero : ReceiptEventsValue := ⟨none, none⟩

inductive ReceiptEventsField where
  | events | hash
  deriving DecidableEq

def receiptEventsFields : List ReceiptEventsField := [.events, .hash]
def receiptEventsFieldName : ReceiptEventsField → GoString
  | .events => go!"Events"
  | .hash => go!"TransactionHash"
def receiptEventsNames : List GoString := projectionNames .receiptEvents

theorem receiptEventsField_names : receiptEventsFields.map receiptEventsFieldName = receiptEventsNames := rfl

theorem receiptEventsField_exists (index : Nat) (bound : index < receiptEventsNames.length) :
    ∃ field, receiptEventsFields[index]? = some field := by
  have inside : index < receiptEventsFields.length := bound
  exact ⟨receiptEventsFields[index], List.getElem?_eq_getElem inside⟩

/-- The complete later input is part of Events acceptance: nested count guards
    inspect remaining bytes, including later fields and the record suffix. -/
inductive ReceiptEventsFieldValid (d : Nat) : ReceiptEventsField → ReceiptEventsValue →
    List w8 → List w8 → ReceiptEventsValue → Prop where
  | events {old : ReceiptEventsValue} {item rest value}
      (valid : EventPointersSpec d (item ++ rest) (.success value rest)) :
      ReceiptEventsFieldValid d .events old item rest {old with events := value}
  | hash {old : ReceiptEventsValue} {item rest value} (valid : FeltValid d item value) :
      ReceiptEventsFieldValid d .hash old item rest {old with hash := value}

inductive ReceiptEventsFieldFailure where
  | events (reason : EventPointersFailureReason)
  | felt (reason : FeltFailureReason)

/-- Both helpers preserve their destination on failure. Events can retain
    consumed input and entered array/map levels; felt decoding rolls back. -/
inductive ReceiptEventsFieldRejected (d : Nat) : ReceiptEventsField → List w8 → Nat → Nat → Nat →
    ReceiptEventsFieldFailure → Prop where
  | events {wire consumed offset extraDepth reason}
      (failure : EventPointersSpec d wire (.failure consumed offset extraDepth reason)) :
      ReceiptEventsFieldRejected d .events wire consumed offset extraDepth (.events reason)
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      ReceiptEventsFieldRejected d .hash wire 0 offset 0 (.felt reason)

theorem receiptEventsField_classify (d : Nat) (field : ReceiptEventsField) (old : ReceiptEventsValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ReceiptEventsFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | events =>
    obtain ⟨outcome, spec⟩ := eventPointers_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := eventPointers_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .events spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .events spec⟩
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .hash failure⟩

theorem receiptEventsField_nonempty (d : Nat) (field : ReceiptEventsField) (old final : ReceiptEventsValue)
    (item rest : List w8) (valid : ReceiptEventsFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | events valid =>
    obtain ⟨other, same, positive⟩ := eventPointers_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive
  | hash valid => cases valid <;> simp

theorem receiptEventsField_prefix_unique (d : Nat) (field : ReceiptEventsField) (old a b : ReceiptEventsValue)
    (left right restA restB : List w8)
    (first : ReceiptEventsFieldValid d field old left restA a)
    (second : ReceiptEventsFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | events valid => cases second with
    | events valid2 =>
      rw [← same] at valid2
      have eq := eventPointersSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | hash valid => cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem receiptEventsField_not_rejected (d : Nat) (field : ReceiptEventsField) (old final : ReceiptEventsValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : ReceiptEventsFieldFailure)
    (valid : ReceiptEventsFieldValid d field old item rest final)
    (failure : ReceiptEventsFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | events valid => cases failure with
    | events failure => have eq := eventPointersSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure d offset item rest _ _ valid failure

theorem receiptEventsField_rejected_unique (d : Nat) (field : ReceiptEventsField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : ReceiptEventsFieldFailure)
    (first : ReceiptEventsFieldRejected d field wire consumedA offsetA depthA a)
    (second : ReceiptEventsFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | events failure => cases second with
    | events failure2 =>
      have eq := eventPointersSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem receiptEventsField_rejected_bounds (d : Nat) (field : ReceiptEventsField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ReceiptEventsFieldFailure)
    (failure : ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | events failure => exact eventPointers_failure_bounds _ _ _ _ _ _ failure
  | hash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩

theorem receiptEventsField_rejected_depth (d : Nat) (field : ReceiptEventsField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ReceiptEventsFieldFailure) (depth : d ≤ 32)
    (failure : ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | events failure => exact eventPointers_failure_depth _ _ _ _ _ _ depth failure
  | hash failure => simpa using depth

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_exists
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptEventsField_rejected_depth
