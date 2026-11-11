import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent two-counter application value. Nullish fields preserve the
    corresponding old counter; every unsigned wire width is accepted. -/
structure AvailabilityValue where
  gas : w64
  dataGas : w64

def availabilityZero : AvailabilityValue := ⟨W64 0, W64 0⟩

inductive AvailabilityField where
  | gas | dataGas
  deriving DecidableEq

def availabilityFields : List AvailabilityField := [.gas, .dataGas]
def availabilityFieldName : AvailabilityField → GoString
  | .gas => go!"L1Gas"
  | .dataGas => go!"L1DataGas"
def availabilityNames : List GoString := [go!"L1Gas", go!"L1DataGas"]
def availabilityGet (field : AvailabilityField) (old : AvailabilityValue) : w64 :=
  match field with | .gas => old.gas | .dataGas => old.dataGas
def availabilitySet (field : AvailabilityField) (old : AvailabilityValue) (value : w64) : AvailabilityValue :=
  match field with | .gas => {old with gas := value} | .dataGas => {old with dataGas := value}

theorem availabilitySet_original (field : AvailabilityField) (old : AvailabilityValue) :
    availabilitySet field old (availabilityGet field old) = old := by cases field <;> rfl

theorem availabilityField_names : availabilityFields.map availabilityFieldName = availabilityNames := rfl

inductive AvailabilityFieldValid (d : Nat) : AvailabilityField → AvailabilityValue → List w8 → List w8 → AvailabilityValue → Prop where
  | scalar {field old item rest value} (valid : ScalarValid (availabilityGet field old) item value) :
      AvailabilityFieldValid d field old item rest (availabilitySet field old value)

inductive AvailabilityFieldFailure where
  | scalar (reason : ExpectedFailureReason)

inductive AvailabilityFieldRejected (d : Nat) : AvailabilityField → List w8 → Nat → AvailabilityFieldFailure → Prop where
  | scalar {field wire reason} (failure : ScalarFailure wire reason) :
      AvailabilityFieldRejected d field wire 0 (.scalar reason)

theorem availabilityField_classify (d : Nat) (field : AvailabilityField) (old : AvailabilityValue) (wire : List w8)
    (_depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ AvailabilityFieldValid d field old item rest final) ∨
    (∃ offset reason, AvailabilityFieldRejected d field wire offset reason) := by
  rcases scalar_classify (availabilityGet field old) wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨item, rest, _, split, .scalar valid⟩
  · exact Or.inr ⟨0, _, .scalar failure⟩

theorem availabilityField_nonempty (d : Nat) (field : AvailabilityField) (old final : AvailabilityValue)
    (item rest : List w8) (valid : AvailabilityFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | scalar valid => cases valid <;> simp

theorem availabilityField_prefix_unique (d : Nat) (field : AvailabilityField) (old a b : AvailabilityValue)
    (left right restA restB : List w8)
    (first : AvailabilityFieldValid d field old left restA a) (second : AvailabilityFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | scalar valid => cases second with
    | scalar valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem availabilityField_not_rejected (d : Nat) (field : AvailabilityField) (old final : AvailabilityValue)
    (item rest : List w8) (offset : Nat) (reason : AvailabilityFieldFailure)
    (valid : AvailabilityFieldValid d field old item rest final)
    (failure : AvailabilityFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | scalar valid => cases failure with
    | scalar failure => exact scalarValid_not_failure _ _ _ _ _ valid failure

theorem availabilityField_rejected_unique (d : Nat) (field : AvailabilityField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : AvailabilityFieldFailure)
    (first : AvailabilityFieldRejected d field wire offsetA a) (second : AvailabilityFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | scalar failure => cases second with
    | scalar failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩

theorem availabilityField_rejected_bound (d : Nat) (field : AvailabilityField) (wire : List w8)
    (offset : Nat) (reason : AvailabilityFieldFailure) (failure : AvailabilityFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by cases failure; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilitySet_original
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityField_rejected_bound
