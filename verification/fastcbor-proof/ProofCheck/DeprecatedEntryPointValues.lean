import ProofCheck.FeltFailureUnique
import ProofCheck.FeltDeterminism
import ProofCheck.FeltDisjoint
import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent stored deprecated entry point. Both the selector and offset
    retain nullable four-limb storage; no integer-width or instruction-validity
    restriction is imposed on these stored felts. -/
structure DeprecatedEntryPointValue where
  selector : Option (GoArray w64 4)
  offset : Option (GoArray w64 4)

def deprecatedEntryPointZero : DeprecatedEntryPointValue := ⟨none, none⟩
inductive DeprecatedEntryPointField where
  | selector | offset
  deriving DecidableEq

def deprecatedEntryPointFields : List DeprecatedEntryPointField := [.selector, .offset]
def deprecatedEntryPointFieldName : DeprecatedEntryPointField → GoString
  | .selector => go!"Selector"
  | .offset => go!"Offset"
def deprecatedEntryPointNames : List GoString := [go!"Selector", go!"Offset"]

theorem deprecatedEntryPointField_names : deprecatedEntryPointFields.map deprecatedEntryPointFieldName = deprecatedEntryPointNames := rfl

inductive DeprecatedEntryPointFieldValid (d : Nat) : DeprecatedEntryPointField → DeprecatedEntryPointValue → List w8 → List w8 → DeprecatedEntryPointValue → Prop where
  | selector {old : DeprecatedEntryPointValue} {item rest value} (valid : FeltValid d item value) :
      DeprecatedEntryPointFieldValid d .selector old item rest {old with selector := value}
  | offset {old : DeprecatedEntryPointValue} {item rest value} (valid : FeltValid d item value) :
      DeprecatedEntryPointFieldValid d .offset old item rest {old with offset := value}

inductive DeprecatedEntryPointFieldFailure where
  | felt (reason : FeltFailureReason)

inductive DeprecatedEntryPointFieldRejected (d : Nat) : DeprecatedEntryPointField → List w8 → Nat → DeprecatedEntryPointFieldFailure → Prop where
  | selector {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeprecatedEntryPointFieldRejected d .selector wire offset (.felt reason)
  | offset {wire offset reason} (failure : FeltFailure d wire offset reason) :
      DeprecatedEntryPointFieldRejected d .offset wire offset (.felt reason)

theorem deprecatedEntryPointField_classify (d : Nat) (field : DeprecatedEntryPointField) (old : DeprecatedEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ DeprecatedEntryPointFieldValid d field old item rest final) ∨
    (∃ offset reason, DeprecatedEntryPointFieldRejected d field wire offset reason) := by
  cases field with
  | selector =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .selector valid⟩
    · exact Or.inr ⟨offset, _, .selector failure⟩
  | offset =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .offset valid⟩
    · exact Or.inr ⟨offset, _, .offset failure⟩

theorem deprecatedEntryPointField_nonempty (d : Nat) (field : DeprecatedEntryPointField) (old final : DeprecatedEntryPointValue)
    (item rest : List w8) (valid : DeprecatedEntryPointFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | selector valid => cases valid <;> simp
  | offset valid => cases valid <;> simp

theorem deprecatedEntryPointField_prefix_unique (d : Nat) (field : DeprecatedEntryPointField) (old a b : DeprecatedEntryPointValue)
    (left right restA restB : List w8)
    (first : DeprecatedEntryPointFieldValid d field old left restA a) (second : DeprecatedEntryPointFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | selector valid => cases second with
    | selector valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | offset valid => cases second with
    | offset valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem deprecatedEntryPointField_not_rejected (d : Nat) (field : DeprecatedEntryPointField) (old final : DeprecatedEntryPointValue)
    (item rest : List w8) (offset : Nat) (reason : DeprecatedEntryPointFieldFailure)
    (valid : DeprecatedEntryPointFieldValid d field old item rest final)
    (failure : DeprecatedEntryPointFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | selector valid => cases failure with
    | selector failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | offset valid => cases failure with
    | offset failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure

theorem deprecatedEntryPointField_rejected_unique (d : Nat) (field : DeprecatedEntryPointField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : DeprecatedEntryPointFieldFailure)
    (first : DeprecatedEntryPointFieldRejected d field wire offsetA a) (second : DeprecatedEntryPointFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | selector failure => cases second with
    | selector failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩
  | offset failure => cases second with
    | offset failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem deprecatedEntryPointField_rejected_bound (d : Nat) (field : DeprecatedEntryPointField) (wire : List w8)
    (offset : Nat) (reason : DeprecatedEntryPointFieldFailure) (failure : DeprecatedEntryPointFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | selector failure => exact feltFailure_offset _ _ _ _ failure
  | offset failure => exact feltFailure_offset _ _ _ _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointField_rejected_bound
