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
structure L1MessageValue where
  sender : GoArray w8 20
  nonce : Option (GoArray w64 4)
  payload : Option (List (GoArray w64 4))
  selector : Option (GoArray w64 4)
  recipient : Option (GoArray w64 4)

def l1MessageZero : L1MessageValue := ⟨zero_val (GoArray w8 20), none, none, none, none⟩

inductive L1MessageField where
  | sender | nonce | payload | selector | recipient
  deriving DecidableEq

def l1MessageFields : List L1MessageField := [.sender, .nonce, .payload, .selector, .recipient]
def l1MessageFieldName : L1MessageField → GoString
  | .sender => go!"From"
  | .nonce => go!"Nonce"
  | .payload => go!"Payload"
  | .selector => go!"Selector"
  | .recipient => go!"To"
def l1MessageNames : List GoString := [go!"From", go!"Nonce", go!"Payload", go!"Selector", go!"To"]

theorem l1MessageField_names : l1MessageFields.map l1MessageFieldName = l1MessageNames := rfl

/-- Felt slices have a contextual count guard, so successful field decoding
    explicitly retains the actual unread suffix used by that guard. -/
inductive L1MessageFieldValid (d : Nat) : L1MessageField → L1MessageValue → List w8 → List w8 → L1MessageValue → Prop where
  | recipient {old : L1MessageValue} {item rest value} (valid : FeltValid d item value) :
      L1MessageFieldValid d .recipient old item rest {old with recipient := value}
  | nonce {old : L1MessageValue} {item rest value} (valid : FeltValid d item value) :
      L1MessageFieldValid d .nonce old item rest {old with nonce := value}
  | selector {old : L1MessageValue} {item rest value} (valid : FeltValid d item value) :
      L1MessageFieldValid d .selector old item rest {old with selector := value}
  | payload {old : L1MessageValue} {item rest value}
      (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      L1MessageFieldValid d .payload old item rest {old with payload := value}
  | sender {old : L1MessageValue} {item rest value}
      (valid : AddressValid old.sender item value) :
      L1MessageFieldValid d .sender old item rest {old with sender := value}

inductive L1MessageFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)
  | address (reason : BytesIntoFailureReason)

/-- All five field helpers roll back their cursor on failure and retain the
    prior destination, but their diagnostic may point inside the rejected input. -/
inductive L1MessageFieldRejected (d : Nat) : L1MessageField → List w8 → Nat → L1MessageFieldFailure → Prop where
  | recipient {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1MessageFieldRejected d .recipient wire offset (.felt reason)
  | nonce {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1MessageFieldRejected d .nonce wire offset (.felt reason)
  | selector {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1MessageFieldRejected d .selector wire offset (.felt reason)
  | payload {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      L1MessageFieldRejected d .payload wire offset (.values reason)
  | sender {wire reason} (failure : AddressRejected wire reason) :
      L1MessageFieldRejected d .sender wire 0 (.address reason)

theorem l1MessageField_classify (d : Nat) (field : L1MessageField) (old : L1MessageValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1MessageFieldValid d field old item rest final) ∨
    (∃ offset reason, L1MessageFieldRejected d field wire offset reason) := by
  cases field with
  | recipient =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .recipient valid⟩
    · exact Or.inr ⟨offset, _, .recipient failure⟩
  | nonce =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonce valid⟩
    · exact Or.inr ⟨offset, _, .nonce failure⟩
  | selector =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .selector valid⟩
    · exact Or.inr ⟨offset, _, .selector failure⟩
  | payload =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .payload spec⟩
    | failure offset reason =>
      exact Or.inr ⟨offset, _, .payload spec⟩
  | sender =>
    rcases address_classify old.sender wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .sender valid⟩
    · exact Or.inr ⟨0, _, .sender failure⟩

theorem l1MessageField_nonempty (d : Nat) (field : L1MessageField) (old final : L1MessageValue)
    (item rest : List w8) (valid : L1MessageFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | recipient valid => cases valid <;> simp
  | nonce valid => cases valid <;> simp
  | selector valid => cases valid <;> simp
  | payload valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive
  | sender valid => exact addressValid_nonempty _ _ _ valid

/-- Unique field values and consumption require comparing the same complete
    unread input; the suffix is part of felt-slice acceptance. -/
theorem l1MessageField_prefix_unique (d : Nat) (field : L1MessageField) (old a b : L1MessageValue)
    (left right restA restB : List w8)
    (first : L1MessageFieldValid d field old left restA a) (second : L1MessageFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | recipient valid =>
    cases second with
    | recipient valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | nonce valid =>
    cases second with
    | nonce valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | selector valid =>
    cases second with
    | selector valid2 =>
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
  | sender valid =>
    cases second with
    | sender valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := addressValid_prefix_unique old.sender _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem l1MessageField_not_rejected (d : Nat) (field : L1MessageField) (old final : L1MessageValue)
    (item rest : List w8) (offset : Nat) (reason : L1MessageFieldFailure)
    (valid : L1MessageFieldValid d field old item rest final)
    (failure : L1MessageFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | recipient valid => cases failure with
    | recipient failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | nonce valid => cases failure with
    | nonce failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | selector valid => cases failure with
    | selector failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | payload valid => cases failure with
    | payload failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | sender valid => cases failure with
    | sender failure => exact addressValid_not_rejected _ _ _ _ _ valid failure

theorem l1MessageField_rejected_unique (d : Nat) (field : L1MessageField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : L1MessageFieldFailure)
    (first : L1MessageFieldRejected d field wire offsetA a) (second : L1MessageFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | recipient failure => cases second with
    | recipient failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | nonce failure => cases second with
    | nonce failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | selector failure => cases second with
    | selector failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | payload failure => cases second with
    | payload failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | sender failure => cases second with
    | sender failure2 =>
      have eq := bytesIntoRejected_unique 20 wire _ _ failure.2.2 failure2.2.2
      cases eq
      exact ⟨rfl, rfl⟩

theorem l1MessageField_rejected_bound (d : Nat) (field : L1MessageField) (wire : List w8)
    (offset : Nat) (reason : L1MessageFieldFailure) (failure : L1MessageFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | recipient failure => exact feltFailure_offset d offset wire _ failure
  | nonce failure => exact feltFailure_offset d offset wire _ failure
  | selector failure => exact feltFailure_offset d offset wire _ failure
  | payload failure => exact feltValues_failure_bound d offset wire _ failure
  | sender failure => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageField_rejected_bound
