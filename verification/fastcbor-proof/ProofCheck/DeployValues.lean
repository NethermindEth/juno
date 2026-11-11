import ProofCheck.FeltValuesUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent stored Deploy fields. Version uses felt limb storage;
    decoding does not enforce a named protocol version. -/
structure DeployValue where
  hash : Option (GoArray w64 4)
  salt : Option (GoArray w64 4)
  contract : Option (GoArray w64 4)
  classHash : Option (GoArray w64 4)
  constructorCallData : Option (List (GoArray w64 4))
  version : Option (GoArray w64 4)

def deployZero : DeployValue := ⟨none, none, none, none, none, none⟩

inductive DeployField where
  | hash | salt | contract | classHash | constructorCallData | version
  deriving DecidableEq

def deployFields : List DeployField := [.hash, .salt, .contract, .classHash, .constructorCallData, .version]
def deployFieldName : DeployField → GoString
  | .hash => go!"TransactionHash"
  | .salt => go!"ContractAddressSalt"
  | .contract => go!"ContractAddress"
  | .classHash => go!"ClassHash"
  | .constructorCallData => go!"ConstructorCallData"
  | .version => go!"Version"
def deployNames : List GoString := [go!"TransactionHash", go!"ContractAddressSalt", go!"ContractAddress", go!"ClassHash", go!"ConstructorCallData", go!"Version"]

theorem deployField_names : deployFields.map deployFieldName = deployNames := rfl

/-- Felt-slice acceptance retains the actual unread suffix for its count guard. -/
inductive DeployFieldValid (d : Nat) : DeployField → DeployValue → List w8 → List w8 → DeployValue → Prop where
  | hash {old : DeployValue} {item rest value} (valid : FeltValid d item value) :
      DeployFieldValid d .hash old item rest {old with hash := value}
  | salt {old : DeployValue} {item rest value} (valid : FeltValid d item value) :
      DeployFieldValid d .salt old item rest {old with salt := value}
  | contract {old : DeployValue} {item rest value} (valid : FeltValid d item value) :
      DeployFieldValid d .contract old item rest {old with contract := value}
  | classHash {old : DeployValue} {item rest value} (valid : FeltValid d item value) :
      DeployFieldValid d .classHash old item rest {old with classHash := value}
  | constructorCallData {old : DeployValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeployFieldValid d .constructorCallData old item rest {old with constructorCallData := value}
  | version {old : DeployValue} {item rest value} (valid : FeltValid d item value) :
      DeployFieldValid d .version old item rest {old with version := value}

inductive DeployFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)

/-- All helpers preserve the prior field and restore the cursor on failure. -/
inductive DeployFieldRejected (d : Nat) : DeployField → List w8 → Nat → DeployFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployFieldRejected d .hash wire offset (.felt reason)
  | salt {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployFieldRejected d .salt wire offset (.felt reason)
  | contract {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployFieldRejected d .contract wire offset (.felt reason)
  | classHash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployFieldRejected d .classHash wire offset (.felt reason)
  | constructorCallData {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeployFieldRejected d .constructorCallData wire offset (.values reason)
  | version {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployFieldRejected d .version wire offset (.felt reason)

theorem deployField_classify (d : Nat) (field : DeployField) (old : DeployValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeployFieldValid d field old item rest final) ∨
    (∃ offset reason, DeployFieldRejected d field wire offset reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨offset, _, .hash failure⟩
  | salt =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .salt valid⟩
    · exact Or.inr ⟨offset, _, .salt failure⟩
  | contract =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .contract valid⟩
    · exact Or.inr ⟨offset, _, .contract failure⟩
  | classHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .classHash valid⟩
    · exact Or.inr ⟨offset, _, .classHash failure⟩
  | constructorCallData =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .constructorCallData spec⟩
    | failure offset reason =>
      exact Or.inr ⟨offset, _, .constructorCallData spec⟩
  | version =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨offset, _, .version failure⟩

theorem deployField_nonempty (d : Nat) (field : DeployField) (old final : DeployValue)
    (item rest : List w8) (valid : DeployFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | salt valid => cases valid <;> simp
  | contract valid => cases valid <;> simp
  | classHash valid => cases valid <;> simp
  | constructorCallData valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have := List.append_cancel_right same
    subst other
    exact positive
  | version valid => cases valid <;> simp

theorem deployField_prefix_unique (d : Nat) (field : DeployField) (old a b : DeployValue)
    (left right restA restB : List w8)
    (first : DeployFieldValid d field old left restA a) (second : DeployFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid =>
    cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | salt valid =>
    cases second with
    | salt valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | contract valid =>
    cases second with
    | contract valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | classHash valid =>
    cases second with
    | classHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | constructorCallData valid =>
    cases second with
    | constructorCallData valid2 =>
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

theorem deployField_not_rejected (d : Nat) (field : DeployField) (old final : DeployValue)
    (item rest : List w8) (offset : Nat) (reason : DeployFieldFailure)
    (valid : DeployFieldValid d field old item rest final)
    (failure : DeployFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | salt valid => cases failure with
    | salt failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | contract valid => cases failure with
    | contract failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | classHash valid => cases failure with
    | classHash failure => exact feltValid_not_failure d offset item rest _ _ valid failure
  | constructorCallData valid => cases failure with
    | constructorCallData failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | version valid => cases failure with
    | version failure => exact feltValid_not_failure d offset item rest _ _ valid failure

theorem deployField_rejected_unique (d : Nat) (field : DeployField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : DeployFieldFailure)
    (first : DeployFieldRejected d field wire offsetA a) (second : DeployFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | salt failure => cases second with
    | salt failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | contract failure => cases second with
    | contract failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | classHash failure => cases second with
    | classHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | constructorCallData failure => cases second with
    | constructorCallData failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | version failure => cases second with
    | version failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique d offsetA offsetB wire _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem deployField_rejected_bound (d : Nat) (field : DeployField) (wire : List w8)
    (offset : Nat) (reason : DeployFieldFailure) (failure : DeployFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | hash failure => exact feltFailure_offset d offset wire _ failure
  | salt failure => exact feltFailure_offset d offset wire _ failure
  | contract failure => exact feltFailure_offset d offset wire _ failure
  | classHash failure => exact feltFailure_offset d offset wire _ failure
  | constructorCallData failure => exact feltValues_failure_bound d offset wire _ failure
  | version failure => exact feltFailure_offset d offset wire _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployField_rejected_bound
