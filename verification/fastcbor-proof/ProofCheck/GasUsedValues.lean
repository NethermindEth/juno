import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent 3-counter application value. Nullish fields preserve the
    corresponding old counter; every unsigned wire width is accepted. -/
structure GasUsedValue where
  gas : w64
  dataGas : w64
  l2Gas : w64

def gasUsedZero : GasUsedValue := ⟨W64 0, W64 0, W64 0⟩

inductive GasUsedField where
  | gas | dataGas | l2Gas
  deriving DecidableEq

def gasUsedFields : List GasUsedField := [.gas, .dataGas, .l2Gas]
def gasUsedFieldName : GasUsedField → GoString
  | .gas => go!"L1Gas"
  | .dataGas => go!"L1DataGas"
  | .l2Gas => go!"L2Gas"
def gasUsedNames : List GoString := [go!"L1Gas", go!"L1DataGas", go!"L2Gas"]
def gasUsedGet (field : GasUsedField) (old : GasUsedValue) : w64 :=
  match field with
  | .gas => old.gas
  | .dataGas => old.dataGas
  | .l2Gas => old.l2Gas
def gasUsedSet (field : GasUsedField) (old : GasUsedValue) (value : w64) : GasUsedValue :=
  match field with
  | .gas => {old with gas := value}
  | .dataGas => {old with dataGas := value}
  | .l2Gas => {old with l2Gas := value}

theorem gasUsedSet_original (field : GasUsedField) (old : GasUsedValue) :
    gasUsedSet field old (gasUsedGet field old) = old := by cases field <;> rfl

theorem gasUsedField_names : gasUsedFields.map gasUsedFieldName = gasUsedNames := rfl

inductive GasUsedFieldValid (d : Nat) : GasUsedField → GasUsedValue → List w8 → List w8 → GasUsedValue → Prop where
  | scalar {field old item rest value} (valid : ScalarValid (gasUsedGet field old) item value) :
      GasUsedFieldValid d field old item rest (gasUsedSet field old value)

inductive GasUsedFieldFailure where
  | scalar (reason : ExpectedFailureReason)

inductive GasUsedFieldRejected (d : Nat) : GasUsedField → List w8 → Nat → GasUsedFieldFailure → Prop where
  | scalar {field wire reason} (failure : ScalarFailure wire reason) :
      GasUsedFieldRejected d field wire 0 (.scalar reason)

theorem gasUsedField_classify (d : Nat) (field : GasUsedField) (old : GasUsedValue) (wire : List w8)
    (_depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasUsedFieldValid d field old item rest final) ∨
    (∃ offset reason, GasUsedFieldRejected d field wire offset reason) := by
  rcases scalar_classify (gasUsedGet field old) wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨item, rest, _, split, .scalar valid⟩
  · exact Or.inr ⟨0, _, .scalar failure⟩

theorem gasUsedField_nonempty (d : Nat) (field : GasUsedField) (old final : GasUsedValue)
    (item rest : List w8) (valid : GasUsedFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | scalar valid => cases valid <;> simp

theorem gasUsedField_prefix_unique (d : Nat) (field : GasUsedField) (old a b : GasUsedValue)
    (left right restA restB : List w8)
    (first : GasUsedFieldValid d field old left restA a) (second : GasUsedFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | scalar valid => cases second with
    | scalar valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem gasUsedField_not_rejected (d : Nat) (field : GasUsedField) (old final : GasUsedValue)
    (item rest : List w8) (offset : Nat) (reason : GasUsedFieldFailure)
    (valid : GasUsedFieldValid d field old item rest final)
    (failure : GasUsedFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | scalar valid => cases failure with
    | scalar failure => exact scalarValid_not_failure _ _ _ _ _ valid failure

theorem gasUsedField_rejected_unique (d : Nat) (field : GasUsedField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : GasUsedFieldFailure)
    (first : GasUsedFieldRejected d field wire offsetA a) (second : GasUsedFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | scalar failure => cases second with
    | scalar failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩

theorem gasUsedField_rejected_bound (d : Nat) (field : GasUsedField) (wire : List w8)
    (offset : Nat) (reason : GasUsedFieldFailure) (failure : GasUsedFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by cases failure; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedSet_original
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedField_rejected_bound
