import ProofCheck.FeltValuesUnique
import ProofCheck.ResourceMapUnique
import ProofCheck.DAModeSpec
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent stored DeployAccount fields. Versions and felts describe limb storage;
    this decoder contract does not enforce transaction protocol validity. -/
structure DeployAccountValue where
  hash : Option (GoArray w64 4)
  salt : Option (GoArray w64 4)
  contract : Option (GoArray w64 4)
  classHash : Option (GoArray w64 4)
  constructorCallData : Option (List (GoArray w64 4))
  version : Option (GoArray w64 4)
  maxFee : Option (GoArray w64 4)
  signature : Option (List (GoArray w64 4))
  nonce : Option (GoArray w64 4)
  resources : Option (GMap w32 ResourceBoundsValue)
  tip : w64
  paymaster : Option (List (GoArray w64 4))
  nonceMode : w32
  feeMode : w32

def deployAccountZero : DeployAccountValue := ⟨none, none, none, none, none, none, none, none, none, none, W64 0, none, W32 0, W32 0⟩

inductive DeployAccountField where
  | hash | salt | contract | classHash | constructorCallData | version | maxFee | signature | nonce | resources | tip | paymaster | nonceMode | feeMode
  deriving DecidableEq

def deployAccountFields : List DeployAccountField := [.hash, .salt, .contract, .classHash, .constructorCallData, .version, .maxFee, .signature, .nonce, .resources, .tip, .paymaster, .nonceMode, .feeMode]
def deployAccountFieldName : DeployAccountField → GoString
  | .hash => go!"TransactionHash"
  | .salt => go!"ContractAddressSalt"
  | .contract => go!"ContractAddress"
  | .classHash => go!"ClassHash"
  | .constructorCallData => go!"ConstructorCallData"
  | .version => go!"Version"
  | .maxFee => go!"MaxFee"
  | .signature => go!"TransactionSignature"
  | .nonce => go!"Nonce"
  | .resources => go!"ResourceBounds"
  | .tip => go!"Tip"
  | .paymaster => go!"PaymasterData"
  | .nonceMode => go!"NonceDAMode"
  | .feeMode => go!"FeeDAMode"
def deployAccountNames : List GoString := [go!"TransactionHash", go!"ContractAddressSalt", go!"ContractAddress", go!"ClassHash", go!"ConstructorCallData", go!"Version", go!"MaxFee", go!"TransactionSignature", go!"Nonce", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"NonceDAMode", go!"FeeDAMode"]

theorem deployAccountField_names : deployAccountFields.map deployAccountFieldName = deployAccountNames := rfl

