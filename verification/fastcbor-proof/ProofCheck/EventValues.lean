import ProofCheck.FeltValuesUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Application values independent of concrete pointers and allocation sharing. -/
structure EventValue where
  sender : Option (GoArray w64 4)
  keys : Option (List (GoArray w64 4))
  data : Option (List (GoArray w64 4))

def eventZero : EventValue := ⟨none, none, none⟩

inductive EventField where
  | sender | keys | data
  deriving DecidableEq

def eventFields : List EventField := [.sender, .keys, .data]
def eventFieldName : EventField → GoString
  | .sender => go!"From"
  | .keys => go!"Keys"
  | .data => go!"Data"
def eventNames : List GoString := [go!"From", go!"Keys", go!"Data"]

theorem eventField_names : eventFields.map eventFieldName = eventNames := rfl

/-- Felt slices have a contextual count guard, so successful field decoding
    explicitly retains the actual unread suffix used by that guard. -/
inductive EventFieldValid (d : Nat) : EventField → EventValue → List w8 → List w8 → EventValue → Prop where
  | sender {old : EventValue} {item rest value} (valid : FeltValid d item value) :
      EventFieldValid d .sender old item rest {old with sender := value}
  | keys {old : EventValue} {item rest value}
      (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      EventFieldValid d .keys old item rest {old with keys := value}
  | data {old : EventValue} {item rest value}
      (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      EventFieldValid d .data old item rest {old with data := value}

inductive EventFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)

/-- All three field helpers roll back their cursor on failure and retain the
    prior destination, but their diagnostic may point inside the rejected input. -/
inductive EventFieldRejected (d : Nat) : EventField → List w8 → Nat → EventFieldFailure → Prop where
  | sender {wire offset reason} (failure : FeltFailure d wire offset reason) :
      EventFieldRejected d .sender wire offset (.felt reason)
  | keys {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      EventFieldRejected d .keys wire offset (.values reason)
  | data {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      EventFieldRejected d .data wire offset (.values reason)

theorem eventField_classify (d : Nat) (field : EventField) (old : EventValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ EventFieldValid d field old item rest final) ∨
    (∃ offset reason, EventFieldRejected d field wire offset reason) := by
  cases field with
  | sender =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .sender valid⟩
    · exact Or.inr ⟨offset, _, .sender failure⟩
  | keys | data =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      first
      | exact Or.inl ⟨item, rest, _, split, .keys spec⟩
      | exact Or.inl ⟨item, rest, _, split, .data spec⟩
    | failure offset reason =>
      first
      | exact Or.inr ⟨offset, _, .keys spec⟩
      | exact Or.inr ⟨offset, _, .data spec⟩

theorem eventField_nonempty (d : Nat) (field : EventField) (old final : EventValue)
    (item rest : List w8) (valid : EventFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | sender valid => cases valid <;> simp
  | keys valid | data valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive

/-- Unique field values and consumption require comparing the same complete
    unread input; the suffix is part of felt-slice acceptance. -/
theorem eventField_prefix_unique (d : Nat) (field : EventField) (old a b : EventValue)
    (left right restA restB : List w8)
    (first : EventFieldValid d field old left restA a) (second : EventFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | sender valid =>
    cases second with
    | sender valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | keys valid =>
    cases second with
    | keys valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | data valid =>
    cases second with
    | data valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩

theorem eventField_not_rejected (d : Nat) (field : EventField) (old final : EventValue)
    (item rest : List w8) (offset : Nat) (reason : EventFieldFailure)
    (valid : EventFieldValid d field old item rest final)
    (failure : EventFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | sender valid => cases failure with
    | sender failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | keys valid => cases failure with
    | keys failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | data valid => cases failure with
    | data failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq

theorem eventField_rejected_unique (d : Nat) (field : EventField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : EventFieldFailure)
    (first : EventFieldRejected d field wire offsetA a) (second : EventFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | sender failure => cases second with
    | sender failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | keys failure => cases second with
    | keys failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | data failure => cases second with
    | data failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩

theorem eventField_rejected_bound (d : Nat) (field : EventField) (wire : List w8)
    (offset : Nat) (reason : EventFieldFailure) (failure : EventFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | sender failure => exact feltFailure_offset d offset wire _ failure
  | keys failure | data failure => exact feltValues_failure_bound d offset wire _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.eventField_rejected_bound
