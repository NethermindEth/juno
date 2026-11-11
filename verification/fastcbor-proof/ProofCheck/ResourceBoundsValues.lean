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

/-- Independent stored resource bounds. The amount is an unsigned counter;
    the price retains nullable four-limb storage, without a uint128 restriction. -/
structure ResourceBoundsValue where
  amount : w64
  price : Option (GoArray w64 4)

def resourceBoundsZero : ResourceBoundsValue := ⟨W64 0, none⟩
inductive ResourceBoundsField where
  | amount | price
  deriving DecidableEq

def resourceBoundsFields : List ResourceBoundsField := [.amount, .price]
def resourceBoundsFieldName : ResourceBoundsField → GoString
  | .amount => go!"MaxAmount"
  | .price => go!"MaxPricePerUnit"
def resourceBoundsNames : List GoString := [go!"MaxAmount", go!"MaxPricePerUnit"]

theorem resourceBoundsField_names : resourceBoundsFields.map resourceBoundsFieldName = resourceBoundsNames := rfl

inductive ResourceBoundsFieldValid (d : Nat) : ResourceBoundsField → ResourceBoundsValue → List w8 → List w8 → ResourceBoundsValue → Prop where
  | amount {old : ResourceBoundsValue} {item rest value} (valid : ScalarValid old.amount item value) :
      ResourceBoundsFieldValid d .amount old item rest {old with amount := value}
  | price {old : ResourceBoundsValue} {item rest value} (valid : FeltValid d item value) :
      ResourceBoundsFieldValid d .price old item rest {old with price := value}

inductive ResourceBoundsFieldFailure where
  | scalar (reason : ExpectedFailureReason)
  | felt (reason : FeltFailureReason)

inductive ResourceBoundsFieldRejected (d : Nat) : ResourceBoundsField → List w8 → Nat → ResourceBoundsFieldFailure → Prop where
  | amount {wire reason} (failure : ScalarFailure wire reason) :
      ResourceBoundsFieldRejected d .amount wire 0 (.scalar reason)
  | price {wire offset reason} (failure : FeltFailure d wire offset reason) :
      ResourceBoundsFieldRejected d .price wire offset (.felt reason)

theorem resourceBoundsField_classify (d : Nat) (field : ResourceBoundsField) (old : ResourceBoundsValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ResourceBoundsFieldValid d field old item rest final) ∨
    (∃ offset reason, ResourceBoundsFieldRejected d field wire offset reason) := by
  cases field with
  | amount =>
    rcases scalar_classify old.amount wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .amount valid⟩
    · exact Or.inr ⟨0, _, .amount failure⟩
  | price =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .price valid⟩
    · exact Or.inr ⟨offset, _, .price failure⟩

theorem resourceBoundsField_nonempty (d : Nat) (field : ResourceBoundsField) (old final : ResourceBoundsValue)
    (item rest : List w8) (valid : ResourceBoundsFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | amount valid => cases valid <;> simp
  | price valid => cases valid <;> simp

theorem resourceBoundsField_prefix_unique (d : Nat) (field : ResourceBoundsField) (old a b : ResourceBoundsValue)
    (left right restA restB : List w8)
    (first : ResourceBoundsFieldValid d field old left restA a) (second : ResourceBoundsFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | amount valid => cases second with
    | amount valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | price valid => cases second with
    | price valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem resourceBoundsField_not_rejected (d : Nat) (field : ResourceBoundsField) (old final : ResourceBoundsValue)
    (item rest : List w8) (offset : Nat) (reason : ResourceBoundsFieldFailure)
    (valid : ResourceBoundsFieldValid d field old item rest final)
    (failure : ResourceBoundsFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | amount valid => cases failure with
    | amount failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | price valid => cases failure with
    | price failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure

theorem resourceBoundsField_rejected_unique (d : Nat) (field : ResourceBoundsField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : ResourceBoundsFieldFailure)
    (first : ResourceBoundsFieldRejected d field wire offsetA a) (second : ResourceBoundsFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | amount failure => cases second with
    | amount failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩
  | price failure => cases second with
    | price failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl⟩

theorem resourceBoundsField_rejected_bound (d : Nat) (field : ResourceBoundsField) (wire : List w8)
    (offset : Nat) (reason : ResourceBoundsFieldFailure) (failure : ResourceBoundsFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by
  cases failure with
  | amount => omega
  | price failure => exact feltFailure_offset _ _ _ _ failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsField_rejected_bound
