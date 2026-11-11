import ProofCheck.RawMessageSpec
import ProofCheck.DeprecatedEntryValuesUnique
import ProofCheck.OpeningFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored legacy class fields. ABI bytes are opaque; the program is decoded
    as text, without validating base64, compression or executable contents. -/
structure DeprecatedClassValue where
  abi : Option (List w8)
  externals : Option (List DeprecatedEntryPointValue)
  l1Handlers : Option (List DeprecatedEntryPointValue)
  constructors : Option (List DeprecatedEntryPointValue)
  program : GoString

def deprecatedClassZero : DeprecatedClassValue := ⟨none, none, none, none, go!""⟩

inductive DeprecatedClassField where
  | abi | externals | l1Handlers | constructors | program
  deriving DecidableEq

def deprecatedClassFields : List DeprecatedClassField := [.abi, .externals, .l1Handlers, .constructors, .program]
def deprecatedClassFieldName : DeprecatedClassField → GoString
  | .abi => go!"Abi"
  | .externals => go!"Externals"
  | .l1Handlers => go!"L1Handlers"
  | .constructors => go!"Constructors"
  | .program => go!"Program"
def deprecatedClassNames : List GoString := [go!"Abi", go!"Externals", go!"L1Handlers", go!"Constructors", go!"Program"]

theorem deprecatedClassField_names : deprecatedClassFields.map deprecatedClassFieldName = deprecatedClassNames := rfl

inductive DeprecatedClassFieldValid (d : Nat) : DeprecatedClassField → DeprecatedClassValue → List w8 → List w8 → DeprecatedClassValue → Prop where
  | abi {old : DeprecatedClassValue} {item rest value} (valid : RawMessageValid item value) :
      DeprecatedClassFieldValid d .abi old item rest {old with abi := value}
  | externals {old : DeprecatedClassValue} {item rest value} (valid : DeprecatedEntryValuesSpec d (item ++ rest) (.success value rest)) :
      DeprecatedClassFieldValid d .externals old item rest {old with externals := value}
  | l1Handlers {old : DeprecatedClassValue} {item rest value} (valid : DeprecatedEntryValuesSpec d (item ++ rest) (.success value rest)) :
      DeprecatedClassFieldValid d .l1Handlers old item rest {old with l1Handlers := value}
  | constructors {old : DeprecatedClassValue} {item rest value} (valid : DeprecatedEntryValuesSpec d (item ++ rest) (.success value rest)) :
      DeprecatedClassFieldValid d .constructors old item rest {old with constructors := value}
  | program {old : DeprecatedClassValue} {item rest value} (valid : StringValid old.program item value) :
      DeprecatedClassFieldValid d .program old item rest {old with program := value}

inductive DeprecatedClassFieldFailure where
  | bytes (reason : BytesFailureReason)
  | entries (reason : DeprecatedEntryValuesFailureReason)
  | string (reason : StringFailureReason)

/-- Failed fields preserve their previous value. Array failures can retain
    successful-prefix consumption and entered child levels; bytes and text
    failures roll their own cursor back to the field's starting position. -/
