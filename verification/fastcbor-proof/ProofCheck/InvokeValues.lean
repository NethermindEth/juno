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

/-- Independent stored Invoke fields. Versions and felts describe limb storage;
    this decoder contract does not enforce transaction protocol validity. -/
structure InvokeValue where
  hash : Option (GoArray w64 4)
  callData : Option (List (GoArray w64 4))
  signature : Option (List (GoArray w64 4))
  maxFee : Option (GoArray w64 4)
  contract : Option (GoArray w64 4)
  version : Option (GoArray w64 4)
  entry : Option (GoArray w64 4)
  nonce : Option (GoArray w64 4)
  sender : Option (GoArray w64 4)
  resources : Option (GMap w32 ResourceBoundsValue)
  tip : w64
  paymaster : Option (List (GoArray w64 4))
  deployment : Option (List (GoArray w64 4))
  nonceMode : w32
  feeMode : w32
  proofFacts : Option (List (GoArray w64 4))

def invokeZero : InvokeValue := ⟨none, none, none, none, none, none, none, none, none, none, W64 0, none, none, W32 0, W32 0, none⟩

inductive InvokeField where
  | hash | callData | signature | maxFee | contract | version | entry | nonce | sender | resources | tip | paymaster | deployment | nonceMode | feeMode | proofFacts
  deriving DecidableEq

def invokeFields : List InvokeField := [.hash, .callData, .signature, .maxFee, .contract, .version, .entry, .nonce, .sender, .resources, .tip, .paymaster, .deployment, .nonceMode, .feeMode, .proofFacts]
def invokeFieldName : InvokeField → GoString
  | .hash => go!"TransactionHash"
  | .callData => go!"CallData"
  | .signature => go!"TransactionSignature"
  | .maxFee => go!"MaxFee"
  | .contract => go!"ContractAddress"
  | .version => go!"Version"
  | .entry => go!"EntryPointSelector"
  | .nonce => go!"Nonce"
  | .sender => go!"SenderAddress"
  | .resources => go!"ResourceBounds"
  | .tip => go!"Tip"
  | .paymaster => go!"PaymasterData"
  | .deployment => go!"AccountDeploymentData"
  | .nonceMode => go!"NonceDAMode"
  | .feeMode => go!"FeeDAMode"
  | .proofFacts => go!"ProofFacts"
def invokeNames : List GoString := [go!"TransactionHash", go!"CallData", go!"TransactionSignature", go!"MaxFee", go!"ContractAddress", go!"Version", go!"EntryPointSelector", go!"Nonce", go!"SenderAddress", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"AccountDeploymentData", go!"NonceDAMode", go!"FeeDAMode", go!"ProofFacts"]

theorem invokeField_names : invokeFields.map invokeFieldName = invokeNames := rfl

