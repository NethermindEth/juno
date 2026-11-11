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

/-- Independent stored Sierra entry point: an unsigned instruction index and
    a nullable four-limb selector. This does not assert that the index names a
    valid instruction or that the selector corresponds to an existing function. -/
structure SierraEntryPointValue where
  index : w64
  selector : Option (GoArray w64 4)

def sierraEntryPointZero : SierraEntryPointValue := ⟨W64 0, none⟩
inductive SierraEntryPointField where
  | index | selector
  deriving DecidableEq

def sierraEntryPointFields : List SierraEntryPointField := [.index, .selector]
def sierraEntryPointFieldName : SierraEntryPointField → GoString
  | .index => go!"Index"
  | .selector => go!"Selector"
def sierraEntryPointNames : List GoString := [go!"Index", go!"Selector"]

theorem sierraEntryPointField_names : sierraEntryPointFields.map sierraEntryPointFieldName = sierraEntryPointNames := rfl

inductive SierraEntryPointFieldValid (d : Nat) : SierraEntryPointField → SierraEntryPointValue → List w8 → List w8 → SierraEntryPointValue → Prop where
  | index {old : SierraEntryPointValue} {item rest value} (valid : ScalarValid old.index item value) :
      SierraEntryPointFieldValid d .index old item rest {old with index := value}
  | selector {old : SierraEntryPointValue} {item rest value} (valid : FeltValid d item value) :
      SierraEntryPointFieldValid d .selector old item rest {old with selector := value}

inductive SierraEntryPointFieldFailure where
  | scalar (reason : ExpectedFailureReason)
  | felt (reason : FeltFailureReason)

inductive SierraEntryPointFieldRejected (d : Nat) : SierraEntryPointField → List w8 → Nat → SierraEntryPointFieldFailure → Prop where
  | index {wire reason} (failure : ScalarFailure wire reason) :
      SierraEntryPointFieldRejected d .index wire 0 (.scalar reason)
  | selector {wire offset reason} (failure : FeltFailure d wire offset reason) :
      SierraEntryPointFieldRejected d .selector wire offset (.felt reason)

theorem sierraEntryPointField_classify (d : Nat) (field : SierraEntryPointField) (old : SierraEntryPointValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SierraEntryPointFieldValid d field old item rest final) ∨
    (∃ offset reason, SierraEntryPointFieldRejected d field wire offset reason) := by
  cases field with
  | index =>
    rcases scalar_classify old.index wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .index valid⟩
    · exact Or.inr ⟨0, _, .index failure⟩
  | selector =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .selector valid⟩
    · exact Or.inr ⟨offset, _, .selector failure⟩

theorem sierraEntryPointField_nonempty (d : Nat) (field : SierraEntryPointField) (old final : SierraEntryPointValue)
    (item rest : List w8) (valid : SierraEntryPointFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | index valid => cases valid <;> simp
  | selector valid => cases valid <;> simp

theorem sierraEntryPointField_prefix_unique (d : Nat) (field : SierraEntryPointField) (old a b : SierraEntryPointValue)
    (left right restA restB : List w8)
    (first : SierraEntryPointFieldValid d field old left restA a) (second : SierraEntryPointFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | index valid => cases second with
    | index valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | selector valid => cases second with
    | selector valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem sierraEntryPointField_not_rejected (d : Nat) (field : SierraEntryPointField) (old final : SierraEntryPointValue)
    (item rest : List w8) (offset : Nat) (reason : SierraEntryPointFieldFailure)
    (valid : SierraEntryPointFieldValid d field old item rest final)
    (failure : SierraEntryPointFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | index valid => cases failure with
    | index failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | selector valid => cases failure with
    | selector failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure

theorem sierraEntryPointField_rejected_unique (d : Nat) (field : SierraEntryPointField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : SierraEntryPointFieldFailure)
    (first : SierraEntryPointFieldRejected d field wire offsetA a) (second : SierraEntryPointFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | index failure => cases second with
    | index failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | selector failure => cases second with
    | selector failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem sierraEntryPointField_rejected_bound (d : Nat) (field : SierraEntryPointField) (wire : List w8)
    (offset : Nat) (reason : SierraEntryPointFieldFailure) (failure : SierraEntryPointFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | index => omega
  | selector failure => exact feltFailure_offset _ _ _ _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointField_rejected_bound