inductive DeprecatedClassFieldRejected (d : Nat) : DeprecatedClassField → List w8 → Nat → Nat → Nat → DeprecatedClassFieldFailure → Prop where
  | abi {wire reason} (failure : RawMessageRejected wire reason) :
      DeprecatedClassFieldRejected d .abi wire 0 0 0 (.bytes reason)
  | externals {wire consumed offset extraDepth reason} (failure : DeprecatedEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      DeprecatedClassFieldRejected d .externals wire consumed offset extraDepth (.entries reason)
  | l1Handlers {wire consumed offset extraDepth reason} (failure : DeprecatedEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      DeprecatedClassFieldRejected d .l1Handlers wire consumed offset extraDepth (.entries reason)
  | constructors {wire consumed offset extraDepth reason} (failure : DeprecatedEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      DeprecatedClassFieldRejected d .constructors wire consumed offset extraDepth (.entries reason)
  | program {wire reason} (failure : StringRejected wire reason) :
      DeprecatedClassFieldRejected d .program wire 0 0 0 (.string reason)

theorem deprecatedClassField_classify (d : Nat) (field : DeprecatedClassField) (old : DeprecatedClassValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedClassFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, DeprecatedClassFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | abi =>
    rcases rawMessage_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .abi valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .abi failure⟩
  | externals =>
    obtain ⟨outcome, spec⟩ := deprecatedEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := deprecatedEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .externals spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .externals spec⟩
  | l1Handlers =>
    obtain ⟨outcome, spec⟩ := deprecatedEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := deprecatedEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .l1Handlers spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .l1Handlers spec⟩
  | constructors =>
    obtain ⟨outcome, spec⟩ := deprecatedEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := deprecatedEntryValues_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .constructors spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .constructors spec⟩
  | program =>
    rcases string_classify old.program wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .program valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .program failure⟩

theorem deprecatedClassField_nonempty (d : Nat) (field : DeprecatedClassField) (old final : DeprecatedClassValue)
    (item rest : List w8) (valid : DeprecatedClassFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | abi valid => exact rawMessageValid_nonempty _ _ valid
  | externals valid =>
    obtain ⟨other, same, positive⟩ := deprecatedEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | l1Handlers valid =>
    obtain ⟨other, same, positive⟩ := deprecatedEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | constructors valid =>
    obtain ⟨other, same, positive⟩ := deprecatedEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | program valid => exact stringValid_nonempty _ _ _ valid

theorem deprecatedClassField_prefix_unique (d : Nat) (field : DeprecatedClassField) (old a b : DeprecatedClassValue)
    (left right restA restB : List w8)
    (first : DeprecatedClassFieldValid d field old left restA a) (second : DeprecatedClassFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | abi valid => cases second with
    | abi valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := rawMessageValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | externals valid => cases second with
    | externals valid2 =>
      rw [← same] at valid2
      have eq := deprecatedEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | l1Handlers valid => cases second with
    | l1Handlers valid2 =>
      rw [← same] at valid2
      have eq := deprecatedEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | constructors valid => cases second with
    | constructors valid2 =>
      rw [← same] at valid2
      have eq := deprecatedEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | program valid => cases second with
    | program valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem deprecatedClassField_not_rejected (d : Nat) (field : DeprecatedClassField) (old final : DeprecatedClassValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : DeprecatedClassFieldFailure)
    (valid : DeprecatedClassFieldValid d field old item rest final)
    (failure : DeprecatedClassFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | abi valid => cases failure with
    | abi failure => exact rawMessageValid_not_rejected _ _ _ _ valid failure
  | externals valid => cases failure with
    | externals failure => have eq := deprecatedEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | l1Handlers valid => cases failure with
    | l1Handlers failure => have eq := deprecatedEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | constructors valid => cases failure with
    | constructors failure => have eq := deprecatedEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | program valid => cases failure with
    | program failure => exact stringValid_not_rejected _ _ _ _ _ valid failure

theorem deprecatedClassField_rejected_unique (d : Nat) (field : DeprecatedClassField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : DeprecatedClassFieldFailure)
    (first : DeprecatedClassFieldRejected d field wire consumedA offsetA depthA a)
    (second : DeprecatedClassFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | abi failure => cases second with
    | abi failure2 =>
      obtain rfl := bytesRejected_unique _ _ _ failure.2.2 failure2.2.2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | externals failure => cases second with
    | externals failure2 =>
      have eq := deprecatedEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | l1Handlers failure => cases second with
    | l1Handlers failure2 =>
      have eq := deprecatedEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | constructors failure => cases second with
    | constructors failure2 =>
      have eq := deprecatedEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | program failure => cases second with
    | program failure2 =>
      obtain rfl := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem deprecatedClassField_rejected_bounds (d : Nat) (field : DeprecatedClassField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeprecatedClassFieldFailure)
    (failure : DeprecatedClassFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | abi => omega
  | externals failure => exact deprecatedEntryValues_failure_bounds _ _ _ _ _ _ failure
  | l1Handlers failure => exact deprecatedEntryValues_failure_bounds _ _ _ _ _ _ failure
  | constructors failure => exact deprecatedEntryValues_failure_bounds _ _ _ _ _ _ failure
  | program => omega

theorem deprecatedClassField_rejected_depth (d : Nat) (field : DeprecatedClassField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : DeprecatedClassFieldFailure) (depth : d ≤ 32)
    (failure : DeprecatedClassFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | abi => omega
  | externals failure => exact deprecatedEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | l1Handlers failure => exact deprecatedEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | constructors failure => exact deprecatedEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | program => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassField_rejected_depth
