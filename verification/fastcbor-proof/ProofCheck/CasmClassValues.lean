import ProofCheck.SegmentGeneral
import ProofCheck.CasmEntryValuesUnique
import ProofCheck.FeltValuesUnique
import ProofCheck.RawMessageSpec
import ProofCheck.BigIntPointerSpec
import ProofCheck.StringSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored CASM fields, without asserting executable validity, prime choice,
    JSON validity, or consistency between bytecode and segment lengths. -/
structure CasmClassValue where
  bytecode : Option (List (GoArray w64 4))
  pythonicHints : Option (List w8)
  compilerVersion : GoString
  hints : Option (List w8)
  prime : Option Int
  external : Option (List CasmEntryPointValue)
  l1Handler : Option (List CasmEntryPointValue)
  constructor : Option (List CasmEntryPointValue)
  segments : SegmentValue

def casmClassZero : CasmClassValue := ⟨none, none, go!"", none, none, none, none, none, segmentZero⟩

inductive CasmClassField where
  | bytecode | pythonicHints | compilerVersion | hints | prime | external | l1Handler | constructor | segments
  deriving DecidableEq

def casmClassFields : List CasmClassField := [.bytecode, .pythonicHints, .compilerVersion, .hints, .prime, .external, .l1Handler, .constructor, .segments]
def casmClassFieldName : CasmClassField → GoString
  | .bytecode => go!"Bytecode"
  | .pythonicHints => go!"PythonicHints"
  | .compilerVersion => go!"CompilerVersion"
  | .hints => go!"Hints"
  | .prime => go!"Prime"
  | .external => go!"External"
  | .l1Handler => go!"L1Handler"
  | .constructor => go!"Constructor"
  | .segments => go!"BytecodeSegmentLengths"
def casmClassNames : List GoString := [go!"Bytecode", go!"PythonicHints", go!"CompilerVersion", go!"Hints", go!"Prime", go!"External", go!"L1Handler", go!"Constructor", go!"BytecodeSegmentLengths"]

theorem casmClassField_names : casmClassFields.map casmClassFieldName = casmClassNames := rfl

inductive CasmClassFieldValid (d : Nat) : CasmClassField → CasmClassValue → List w8 → List w8 → CasmClassValue → Prop where
  | bytecode {old : CasmClassValue} {item rest value} (valid : FeltValuesSpec d (item ++ rest) (.success value rest)) :
      CasmClassFieldValid d .bytecode old item rest {old with bytecode := value}
  | pythonicHints {old : CasmClassValue} {item rest value} (valid : RawMessageValid item value) :
      CasmClassFieldValid d .pythonicHints old item rest {old with pythonicHints := value}
  | compilerVersion {old : CasmClassValue} {item rest value} (valid : StringValid old.compilerVersion item value) :
      CasmClassFieldValid d .compilerVersion old item rest {old with compilerVersion := value}
  | hints {old : CasmClassValue} {item rest value} (valid : RawMessageValid item value) :
      CasmClassFieldValid d .hints old item rest {old with hints := value}
  | prime {old : CasmClassValue} {item rest value} (valid : BigIntPointerValid item value) :
      CasmClassFieldValid d .prime old item rest {old with prime := value}
  | external {old : CasmClassValue} {item rest value} (valid : CasmEntryValuesSpec d (item ++ rest) (.success value rest)) :
      CasmClassFieldValid d .external old item rest {old with external := value}
  | l1Handler {old : CasmClassValue} {item rest value} (valid : CasmEntryValuesSpec d (item ++ rest) (.success value rest)) :
      CasmClassFieldValid d .l1Handler old item rest {old with l1Handler := value}
  | constructor {old : CasmClassValue} {item rest value} (valid : CasmEntryValuesSpec d (item ++ rest) (.success value rest)) :
      CasmClassFieldValid d .constructor old item rest {old with constructor := value}
  | segments {old : CasmClassValue} {item rest value} (valid : SegmentRecordValid d old.segments item rest value) :
      CasmClassFieldValid d .segments old item rest {old with segments := value}

inductive CasmClassFieldFailure where
  | felts (reason : FeltValuesFailureReason)
  | bytes (reason : BytesFailureReason)
  | string (reason : StringFailureReason)
  | bigint (reason : BigIntFailureReason)
  | entries (reason : CasmEntryValuesFailureReason)
  | segment (reason : SegmentFailureReason)

/-- Failed slices and pointers retain their old fields. The embedded recursive
    segment record can retain partial changes, which are reflected in the class. -/
