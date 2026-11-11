import ProofCheck.FeltValuesUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent stored L1-handler fields. Version uses felt limb storage;
    decoding does not enforce a named protocol version. -/
structure L1HandlerValue where
  hash : Option (GoArray w64 4)
  contract : Option (GoArray w64 4)
  entry : Option (GoArray w64 4)
  nonce : Option (GoArray w64 4)
  callData : Option (List (GoArray w64 4))
  version : Option (GoArray w64 4)

def l1HandlerZero : L1HandlerValue := ⟨none, none, none, none, none, none⟩

inductive L1HandlerField where
  | hash | contract | entry | nonce | callData | version
  deriving DecidableEq

def l1HandlerFields : List L1HandlerField := [.hash, .contract, .entry, .nonce, .callData, .version]
def l1HandlerFieldName : L1HandlerField → GoString
  | .hash => go!"TransactionHash"
  | .contract => go!"ContractAddress"
  | .entry => go!"EntryPointSelector"
  | .nonce => go!"Nonce"
  | .callData => go!"CallData"
  | .version => go!"Version"
def l1HandlerNames : List GoString := [go!"TransactionHash", go!"ContractAddress", go!"EntryPointSelector", go!"Nonce", go!"CallData", go!"Version"]

theorem l1HandlerField_names : l1HandlerFields.map l1HandlerFieldName = l1HandlerNames := rfl

/-- Felt-slice acceptance retains the actual unread suffix for its count guard. -/
inductive L1HandlerFieldValid (d : Nat) : L1HandlerField → L1HandlerValue → List w8 → List w8 → L1HandlerValue → Prop where
  | hash {old : L1HandlerValue} {item rest value} (valid : FeltValid d item value) :
      L1HandlerFieldValid d .hash old item rest {old with hash := value}
  | contract {old : L1HandlerValue} {item rest value} (valid : FeltValid d item value) :
      L1HandlerFieldValid d .contract old item rest {old with contract := value}
  | entry {old : L1HandlerValue} {item rest value} (valid : FeltValid d item value) :
      L1HandlerFieldValid d .entry old item rest {old with entry := value}
  | nonce {old : L1HandlerValue} {item rest value} (valid : FeltValid d item value) :
      L1HandlerFieldValid d .nonce old item rest {old with nonce := value}
  | callData {old : L1HandlerValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      L1HandlerFieldValid d .callData old item rest {old with callData := value}
  | version {old : L1HandlerValue} {item rest value} (valid : FeltValid d item value) :
      L1HandlerFieldValid d .version old item rest {old with version := value}

inductive L1HandlerFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)

/-- All helpers preserve the prior field and restore the cursor on failure. -/
inductive L1HandlerFieldRejected (d : Nat) : L1HandlerField → List w8 → Nat → L1HandlerFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1HandlerFieldRejected d .hash wire offset (.felt reason)
  | contract {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1HandlerFieldRejected d .contract wire offset (.felt reason)
  | entry {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1HandlerFieldRejected d .entry wire offset (.felt reason)
  | nonce {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1HandlerFieldRejected d .nonce wire offset (.felt reason)
  | callData {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      L1HandlerFieldRejected d .callData wire offset (.values reason)
  | version {wire offset reason} (failure : FeltFailure d wire offset reason) :
      L1HandlerFieldRejected d .version wire offset (.felt reason)

theorem l1HandlerField_classify (d : Nat) (field : L1HandlerField) (old : L1HandlerValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ L1HandlerFieldValid d field old item rest final) ∨
    (∃ offset reason, L1HandlerFieldRejected d field wire offset reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨offset, _, .hash failure⟩
  | contract =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .contract valid⟩
    · exact Or.inr ⟨offset, _, .contract failure⟩
  | entry =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .entry valid⟩
    · exact Or.inr ⟨offset, _, .entry failure⟩
  | nonce =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonce valid⟩
    · exact Or.inr ⟨offset, _, .nonce failure⟩
  | callData =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .callData spec⟩
    | failure offset reason =>
      exact Or.inr ⟨offset, _, .callData spec⟩
  | version =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨offset, _, .version failure⟩

theorem l1HandlerField_nonempty (d : Nat) (field : L1HandlerField) (old final : L1HandlerValue)
    (item rest : List w8) (valid : L1HandlerFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | contract valid => cases valid <;> simp
  | entry valid => cases valid <;> simp
  | nonce valid => cases valid <;> simp
  | callData valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive
  | version valid => cases valid <;> simp

theorem l1HandlerField_prefix_unique (d : Nat) (field : L1HandlerField) (old a b : L1HandlerValue)
    (left right restA restB : List w8)
    (first : L1HandlerFieldValid d field old left restA a) (second : L1HandlerFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid =>
    cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | contract valid =>
    cases second with
    | contract valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | entry valid =>
    cases second with
    | entry valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | nonce valid =>
    cases second with
    | nonce valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | callData valid =>
    cases second with
    | callData valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | version valid =>
    cases second with
    | version valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem l1HandlerField_not_rejected (d : Nat) (field : L1HandlerField) (old final : L1HandlerValue)
    (item rest : List w8) (offset : Nat) (reason : L1HandlerFieldFailure)
    (valid : L1HandlerFieldValid d field old item rest final)
    (failure : L1HandlerFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | contract valid => cases failure with
    | contract failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | entry valid => cases failure with
    | entry failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | nonce valid => cases failure with
    | nonce failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | callData valid => cases failure with
    | callData failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | version valid => cases failure with
    | version failure => exact feltValid_not_failure d offset item rest _ _ valid failure

theorem l1HandlerField_rejected_unique (d : Nat) (field : L1HandlerField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : L1HandlerFieldFailure)
    (first : L1HandlerFieldRejected d field wire offsetA a) (second : L1HandlerFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | contract failure => cases second with
    | contract failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | entry failure => cases second with
    | entry failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | nonce failure => cases second with
    | nonce failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | callData failure => cases second with
    | callData failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | version failure => cases second with
    | version failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem l1HandlerField_rejected_bound (d : Nat) (field : L1HandlerField) (wire : List w8)
    (offset : Nat) (reason : L1HandlerFieldFailure) (failure : L1HandlerFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | hash failure => exact feltFailure_offset d offset wire _ failure
  | contract failure => exact feltFailure_offset d offset wire _ failure
  | entry failure => exact feltFailure_offset d offset wire _ failure
  | nonce failure => exact feltFailure_offset d offset wire _ failure
  | callData failure => exact feltValues_failure_bound d offset wire _ failure
  | version failure => exact feltFailure_offset d offset wire _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerField_rejected_bound
