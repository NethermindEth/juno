import ProofCheck.FeltValuesUnique
import ProofCheck.WireFailurePrefix
import ProofCheck.AddressSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Application values independent of concrete pointers and allocation sharing. -/
structure L2MessageValue where
  sender : Option (GoArray w64 4)
  payload : Option (List (GoArray w64 4))
  recipient : GoArray w8 20

def l2MessageZero : L2MessageValue := ⟨none, none, zero_val (GoArray w8 20)⟩

inductive L2MessageField where
  | sender | payload | recipient
  deriving DecidableEq

def l2MessageFields : List L2MessageField := [.sender, .payload, .recipient]
def l2MessageFieldName : L2MessageField → GoString
  | .sender => go!"From"
  | .payload => go!"Payload"
  | .recipient => go!"To"
def l2MessageNames : List GoString := [go!"From", go!"Payload", go!"To"]

theorem l2MessageField_names : l2MessageFields.map l2MessageFieldName = l2MessageNames := rfl

/-- Felt slices have a contextual count guard, so successful field decoding
    explicitly retains the actual unread suffix used by that guard. -/
inductive L2MessageFieldValid (d : Nat) : L2MessageField → L2MessageValue → List w8 → List w8 → L2MessageValue → Prop where
  | sender {old : L2MessageValue} {item rest value} (valid : FeltValid d item value) :
      L2MessageFieldValid d .sender old item rest {old with sender := value}
  | payload {old : L2MessageValue} {item rest value}
      (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      L2MessageFieldValid d .payload old item rest {old with payload := value}
  | recipient {old : L2MessageValue} {item rest value}
      (valid : AddressValid old.recipient item value) :
      L2MessageFieldValid d .recipient old item rest {old with recipient := value}

inductive L2MessageFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)
  | address (reason : BytesIntoFailureReason)

/-- All three field helpers roll back their cursor on failure and retain the
    prior destination, but their diagnostic may point inside the rejected input. -/
inductive L2MessageFieldRejected (d : Nat) : L2MessageField → List w8 → Nat → L2MessageFieldFailure → Prop where
  | sender {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L2MessageFieldRejected d .sender wire offset (.felt reason)
  | payload {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      L2MessageFieldRejected d .payload wire offset (.values reason)
  | recipient {wire reason} (failure : AddressRejected wire reason) :
      L2MessageFieldRejected d .recipient wire 0 (.address reason)

theorem l2MessageField_classify (d : Nat) (field : L2MessageField) (old : L2MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L2MessageFieldValid d field old item rest final) ∨
    (∃ offset reason, L2MessageFieldRejected d field wire offset reason) := by
  cases field with
  | sender =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .sender valid⟩
    · exact Or.inr ⟨offset, _, .sender failure⟩
  | payload =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .payload spec⟩
    | failure offset reason =>
      exact Or.inr ⟨offset, _, .payload spec⟩
  | recipient =>
    rcases address_classify old.recipient wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .recipient valid⟩
    · exact Or.inr ⟨0, _, .recipient failure⟩

theorem l2MessageField_nonempty (d : Nat) (field : L2MessageField) (old final : L2MessageValue)
    (item rest : List w8) (valid : L2MessageFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | sender valid => cases valid <;> simp
  | payload valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive
  | recipient valid => exact addressValid_nonempty _ _ _ valid

/-- Unique field values and consumption require comparing the same complete
    unread input; the suffix is part of felt-slice acceptance. -/
theorem l2MessageField_prefix_unique (d : Nat) (field : L2MessageField) (old a b : L2MessageValue)
    (left right restA restB : List w8)
    (first : L2MessageFieldValid d field old left restA a) (second : L2MessageFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | sender valid =>
    cases second with
    | sender valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | payload valid =>
    cases second with
    | payload valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | recipient valid =>
    cases second with
    | recipient valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := addressValid_prefix_unique old.recipient _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem l2MessageField_not_rejected (d : Nat) (field : L2MessageField) (old final : L2MessageValue)
    (item rest : List w8) (offset : Nat) (reason : L2MessageFieldFailure)
    (valid : L2MessageFieldValid d field old item rest final)
    (failure : L2MessageFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | sender valid => cases failure with
    | sender failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | payload valid => cases failure with
    | payload failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | recipient valid => cases failure with
    | recipient failure => exact addressValid_not_rejected _ _ _ _ _ valid failure

theorem l2MessageField_rejected_unique (d : Nat) (field : L2MessageField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : L2MessageFieldFailure)
    (first : L2MessageFieldRejected d field wire offsetA a) (second : L2MessageFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | sender failure => cases second with
    | sender failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | payload failure => cases second with
    | payload failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | recipient failure => cases second with
    | recipient failure2 =>
      have eq := bytesIntoRejected_unique 20 wire _ _ failure.2.2 failure2.2.2
      cases eq
      exact ⟨rfl, rfl⟩

theorem l2MessageField_rejected_bound (d : Nat) (field : L2MessageField) (wire : List w8)
    (offset : Nat) (reason : L2MessageFieldFailure) (failure : L2MessageFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | sender failure => exact feltFailure_offset d offset wire _ failure
  | payload failure => exact feltValues_failure_bound d offset wire _ failure
  | recipient failure => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageField_rejected_bound