inductive CasmClassFieldRejected (d : Nat) : CasmClassField → CasmClassValue → List w8 → CasmClassValue →
    Nat → Nat → Nat → CasmClassFieldFailure → Prop where
  | bytecode {old : CasmClassValue} {wire offset reason} (failure : FeltValuesSpec d wire (.failure offset reason)) :
      CasmClassFieldRejected d .bytecode old wire old 0 offset 0 (.felts reason)
  | pythonicHints {old : CasmClassValue} {wire reason} (failure : RawMessageRejected wire reason) :
      CasmClassFieldRejected d .pythonicHints old wire old 0 0 0 (.bytes reason)
  | compilerVersion {old : CasmClassValue} {wire reason} (failure : StringRejected wire reason) :
      CasmClassFieldRejected d .compilerVersion old wire old 0 0 0 (.string reason)
  | hints {old : CasmClassValue} {wire reason} (failure : RawMessageRejected wire reason) :
      CasmClassFieldRejected d .hints old wire old 0 0 0 (.bytes reason)
  | prime {old : CasmClassValue} {wire reason} (failure : BigIntPointerRejected wire reason) :
      CasmClassFieldRejected d .prime old wire old 0 (bigIntDiagnosticOffset reason) 0 (.bigint reason)
  | external {old : CasmClassValue} {wire consumed offset extraDepth reason} (failure : CasmEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      CasmClassFieldRejected d .external old wire old consumed offset extraDepth (.entries reason)
  | l1Handler {old : CasmClassValue} {wire consumed offset extraDepth reason} (failure : CasmEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      CasmClassFieldRejected d .l1Handler old wire old consumed offset extraDepth (.entries reason)
  | constructor {old : CasmClassValue} {wire consumed offset extraDepth reason} (failure : CasmEntryValuesSpec d wire (.failure consumed offset extraDepth reason)) :
      CasmClassFieldRejected d .constructor old wire old consumed offset extraDepth (.entries reason)
  | segments {old : CasmClassValue} {wire value consumed offset extraDepth reason} (failure : SegmentRecordRejected d old.segments wire value consumed offset extraDepth reason) :
      CasmClassFieldRejected d .segments old wire {old with segments := value} consumed offset extraDepth (.segment reason)

theorem casmClassField_classify (d : Nat) (field : CasmClassField) (old : CasmClassValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmClassFieldValid d field old item rest final) ∨
    (∃ final consumed offset extraDepth reason, CasmClassFieldRejected d field old wire final consumed offset extraDepth reason) := by
  cases field with
  | bytecode =>
    obtain ⟨outcome, spec⟩ := feltValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := feltValues_success_suffix d wire rest value spec
      subst wire
      exact Or.inl ⟨item, rest, _, rfl, .bytecode spec⟩
    | failure offset reason => exact Or.inr ⟨_, 0, offset, 0, _, .bytecode spec⟩
  | pythonicHints =>
    rcases rawMessage_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .pythonicHints valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .pythonicHints failure⟩
  | compilerVersion =>
    rcases string_classify old.compilerVersion wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .compilerVersion valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .compilerVersion failure⟩
  | hints =>
    rcases rawMessage_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hints valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .hints failure⟩
  | prime =>
    rcases bigIntPointer_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .prime valid⟩
    · exact Or.inr ⟨_, 0, bigIntDiagnosticOffset reason, 0, _, .prime failure⟩
  | external =>
    obtain ⟨outcome, spec⟩ := casmEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := casmEntryValues_success_suffix d wire rest value spec
      subst wire
      exact Or.inl ⟨item, rest, _, rfl, .external spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨_, consumed, offset, extraDepth, _, .external spec⟩
  | l1Handler =>
    obtain ⟨outcome, spec⟩ := casmEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := casmEntryValues_success_suffix d wire rest value spec
      subst wire
      exact Or.inl ⟨item, rest, _, rfl, .l1Handler spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨_, consumed, offset, extraDepth, _, .l1Handler spec⟩
  | constructor =>
    obtain ⟨outcome, spec⟩ := casmEntryValues_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := casmEntryValues_success_suffix d wire rest value spec
      subst wire
      exact Or.inl ⟨item, rest, _, rfl, .constructor spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨_, consumed, offset, extraDepth, _, .constructor spec⟩
  | segments =>
    rcases segmentRecord_classify d old.segments wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨value, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .segments valid⟩
    · exact Or.inr ⟨_, consumed, offset, extraDepth, _, .segments failure⟩

theorem casmClassField_nonempty (d : Nat) (field : CasmClassField) (old final : CasmClassValue)
    (item rest : List w8) (valid : CasmClassFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | bytecode valid =>
    obtain ⟨other, same, positive⟩ := feltValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | pythonicHints valid => exact rawMessageValid_nonempty _ _ valid
  | compilerVersion valid => exact stringValid_nonempty _ _ _ valid
  | hints valid => exact rawMessageValid_nonempty _ _ valid
  | prime valid => exact bigIntPointerValid_nonempty _ _ valid
  | external valid =>
    obtain ⟨other, same, positive⟩ := casmEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | l1Handler valid =>
    obtain ⟨other, same, positive⟩ := casmEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | constructor valid =>
    obtain ⟨other, same, positive⟩ := casmEntryValues_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | segments valid => exact segmentRecord_nonempty _ _ _ _ _ valid

theorem casmClassField_prefix_unique (d : Nat) (field : CasmClassField) (old a b : CasmClassValue)
    (left right restA restB : List w8)
    (first : CasmClassFieldValid d field old left restA a) (second : CasmClassFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | bytecode valid => cases second with
    | bytecode valid2 =>
      rw [← same] at valid2
      have eq := feltValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | pythonicHints valid => cases second with
    | pythonicHints valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := rawMessageValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | compilerVersion valid => cases second with
    | compilerVersion valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stringValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | hints valid => cases second with
    | hints valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := rawMessageValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | prime valid => cases second with
    | prime valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := bigIntPointerValid_prefix_unique _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | external valid => cases second with
    | external valid2 =>
      rw [← same] at valid2
      have eq := casmEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | l1Handler valid => cases second with
    | l1Handler valid2 =>
      rw [← same] at valid2
      have eq := casmEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | constructor valid => cases second with
    | constructor valid2 =>
      rw [← same] at valid2
      have eq := casmEntryValuesSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      exact ⟨List.append_cancel_right same, rfl, rfl⟩
  | segments valid => cases second with
    | segments valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := (segmentRecord_deterministic_any_depth d).successful _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem casmClassField_not_rejected (d : Nat) (field : CasmClassField) (old final retained : CasmClassValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : CasmClassFieldFailure)
    (valid : CasmClassFieldValid d field old item rest final)
    (failure : CasmClassFieldRejected d field old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases valid with
  | bytecode valid => cases failure with
    | bytecode failure => have eq := feltValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | pythonicHints valid => cases failure with
    | pythonicHints failure => exact rawMessageValid_not_rejected _ _ _ _ valid failure
  | compilerVersion valid => cases failure with
    | compilerVersion failure => exact stringValid_not_rejected _ _ _ _ _ valid failure
  | hints valid => cases failure with
    | hints failure => exact rawMessageValid_not_rejected _ _ _ _ valid failure
  | prime valid => cases failure with
    | prime failure => exact bigIntPointerValid_not_rejected _ _ _ _ valid failure
  | external valid => cases failure with
    | external failure => have eq := casmEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | l1Handler valid => cases failure with
    | l1Handler failure => have eq := casmEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | constructor valid => cases failure with
    | constructor failure => have eq := casmEntryValuesSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | segments valid => cases failure with
    | segments failure => exact (segmentRecord_deterministic_any_depth d).disjoint _ _ _ _ _ _ _ _ _ valid failure

theorem casmClassField_rejected_unique (d : Nat) (field : CasmClassField) (old a b : CasmClassValue) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (reasonA reasonB : CasmClassFieldFailure)
    (first : CasmClassFieldRejected d field old wire a consumedA offsetA depthA reasonA)
    (second : CasmClassFieldRejected d field old wire b consumedB offsetB depthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB := by
  cases first with
  | bytecode failure => cases second with
    | bytecode failure2 =>
      have eq := feltValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | pythonicHints failure => cases second with
    | pythonicHints failure2 =>
      obtain rfl := bytesRejected_unique _ _ _ failure.2.2 failure2.2.2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | compilerVersion failure => cases second with
    | compilerVersion failure2 =>
      obtain rfl := stringRejected_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | hints failure => cases second with
    | hints failure2 =>
      obtain rfl := bytesRejected_unique _ _ _ failure.2.2 failure2.2.2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | prime failure => cases second with
    | prime failure2 =>
      obtain rfl := bigIntRejected_unique _ _ _ failure.2.2 failure2.2.2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | external failure => cases second with
    | external failure2 =>
      have eq := casmEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | l1Handler failure => cases second with
    | l1Handler failure2 =>
      have eq := casmEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | constructor failure => cases second with
    | constructor failure2 =>
      have eq := casmEntryValuesSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | segments failure => cases second with
    | segments failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := (segmentRecord_deterministic_any_depth d).rejected _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

theorem casmClassField_rejected_bounds (d : Nat) (field : CasmClassField) (old retained : CasmClassValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : CasmClassFieldFailure)
    (failure : CasmClassFieldRejected d field old wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | bytecode failure => exact ⟨Nat.zero_le _, feltValues_failure_bound _ _ _ _ failure⟩
  | pythonicHints failure => omega
  | compilerVersion failure => omega
  | hints failure => omega
  | prime failure => exact ⟨Nat.zero_le _, bigIntPointerRejected_offset_bounds _ _ failure⟩
  | external failure => exact casmEntryValues_failure_bounds _ _ _ _ _ _ failure
  | l1Handler failure => exact casmEntryValues_failure_bounds _ _ _ _ _ _ failure
  | constructor failure => exact casmEntryValues_failure_bounds _ _ _ _ _ _ failure
  | segments failure => exact segmentRecordRejected_position_bounds _ _ _ _ _ _ _ _ failure

theorem casmClassField_rejected_depth (d : Nat) (field : CasmClassField) (old retained : CasmClassValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : CasmClassFieldFailure) (depth : d ≤ 32)
    (failure : CasmClassFieldRejected d field old wire retained consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | bytecode failure => omega
  | pythonicHints failure => omega
  | compilerVersion failure => omega
  | hints failure => omega
  | prime failure => omega
  | external failure => exact casmEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | l1Handler failure => exact casmEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | constructor failure => exact casmEntryValues_failure_depth _ _ _ _ _ _ depth failure
  | segments failure => exact (segmentRecordRejected_bounds d depth _ _ _ _ _ _ _ failure).2.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassField_rejected_depth
