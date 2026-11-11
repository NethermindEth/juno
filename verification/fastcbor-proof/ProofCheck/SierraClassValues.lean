import ProofCheck.CasmClassPointerSpec
import ProofCheck.SierraEntriesRecordUnique
import ProofCheck.FeltValuesUnique
import ProofCheck.StringSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored Sierra class fields. Hash consistency, ABI syntax and executable
    validity are outside this decoding relation. -/
structure SierraClassValue where
  abi : GoString
  abiHash : Option (GoArray w64 4)
  entries : SierraEntriesValue
  program : Option (List (GoArray w64 4))
  programHash : Option (GoArray w64 4)
  version : GoString
  compiled : Option CasmClassValue

def sierraClassZero : SierraClassValue := ⟨go!"", none, sierraEntriesZero, none, none, go!"", none⟩

inductive SierraClassField where
  | abi | abiHash | entries | program | programHash | version | compiled
  deriving DecidableEq

def sierraClassFields : List SierraClassField := [.abi, .abiHash, .entries, .program, .programHash, .version, .compiled]
def sierraClassFieldName : SierraClassField → GoString
  | .abi => go!"Abi"
  | .abiHash => go!"AbiHash"
  | .entries => go!"EntryPoints"
  | .program => go!"Program"
  | .programHash => go!"ProgramHash"
  | .version => go!"SemanticVersion"
  | .compiled => go!"Compiled"
def sierraClassNames : List GoString := [go!"Abi", go!"AbiHash", go!"EntryPoints", go!"Program", go!"ProgramHash", go!"SemanticVersion", go!"Compiled"]

theorem sierraClassField_names : sierraClassFields.map sierraClassFieldName = sierraClassNames := rfl

