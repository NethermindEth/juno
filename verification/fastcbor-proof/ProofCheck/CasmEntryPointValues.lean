import ProofCheck.FeltValuesUnique
import ProofCheck.StringsUnique
import ProofCheck.ExpectedFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored CASM entry point: instruction offset, nullable builtin-name array,
    and nullable selector limbs. This describes decoding, not executable validity. -/
structure CasmEntryPointValue where
  offset : w64
  builtins : Option (List GoString)
  selector : Option (GoArray w64 4)

def casmEntryPointZero : CasmEntryPointValue := ⟨W64 0, none, none⟩

inductive CasmEntryPointField where
  | offset | builtins | selector
  deriving DecidableEq

def casmEntryPointFields : List CasmEntryPointField := [.offset, .builtins, .selector]
def casmEntryPointFieldName : CasmEntryPointField → GoString
  | .offset => go!"Offset"
  | .builtins => go!"Builtins"
  | .selector => go!"Selector"
def casmEntryPointNames : List GoString := [go!"Offset", go!"Builtins", go!"Selector"]

theorem casmEntryPointField_names : casmEntryPointFields.map casmEntryPointFieldName = casmEntryPointNames := rfl

inductive CasmEntryPointFieldValid (d : Nat) : CasmEntryPointField → CasmEntryPointValue → List w8 → List w8 → CasmEntryPointValue → Prop where
  | offset {old : CasmEntryPointValue} {item rest value} (valid : ScalarValid old.offset item value) :
      CasmEntryPointFieldValid d .offset old item rest {old with offset := value}
  | builtins {old : CasmEntryPointValue} {item rest value} (valid : StringsSpec d (item ++ rest) (.success value rest)) :
      CasmEntryPointFieldValid d .builtins old item rest {old with builtins := value}
  | selector {old : CasmEntryPointValue} {item rest value} (valid : FeltValid d item value) :
      CasmEntryPointFieldValid d .selector old item rest {old with selector := value}

inductive CasmEntryPointFieldFailure where
  | felt (reason : FeltFailureReason)
  | strings (reason : StringsFailureReason)
  | scalar (reason : ExpectedFailureReason)

/-- Failed fields preserve their old value. A failed builtin array retains
    its successful prefix's consumption and entered array depth; scalar and
    felt failures roll back their own cursor, with exact diagnostic offsets. -/
inductive CasmEntryPointFieldRejected (d : Nat) : CasmEntryPointField → List w8 → Nat → Nat → Nat → CasmEntryPointFieldFailure → Prop where
  | offset {wire reason} (failure : ScalarFailure wire reason) :
      CasmEntryPointFieldRejected d .offset wire 0 0 0 (.scalar reason)
  | builtins {wire offset extraDepth reason} (failure : StringsSpec d wire (.failure offset extraDepth reason)) :
      CasmEntryPointFieldRejected d .builtins wire offset offset extraDepth (.strings reason)
  | selector {wire offset reason} (failure : FeltFailure d wire offset reason) :
      CasmEntryPointFieldRejected d .selector wire 0 offset 0 (.felt reason)

theorem casmEntryPointField_classify (d : Nat) (field : CasmEntryPointField) (old : CasmEntryPointValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ CasmEntryPointFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, CasmEntryPointFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | offset =>
    rcases scalar_classify old.offset wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .offset valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .offset failure⟩
  | builtins =>
    obtain ⟨outcome, spec⟩ := strings_classify d wire depth
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := strings_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .builtins spec⟩
    | failure offset extraDepth reason => exact Or.inr ⟨offset, offset, extraDepth, _, .builtins spec⟩
  | selector =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .selector valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .selector failure⟩

theorem casmEntryPointField_nonempty (d : Nat) (field : CasmEntryPointField) (old final : CasmEntryPointValue)
    (item rest : List w8) (valid : CasmEntryPointFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | offset valid => cases valid <;> simp
  | builtins valid =>
    obtain ⟨other, same, positive⟩ := strings_success_suffix d (item ++ rest) rest _ valid
    have eq := List.append_cancel_right same
    subst other
    exact positive
  | selector valid => cases valid <;> simp

theorem casmEntryPointField_prefix_unique (d : Nat) (field : CasmEntryPointField) (old a b : CasmEntryPointValue)
    (left right restA restB : List w8)
    (first : CasmEntryPointFieldValid d field old left restA a) (second : CasmEntryPointFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | offset valid => cases second with
    | offset valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | builtins valid => cases second with
    | builtins valid2 =>
      rw [← same] at valid2
      have eq := stringsSpec_unique d (left ++ restA) _ _ valid valid2
      cases eq
      have eq := List.append_cancel_right same
      subst right
      exact ⟨rfl, rfl, rfl⟩
  | selector valid => cases second with
    | selector valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem casmEntryPointField_not_rejected (d : Nat) (field : CasmEntryPointField) (old final : CasmEntryPointValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : CasmEntryPointFieldFailure)
    (valid : CasmEntryPointFieldValid d field old item rest final)
    (failure : CasmEntryPointFieldRejected d field (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | offset valid => cases failure with
    | offset failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | builtins valid => cases failure with
    | builtins failure => have eq := stringsSpec_unique d (item ++ rest) _ _ valid failure; cases eq
  | selector valid => cases failure with
    | selector failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure

theorem casmEntryPointField_rejected_unique (d : Nat) (field : CasmEntryPointField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : CasmEntryPointFieldFailure)
    (first : CasmEntryPointFieldRejected d field wire consumedA offsetA depthA a)
    (second : CasmEntryPointFieldRejected d field wire consumedB offsetB depthB b) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | offset failure => cases second with
    | offset failure2 =>
      obtain rfl := scalarFailure_reason_unique _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | builtins failure => cases second with
    | builtins failure2 =>
      have eq := stringsSpec_unique d wire _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl⟩
  | selector failure => cases second with
    | selector failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem casmEntryPointField_rejected_bounds (d : Nat) (field : CasmEntryPointField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : CasmEntryPointFieldFailure)
    (failure : CasmEntryPointFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | offset failure => omega
  | builtins failure => exact ⟨Nat.le_refl _, strings_failure_bounds _ _ _ _ _ failure⟩
  | selector failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩

theorem casmEntryPointField_rejected_depth (d : Nat) (field : CasmEntryPointField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : CasmEntryPointFieldFailure) (depth : d ≤ 32)
    (failure : CasmEntryPointFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | offset => omega
  | builtins failure => exact strings_failure_depth _ _ _ _ _ depth failure
  | selector => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointField_rejected_depth