inductive DeployAccountFieldValid (d : Nat) : DeployAccountField → DeployAccountValue → List w8 → List w8 → DeployAccountValue → Prop where
  | hash {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .hash old item rest {old with hash := value}
  | salt {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .salt old item rest {old with salt := value}
  | contract {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .contract old item rest {old with contract := value}
  | classHash {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .classHash old item rest {old with classHash := value}
  | constructorCallData {old : DeployAccountValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeployAccountFieldValid d .constructorCallData old item rest {old with constructorCallData := value}
  | version {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .version old item rest {old with version := value}
  | maxFee {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .maxFee old item rest {old with maxFee := value}
  | signature {old : DeployAccountValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeployAccountFieldValid d .signature old item rest {old with signature := value}
  | nonce {old : DeployAccountValue} {item rest value} (valid : FeltValid d item value) :
      DeployAccountFieldValid d .nonce old item rest {old with nonce := value}
  | resources {old : DeployAccountValue} {item rest value} (valid : ResourceMapSpec d (item ++ rest) (.success value rest)) :
      DeployAccountFieldValid d .resources old item rest {old with resources := value}
  | tip {old : DeployAccountValue} {item rest value} (valid : ScalarValid old.tip item value) :
      DeployAccountFieldValid d .tip old item rest {old with tip := value}
  | paymaster {old : DeployAccountValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeployAccountFieldValid d .paymaster old item rest {old with paymaster := value}
  | nonceMode {old : DeployAccountValue} {item rest value} (valid : DAModeValid old.nonceMode item value) :
      DeployAccountFieldValid d .nonceMode old item rest {old with nonceMode := value}
  | feeMode {old : DeployAccountValue} {item rest value} (valid : DAModeValid old.feeMode item value) :
      DeployAccountFieldValid d .feeMode old item rest {old with feeMode := value}

inductive DeployAccountFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)
  | resources (reason : ResourceMapFailureReason)
  | scalar (reason : ExpectedFailureReason)
  | mode (reason : DAModeFailureReason)

/-- Failed fields preserve the destination. Map failures retain private child
    allocations and entered levels; DA-mode overflow consumes its integer. Its
    numeric failure position denotes consumption, not an allocated diagnostic. -/
inductive DeployAccountFieldRejected (d : Nat) : DeployAccountField → List w8 → Nat → Nat → Nat → DeployAccountFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .hash wire 0 offset 0 (.felt reason)
  | salt {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .salt wire 0 offset 0 (.felt reason)
  | contract {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .contract wire 0 offset 0 (.felt reason)
  | classHash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .classHash wire 0 offset 0 (.felt reason)
  | constructorCallData {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeployAccountFieldRejected d .constructorCallData wire 0 offset 0 (.values reason)
  | version {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .version wire 0 offset 0 (.felt reason)
  | maxFee {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .maxFee wire 0 offset 0 (.felt reason)
  | signature {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeployAccountFieldRejected d .signature wire 0 offset 0 (.values reason)
  | nonce {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeployAccountFieldRejected d .nonce wire 0 offset 0 (.felt reason)
  | resources {wire consumed offset extraDepth reason} (failure : ResourceMapSpec d wire (.failure consumed offset extraDepth reason)) :
      DeployAccountFieldRejected d .resources wire consumed offset extraDepth (.resources reason)
  | tip {wire reason} (failure : ScalarFailure wire reason) :
      DeployAccountFieldRejected d .tip wire 0 0 0 (.scalar reason)
  | paymaster {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeployAccountFieldRejected d .paymaster wire 0 offset 0 (.values reason)
  | nonceMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      DeployAccountFieldRejected d .nonceMode wire consumed consumed 0 (.mode reason)
  | feeMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      DeployAccountFieldRejected d .feeMode wire consumed consumed 0 (.mode reason)

theorem deployAccountField_classify (d : Nat) (field : DeployAccountField) (old : DeployAccountValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeployAccountFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, DeployAccountFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .hash failure⟩
  | salt =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .salt valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .salt failure⟩
  | contract =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .contract valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .contract failure⟩
  | classHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .classHash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .classHash failure⟩
  | constructorCallData =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .constructorCallData spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .constructorCallData spec⟩
  | version =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .version failure⟩
  | maxFee =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .maxFee valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .maxFee failure⟩
  | signature =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .signature spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .signature spec⟩
  | nonce =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonce valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .nonce failure⟩
  | resources =>
    obtain ⟨outcome, spec⟩ := resourceMap_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := resourceMap_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .resources spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .resources spec⟩
  | tip =>
    rcases scalar_classify old.tip wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .tip valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .tip failure⟩
  | paymaster =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .paymaster spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .paymaster spec⟩
  | nonceMode =>
    rcases daMode_classify old.nonceMode wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonceMode valid⟩
    · exact Or.inr ⟨consumed, consumed, 0, _, .nonceMode failure⟩
  | feeMode =>
    rcases daMode_classify old.feeMode wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .feeMode valid⟩
    · exact Or.inr ⟨consumed, consumed, 0, _, .feeMode failure⟩

theorem deployAccountField_nonempty (d : Nat) (field : DeployAccountField) (old final : DeployAccountValue)
    (item rest : List w8) (valid : DeployAccountFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | salt valid => cases valid <;> simp
  | contract valid => cases valid <;> simp
  | classHash valid => cases valid <;> simp
  | constructorCallData valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | version valid => cases valid <;> simp
  | maxFee valid => cases valid <;> simp
  | signature valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | nonce valid => cases valid <;> simp
  | resources valid =>
    obtain ⟨other, same, positive⟩ := resourceMap_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | tip valid => cases valid <;> simp
  | paymaster valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | nonceMode valid => exact daModeValid_nonempty _ _ _ valid
  | feeMode valid => exact daModeValid_nonempty _ _ _ valid

theorem deployAccountField_prefix_unique (d : Nat) (field : DeployAccountField) (old a b : DeployAccountValue)
    (left right restA restB : List w8)
    (first : DeployAccountFieldValid d field old left restA a) (second : DeployAccountFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid => cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | salt valid => cases second with
    | salt valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | contract valid => cases second with
    | contract valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | classHash valid => cases second with
    | classHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | constructorCallData valid => cases second with
    | constructorCallData valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | version valid => cases second with
    | version valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | maxFee valid => cases second with
    | maxFee valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | signature valid => cases second with
    | signature valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | nonce valid => cases second with
    | nonce valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | resources valid => cases second with
    | resources valid2 =>
      rw [← same] at valid2
      have eq := resourceMapSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | tip valid => cases second with
    | tip valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | paymaster valid => cases second with
    | paymaster valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | nonceMode valid => cases second with
    | nonceMode valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := daModeValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | feeMode valid => cases second with
    | feeMode valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := daModeValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem deployAccountField_not_rejected (d : Nat) (field : DeployAccountField) (old final : DeployAccountValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : DeployAccountFieldFailure)
    (valid : DeployAccountFieldValid d field old item rest final)
    (failure : DeployAccountFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | salt valid => cases failure with
    | salt failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | contract valid => cases failure with
    | contract failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | classHash valid => cases failure with
    | classHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | constructorCallData valid => cases failure with
    | constructorCallData failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | version valid => cases failure with
    | version failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | maxFee valid => cases failure with
    | maxFee failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | signature valid => cases failure with
    | signature failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | nonce valid => cases failure with
    | nonce failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | resources valid => cases failure with
    | resources failure => have eq := resourceMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | tip valid => cases failure with
    | tip failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | paymaster valid => cases failure with
    | paymaster failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | nonceMode valid => cases failure with
    | nonceMode failure => exact daModeValid_not_rejected _ _ _ _ _ _ valid failure
  | feeMode valid => cases failure with
    | feeMode failure => exact daModeValid_not_rejected _ _ _ _ _ _ valid failure

theorem deployAccountField_rejected_unique (d : Nat) (field : DeployAccountField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : DeployAccountFieldFailure)
    (first : DeployAccountFieldRejected d field wire consumedA offsetA depthA a)
    (second : DeployAccountFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | salt failure => cases second with
    | salt failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | contract failure => cases second with
    | contract failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | classHash failure => cases second with
    | classHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | constructorCallData failure => cases second with
    | constructorCallData failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | version failure => cases second with
    | version failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | maxFee failure => cases second with
    | maxFee failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | signature failure => cases second with
    | signature failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | nonce failure => cases second with
    | nonce failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | resources failure => cases second with
    | resources failure2 =>
      have eq := resourceMapSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | tip failure => cases second with
    | tip failure2 =>
      obtain rfl := scalarFailure_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | paymaster failure => cases second with
    | paymaster failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | nonceMode failure => cases second with
    | nonceMode failure2 =>
      obtain ⟨rfl, rfl⟩ := daModeRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | feeMode failure => cases second with
    | feeMode failure2 =>
      obtain ⟨rfl, rfl⟩ := daModeRejected_unique _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem deployAccountField_rejected_bounds (d : Nat) (field : DeployAccountField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeployAccountFieldFailure)
    (failure : DeployAccountFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | hash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | salt failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | contract failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | classHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | constructorCallData failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | version failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | maxFee failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | signature failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | nonce failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | resources failure => exact resourceMap_failure_bounds _ _ _ _ _ _ failure
  | tip failure => omega
  | paymaster failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | nonceMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩
  | feeMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩

theorem deployAccountField_rejected_depth (d : Nat) (field : DeployAccountField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeployAccountFieldFailure) (depth : d ≤ 32)
    (failure : DeployAccountFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | hash => omega
  | salt => omega
  | contract => omega
  | classHash => omega
  | constructorCallData => omega
  | version => omega
  | maxFee => omega
  | signature => omega
  | nonce => omega
  | resources failure => exact resourceMap_failure_depth _ _ _ _ _ _ depth failure
  | tip => omega
  | paymaster => omega
  | nonceMode => omega
  | feeMode => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountField_rejected_depth