inductive SierraClassFieldValid (d : Nat) : SierraClassField → SierraClassValue → List w8 → List w8 → SierraClassValue → Prop where
  | abi {old : SierraClassValue} {item rest value} (valid : StringValid old.abi item value) :
      SierraClassFieldValid d .abi old item rest {old with abi := value}
  | abiHash {old : SierraClassValue} {item rest value} (valid : FeltValid d item value) :
      SierraClassFieldValid d .abiHash old item rest {old with abiHash := value}
  | entries {old : SierraClassValue} {item rest value} (valid : SierraEntriesRecordValid d old.entries item rest value) :
      SierraClassFieldValid d .entries old item rest {old with entries := value}
  | program {old : SierraClassValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      SierraClassFieldValid d .program old item rest {old with program := value}
  | programHash {old : SierraClassValue} {item rest value} (valid : FeltValid d item value) :
      SierraClassFieldValid d .programHash old item rest {old with programHash := value}
  | version {old : SierraClassValue} {item rest value} (valid : StringValid old.version item value) :
      SierraClassFieldValid d .version old item rest {old with version := value}
  | compiled {old : SierraClassValue} {item rest value} (valid : CasmClassPointerValid d item rest value) :
      SierraClassFieldValid d .compiled old item rest {old with compiled := value}

inductive SierraClassFieldFailure where
  | string (reason : StringFailureReason)
  | felt (reason : FeltFailureReason)
  | entries (reason : SierraEntriesFailureReason)
  | felts (reason : FeltValuesFailureReason)
  | compiled (retained : CasmClassValue) (reason : CasmClassFailureReason)

/-- The embedded entry-point record retains partial updates on failure. Failed
    pointer/slice reads preserve their prior fields; failed Compiled allocations
    remain private and are described in the failure reason. -/
inductive SierraClassFieldRejected (d : Nat) : SierraClassField → SierraClassValue → List w8 → SierraClassValue →
    Nat → Nat → Nat → SierraClassFieldFailure → Prop where
  | abi {old : SierraClassValue} {wire reason} (failure : StringRejected wire reason) :
      SierraClassFieldRejected d .abi old wire old 0 0 0 (.string reason)
  | abiHash {old : SierraClassValue} {wire offset reason} (failure : FeltFailure d wire offset reason) :
      SierraClassFieldRejected d .abiHash old wire old 0 offset 0 (.felt reason)
  | entries {old : SierraClassValue} {wire value consumed offset extraDepth reason} (failure : SierraEntriesRecordRejected d old.entries wire value consumed offset extraDepth reason) :
      SierraClassFieldRejected d .entries old wire {old with entries := value} consumed offset extraDepth (.entries reason)
  | program {old : SierraClassValue} {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      SierraClassFieldRejected d .program old wire old 0 offset 0 (.felts reason)
  | programHash {old : SierraClassValue} {wire offset reason} (failure : FeltFailure d wire offset reason) :
      SierraClassFieldRejected d .programHash old wire old 0 offset 0 (.felt reason)
  | version {old : SierraClassValue} {wire reason} (failure : StringRejected wire reason) :
      SierraClassFieldRejected d .version old wire old 0 0 0 (.string reason)
  | compiled {old : SierraClassValue} {wire retained consumed offset extraDepth reason} (failure : CasmClassPointerRejected d wire retained consumed offset extraDepth reason) :
      SierraClassFieldRejected d .compiled old wire old consumed offset extraDepth (.compiled retained reason)

theorem sierraClassField_classify (d : Nat) (field : SierraClassField) (old : SierraClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SierraClassFieldValid d field old item rest final) ∨
    (∃ final consumed offset extraDepth reason, SierraClassFieldRejected d field old wire final consumed offset extraDepth reason) := by
  cases field with
  | abi =>
    rcases string_classify old.abi wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .abi valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .abi failure⟩
  | abiHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .abiHash valid⟩
    · exact Or.inr ⟨_, 0, offset, 0, _, .abiHash failure⟩
  | entries =>
    rcases sierraEntriesRecord_classify d old.entries wire depth with ⟨item, rest, value, split, valid⟩ | ⟨value, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .entries valid⟩
    · exact Or.inr ⟨_, consumed, offset, extraDepth, _, .entries failure⟩
  | program =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      subst wire
      exact Or.inl ⟨item, rest, _, rfl, .program spec⟩
    | failure offset reason => exact Or.inr ⟨_, 0, offset, 0, _, .program spec⟩
  | programHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .programHash valid⟩
    · exact Or.inr ⟨_, 0, offset, 0, _, .programHash failure⟩
  | version =>
    rcases string_classify old.version wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .version valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .version failure⟩
  | compiled =>
    rcases casmClassPointer_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .compiled valid⟩
    · exact Or.inr ⟨_, consumed, offset, extraDepth, _, .compiled failure⟩

theorem sierraClassField_nonempty (d : Nat) (field : SierraClassField) (old final : SierraClassValue)
    (item rest : List w8) (valid : SierraClassFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | abi valid => exact stringValid_nonempty _ _ _ valid
  | abiHash valid => cases valid <;> simp
  | entries valid => exact sierraEntriesRecord_nonempty _ _ _ _ _ valid
  | program valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | programHash valid => cases valid <;> simp
  | version valid => exact stringValid_nonempty _ _ _ valid
  | compiled valid => exact casmClassPointer_nonempty _ _ _ _ valid

theorem sierraClassField_prefix_unique (d : Nat) (field : SierraClassField) (old a b : SierraClassValue)
    (left right restA restB : List w8)
    (first : SierraClassFieldValid d field old left restA a) (second : SierraClassFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | abi valid => cases second with
    | abi valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | abiHash valid => cases second with
    | abiHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | entries valid => cases second with
    | entries valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := sierraEntriesRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | program valid => cases second with
    | program valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | programHash valid => cases second with
    | programHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | version valid => cases second with
    | version valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | compiled valid => cases second with
    | compiled valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := casmClassPointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem sierraClassField_not_rejected (d : Nat) (field : SierraClassField) (old final retained : SierraClassValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : SierraClassFieldFailure)
    (valid : SierraClassFieldValid d field old item rest final)
    (failure : SierraClassFieldRejected d field old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases valid with
  | abi valid => cases failure with
    | abi failure => exact stringValid_not_rejected _ _ _ _ _ valid failure
  | abiHash valid => cases failure with
    | abiHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | entries valid => cases failure with
    | entries failure => exact sierraEntriesRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure
  | program valid => cases failure with
    | program failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | programHash valid => cases failure with
    | programHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | version valid => cases failure with
    | version failure => exact stringValid_not_rejected _ _ _ _ _ valid failure
  | compiled valid => cases failure with
    | compiled failure => exact casmClassPointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure

theorem sierraClassField_rejected_unique (d : Nat) (field : SierraClassField) (old a b : SierraClassValue) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (reasonA reasonB : SierraClassFieldFailure)
    (first : SierraClassFieldRejected d field old wire a consumedA offsetA depthA reasonA)
    (second : SierraClassFieldRejected d field old wire b consumedB offsetB depthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB := by
  cases first with
  | abi failure => cases second with
    | abi failure2 =>
      obtain rfl := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | abiHash failure => cases second with
    | abiHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | entries failure => cases second with
    | entries failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := sierraEntriesRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | program failure => cases second with
    | program failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | programHash failure => cases second with
    | programHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | version failure => cases second with
    | version failure2 =>
      obtain rfl := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | compiled failure => cases second with
    | compiled failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := casmClassPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

theorem sierraClassField_rejected_bounds (d : Nat) (field : SierraClassField) (old retained : SierraClassValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : SierraClassFieldFailure)
    (failure : SierraClassFieldRejected d field old wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | abi failure => omega
  | abiHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | entries failure => exact sierraEntriesRecordRejected_bounds _ _ _ _ _ _ _ _ failure
  | program failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | programHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | version failure => omega
  | compiled failure => exact casmClassPointerRejected_bounds _ _ _ _ _ _ _ failure

theorem sierraClassField_rejected_depth (d : Nat) (field : SierraClassField) (old retained : SierraClassValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : SierraClassFieldFailure) (depth : d ≤ 32)
    (failure : SierraClassFieldRejected d field old wire retained consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | abi failure => omega
  | abiHash failure => omega
  | entries failure => exact sierraEntriesRecordRejected_depth _ _ _ _ _ _ _ _ depth failure
  | program failure => omega
  | programHash failure => omega
  | version failure => omega
  | compiled failure => cases failure with
    | record hn hu failure => exact casmClassRecordRejected_depth _ _ _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassField_rejected_depth
