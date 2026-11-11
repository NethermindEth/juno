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

/-- Independent stored Declare fields. Versions and felts describe limb storage;
    this decoder contract does not enforce transaction protocol validity. -/
structure DeclareValue where
  hash : Option (GoArray w64 4)
  classHash : Option (GoArray w64 4)
  sender : Option (GoArray w64 4)
  maxFee : Option (GoArray w64 4)
  signature : Option (List (GoArray w64 4))
  nonce : Option (GoArray w64 4)
  version : Option (GoArray w64 4)
  compiledClassHash : Option (GoArray w64 4)
  resources : Option (GMap w32 ResourceBoundsValue)
  tip : w64
  paymaster : Option (List (GoArray w64 4))
  deployment : Option (List (GoArray w64 4))
  nonceMode : w32
  feeMode : w32

def declareZero : DeclareValue := ⟨none, none, none, none, none, none, none, none, none, W64 0, none, none, W32 0, W32 0⟩

inductive DeclareField where
  | hash | classHash | sender | maxFee | signature | nonce | version | compiledClassHash | resources | tip | paymaster | deployment | nonceMode | feeMode
  deriving DecidableEq

def declareFields : List DeclareField := [.hash, .classHash, .sender, .maxFee, .signature, .nonce, .version, .compiledClassHash, .resources, .tip, .paymaster, .deployment, .nonceMode, .feeMode]
def declareFieldName : DeclareField → GoString
  | .hash => go!"TransactionHash"
  | .classHash => go!"ClassHash"
  | .sender => go!"SenderAddress"
  | .maxFee => go!"MaxFee"
  | .signature => go!"TransactionSignature"
  | .nonce => go!"Nonce"
  | .version => go!"Version"
  | .compiledClassHash => go!"CompiledClassHash"
  | .resources => go!"ResourceBounds"
  | .tip => go!"Tip"
  | .paymaster => go!"PaymasterData"
  | .deployment => go!"AccountDeploymentData"
  | .nonceMode => go!"NonceDAMode"
  | .feeMode => go!"FeeDAMode"
def declareNames : List GoString := [go!"TransactionHash", go!"ClassHash", go!"SenderAddress", go!"MaxFee", go!"TransactionSignature", go!"Nonce", go!"Version", go!"CompiledClassHash", go!"ResourceBounds", go!"Tip", go!"PaymasterData", go!"AccountDeploymentData", go!"NonceDAMode", go!"FeeDAMode"]

theorem declareField_names : declareFields.map declareFieldName = declareNames := rfl