inductive InvokeFieldValid (d : Nat) : InvokeField → InvokeValue → List w8 → List w8 → InvokeValue → Prop where
  | hash {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .hash old item rest {old with hash := value}
  | callData {old : InvokeValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .callData old item rest {old with callData := value}
  | signature {old : InvokeValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .signature old item rest {old with signature := value}
  | maxFee {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .maxFee old item rest {old with maxFee := value}
  | contract {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .contract old item rest {old with contract := value}
  | version {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .version old item rest {old with version := value}
  | entry {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .entry old item rest {old with entry := value}
  | nonce {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .nonce old item rest {old with nonce := value}
  | sender {old : InvokeValue} {item rest value} (valid : FeltValid d item value) :
      InvokeFieldValid d .sender old item rest {old with sender := value}
  | resources {old : InvokeValue} {item rest value} (valid : ResourceMapSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .resources old item rest {old with resources := value}
  | tip {old : InvokeValue} {item rest value} (valid : ScalarValid old.tip item value) :
      InvokeFieldValid d .tip old item rest {old with tip := value}
  | paymaster {old : InvokeValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .paymaster old item rest {old with paymaster := value}
  | deployment {old : InvokeValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .deployment old item rest {old with deployment := value}
  | nonceMode {old : InvokeValue} {item rest value} (valid : DAModeValid old.nonceMode item value) :
      InvokeFieldValid d .nonceMode old item rest {old with nonceMode := value}
  | feeMode {old : InvokeValue} {item rest value} (valid : DAModeValid old.feeMode item value) :
      InvokeFieldValid d .feeMode old item rest {old with feeMode := value}
  | proofFacts {old : InvokeValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      InvokeFieldValid d .proofFacts old item rest {old with proofFacts := value}

inductive InvokeFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)
  | resources (reason : ResourceMapFailureReason)
  | scalar (reason : ExpectedFailureReason)
  | mode (reason : DAModeFailureReason)

/-- Failed fields preserve the destination. Map failures retain private child
    allocations and entered levels; DA-mode overflow consumes its integer. Its
    numeric failure position denotes consumption, not an allocated diagnostic. -/
inductive InvokeFieldRejected (d : Nat) : InvokeField → List w8 → Nat → Nat → Nat → InvokeFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .hash wire 0 offset 0 (.felt reason)
  | callData {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      InvokeFieldRejected d .callData wire 0 offset 0 (.values reason)
  | signature {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      InvokeFieldRejected d .signature wire 0 offset 0 (.values reason)
  | maxFee {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .maxFee wire 0 offset 0 (.felt reason)
  | contract {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .contract wire 0 offset 0 (.felt reason)
  | version {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .version wire 0 offset 0 (.felt reason)
  | entry {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .entry wire 0 offset 0 (.felt reason)
  | nonce {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .nonce wire 0 offset 0 (.felt reason)
  | sender {wire offset reason} (failure : FeltFailure d wire offset reason) :
      InvokeFieldRejected d .sender wire 0 offset 0 (.felt reason)
  | resources {wire consumed offset extraDepth reason} (failure : ResourceMapSpec d wire (.failure consumed offset extraDepth reason)) :
      InvokeFieldRejected d .resources wire consumed offset extraDepth (.resources reason)
  | tip {wire reason} (failure : ScalarFailure wire reason) :
      InvokeFieldRejected d .tip wire 0 0 0 (.scalar reason)
  | paymaster {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      InvokeFieldRejected d .paymaster wire 0 offset 0 (.values reason)
  | deployment {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      InvokeFieldRejected d .deployment wire 0 offset 0 (.values reason)
  | nonceMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      InvokeFieldRejected d .nonceMode wire consumed consumed 0 (.mode reason)
  | feeMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      InvokeFieldRejected d .feeMode wire consumed consumed 0 (.mode reason)
  | proofFacts {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      InvokeFieldRejected d .proofFacts wire 0 offset 0 (.values reason)

theorem invokeField_classify (d : Nat) (field : InvokeField) (old : InvokeValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ InvokeFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, InvokeFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .hash failure⟩
  | callData =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .callData spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .callData spec⟩
  | signature =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .signature spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .signature spec⟩
  | maxFee =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .maxFee valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .maxFee failure⟩
  | contract =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .contract valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .contract failure⟩
  | version =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .version failure⟩
  | entry =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .entry valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .entry failure⟩
  | nonce =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonce valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .nonce failure⟩
  | sender =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .sender valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .sender failure⟩
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
  | deployment =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .deployment spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .deployment spec⟩
  | nonceMode =>
    rcases daMode_classify old.nonceMode wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .nonceMode valid⟩
    · exact Or.inr ⟨consumed, consumed, 0, _, .nonceMode failure⟩
  | feeMode =>
    rcases daMode_classify old.feeMode wire with ⟨item, rest, value, split, valid⟩ | ⟨consumed, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .feeMode valid⟩
    · exact Or.inr ⟨consumed, consumed, 0, _, .feeMode failure⟩
  | proofFacts =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .proofFacts spec⟩
    | failure offset reason => exact Or.inr ⟨0, offset, 0, _, .proofFacts spec⟩

theorem invokeField_nonempty (d : Nat) (field : InvokeField) (old final : InvokeValue)
    (item rest : List w8) (valid : InvokeFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | callData valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | signature valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | maxFee valid => cases valid <;> simp
  | contract valid => cases valid <;> simp
  | version valid => cases valid <;> simp
  | entry valid => cases valid <;> simp
  | nonce valid => cases valid <;> simp
  | sender valid => cases valid <;> simp
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
  | deployment valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | nonceMode valid => exact daModeValid_nonempty _ _ _ valid
  | feeMode valid => exact daModeValid_nonempty _ _ _ valid
  | proofFacts valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive

theorem invokeField_prefix_unique (d : Nat) (field : InvokeField) (old a b : InvokeValue)
    (left right restA restB : List w8)
    (first : InvokeFieldValid d field old left restA a) (second : InvokeFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid => cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | callData valid => cases second with
    | callData valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | signature valid => cases second with
    | signature valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | maxFee valid => cases second with
    | maxFee valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | contract valid => cases second with
    | contract valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | version valid => cases second with
    | version valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | entry valid => cases second with
    | entry valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | nonce valid => cases second with
    | nonce valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | sender valid => cases second with
    | sender valid2 =>
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
  | deployment valid => cases second with
    | deployment valid2 =>
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
  | proofFacts valid => cases second with
    | proofFacts valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩

theorem invokeField_not_rejected (d : Nat) (field : InvokeField) (old final : InvokeValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : InvokeFieldFailure)
    (valid : InvokeFieldValid d field old item rest final)
    (failure : InvokeFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | callData valid => cases failure with
    | callData failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | signature valid => cases failure with
    | signature failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | maxFee valid => cases failure with
    | maxFee failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | contract valid => cases failure with
    | contract failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | version valid => cases failure with
    | version failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | entry valid => cases failure with
    | entry failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | nonce valid => cases failure with
    | nonce failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | sender valid => cases failure with
    | sender failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | resources valid => cases failure with
    | resources failure => have eq := resourceMapSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | tip valid => cases failure with
    | tip failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | paymaster valid => cases failure with
    | paymaster failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | deployment valid => cases failure with
    | deployment failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | nonceMode valid => cases failure with
    | nonceMode failure => exact daModeValid_not_rejected _ _ _ _ _ _ valid failure
  | feeMode valid => cases failure with
    | feeMode failure => exact daModeValid_not_rejected _ _ _ _ _ _ valid failure
  | proofFacts valid => cases failure with
    | proofFacts failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq

theorem invokeField_rejected_unique (d : Nat) (field : InvokeField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : InvokeFieldFailure)
    (first : InvokeFieldRejected d field wire consumedA offsetA depthA a)
    (second : InvokeFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | callData failure => cases second with
    | callData failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | signature failure => cases second with
    | signature failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | maxFee failure => cases second with
    | maxFee failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | contract failure => cases second with
    | contract failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | version failure => cases second with
    | version failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | entry failure => cases second with
    | entry failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | nonce failure => cases second with
    | nonce failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | sender failure => cases second with
    | sender failure2 =>
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
  | deployment failure => cases second with
    | deployment failure2 =>
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
  | proofFacts failure => cases second with
    | proofFacts failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem invokeField_rejected_bounds (d : Nat) (field : InvokeField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : InvokeFieldFailure)
    (failure : InvokeFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | hash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | callData failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | signature failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | maxFee failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | contract failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | version failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | entry failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | nonce failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | sender failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | resources failure => exact resourceMap_failure_bounds _ _ _ _ _ _ failure
  | tip failure => omega
  | paymaster failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | deployment failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | nonceMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩
  | feeMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩
  | proofFacts failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩

theorem invokeField_rejected_depth (d : Nat) (field : InvokeField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : InvokeFieldFailure) (depth : d ≤ 32)
    (failure : InvokeFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | hash => omega
  | callData => omega
  | signature => omega
  | maxFee => omega
  | contract => omega
  | version => omega
  | entry => omega
  | nonce => omega
  | sender => omega
  | resources failure => exact resourceMap_failure_depth _ _ _ _ _ _ depth failure
  | tip => omega
  | paymaster => omega
  | deployment => omega
  | nonceMode => omega
  | feeMode => omega
  | proofFacts => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeField_rejected_depth