inductive DeclareFieldValid (d : Nat) : DeclareField → DeclareValue → List w8 → List w8 → DeclareValue → Prop where
  | hash {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .hash old item rest {old with hash := value}
  | classHash {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .classHash old item rest {old with classHash := value}
  | sender {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .sender old item rest {old with sender := value}
  | maxFee {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .maxFee old item rest {old with maxFee := value}
  | signature {old : DeclareValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeclareFieldValid d .signature old item rest {old with signature := value}
  | nonce {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .nonce old item rest {old with nonce := value}
  | version {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .version old item rest {old with version := value}
  | compiledClassHash {old : DeclareValue} {item rest value} (valid : FeltValid d item value) :
      DeclareFieldValid d .compiledClassHash old item rest {old with compiledClassHash := value}
  | resources {old : DeclareValue} {item rest value} (valid : ResourceMapSpec d (item ++ rest) (.success value rest)) :
      DeclareFieldValid d .resources old item rest {old with resources := value}
  | tip {old : DeclareValue} {item rest value} (valid : ScalarValid old.tip item value) :
      DeclareFieldValid d .tip old item rest {old with tip := value}
  | paymaster {old : DeclareValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeclareFieldValid d .paymaster old item rest {old with paymaster := value}
  | deployment {old : DeclareValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      DeclareFieldValid d .deployment old item rest {old with deployment := value}
  | nonceMode {old : DeclareValue} {item rest value} (valid : DAModeValid old.nonceMode item value) :
      DeclareFieldValid d .nonceMode old item rest {old with nonceMode := value}
  | feeMode {old : DeclareValue} {item rest value} (valid : DAModeValid old.feeMode item value) :
      DeclareFieldValid d .feeMode old item rest {old with feeMode := value}

inductive DeclareFieldFailure where
  | felt (reason : FeltFailureReason)
  | values (reason : FeltValuesFailureReason)
  | resources (reason : ResourceMapFailureReason)
  | scalar (reason : ExpectedFailureReason)
  | mode (reason : DAModeFailureReason)

/-- Failed fields preserve the destination. Map failures retain private child
    allocations and entered levels; DA-mode overflow consumes its integer. Its
    numeric failure position denotes consumption, not an allocated diagnostic. -/
inductive DeclareFieldRejected (d : Nat) : DeclareField → List w8 → Nat → Nat → Nat → DeclareFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .hash wire 0 offset 0 (.felt reason)
  | classHash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .classHash wire 0 offset 0 (.felt reason)
  | sender {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .sender wire 0 offset 0 (.felt reason)
  | maxFee {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .maxFee wire 0 offset 0 (.felt reason)
  | signature {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeclareFieldRejected d .signature wire 0 offset 0 (.values reason)
  | nonce {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .nonce wire 0 offset 0 (.felt reason)
  | version {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .version wire 0 offset 0 (.felt reason)
  | compiledClassHash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeclareFieldRejected d .compiledClassHash wire 0 offset 0 (.felt reason)
  | resources {wire consumed offset extraDepth reason} (failure : ResourceMapSpec d wire (.failure consumed offset extraDepth reason)) :
      DeclareFieldRejected d .resources wire consumed offset extraDepth (.resources reason)
  | tip {wire reason} (failure : ScalarFailure wire reason) :
      DeclareFieldRejected d .tip wire 0 0 0 (.scalar reason)
  | paymaster {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeclareFieldRejected d .paymaster wire 0 offset 0 (.values reason)
  | deployment {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      DeclareFieldRejected d .deployment wire 0 offset 0 (.values reason)
  | nonceMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      DeclareFieldRejected d .nonceMode wire consumed consumed 0 (.mode reason)
  | feeMode {wire consumed reason} (failure : DAModeRejected wire consumed reason) :
      DeclareFieldRejected d .feeMode wire consumed consumed 0 (.mode reason)

theorem declareField_classify (d : Nat) (field : DeclareField) (old : DeclareValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeclareFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, DeclareFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .hash failure⟩
  | classHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .classHash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .classHash failure⟩
  | sender =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .sender valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .sender failure⟩
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
  | version =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .version failure⟩
  | compiledClassHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .compiledClassHash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .compiledClassHash failure⟩
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

theorem declareField_nonempty (d : Nat) (field : DeclareField) (old final : DeclareValue)
    (item rest : List w8) (valid : DeclareFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | classHash valid => cases valid <;> simp
  | sender valid => cases valid <;> simp
  | maxFee valid => cases valid <;> simp
  | signature valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | nonce valid => cases valid <;> simp
  | version valid => cases valid <;> simp
  | compiledClassHash valid => cases valid <;> simp
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

theorem declareField_prefix_unique (d : Nat) (field : DeclareField) (old a b : DeclareValue)
    (left right restA restB : List w8)
    (first : DeclareFieldValid d field old left restA a) (second : DeclareFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid => cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | classHash valid => cases second with
    | classHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | sender valid => cases second with
    | sender valid2 =>
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
  | version valid => cases second with
    | version valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | compiledClassHash valid => cases second with
    | compiledClassHash valid2 =>
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

theorem declareField_not_rejected (d : Nat) (field : DeclareField) (old final : DeclareValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : DeclareFieldFailure)
    (valid : DeclareFieldValid d field old item rest final)
    (failure : DeclareFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | hash valid => cases failure with
    | hash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | classHash valid => cases failure with
    | classHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | sender valid => cases failure with
    | sender failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | maxFee valid => cases failure with
    | maxFee failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | signature valid => cases failure with
    | signature failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | nonce valid => cases failure with
    | nonce failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | version valid => cases failure with
    | version failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | compiledClassHash valid => cases failure with
    | compiledClassHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
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

theorem declareField_rejected_unique (d : Nat) (field : DeclareField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : DeclareFieldFailure)
    (first : DeclareFieldRejected d field wire consumedA offsetA depthA a)
    (second : DeclareFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | hash failure => cases second with
    | hash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | classHash failure => cases second with
    | classHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | sender failure => cases second with
    | sender failure2 =>
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
  | version failure => cases second with
    | version failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | compiledClassHash failure => cases second with
    | compiledClassHash failure2 =>
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

theorem declareField_rejected_bounds (d : Nat) (field : DeclareField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeclareFieldFailure)
    (failure : DeclareFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | hash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | classHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | sender failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | maxFee failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | signature failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | nonce failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | version failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | compiledClassHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | resources failure => exact resourceMap_failure_bounds _ _ _ _ _ _ failure
  | tip failure => omega
  | paymaster failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | deployment failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | nonceMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩
  | feeMode failure => exact ⟨Nat.le_refl _, daModeRejected_bound _ _ _ failure⟩

theorem declareField_rejected_depth (d : Nat) (field : DeclareField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeclareFieldFailure) (depth : d ≤ 32)
    (failure : DeclareFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | hash => omega
  | classHash => omega
  | sender => omega
  | maxFee => omega
  | signature => omega
  | nonce => omega
  | version => omega
  | compiledClassHash => omega
  | resources failure => exact resourceMap_failure_depth _ _ _ _ _ _ depth failure
  | tip => omega
  | paymaster => omega
  | deployment => omega
  | nonceMode => omega
  | feeMode => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.declareField_rejected_depth
