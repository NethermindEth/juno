import ProofCheck.BuiltinsRecordUnique
import ProofCheck.AvailabilitySlotSpec
import ProofCheck.GasUsedSlotSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Logical state of the production shared block. Presence is separate from the
    supplied storage: clearing a pointer does not erase that storage. This covers
    every state reachable from the zero block, including partial failures. -/
structure ExecutionValue where
  builtins : BuiltinsValue
  holes : w64
  steps : w64
  availabilityPresent : Bool
  gasPresent : Bool
  availabilityStorage : AvailabilityValue
  gasStorage : GasUsedValue

def executionZero : ExecutionValue :=
  ⟨builtinsZero, W64 0, W64 0, false, false, availabilityZero, gasUsedZero⟩

def executionSetAvailability (old : ExecutionValue) (value : Option AvailabilityValue) : ExecutionValue :=
  {old with availabilityPresent := value.isSome, availabilityStorage := value.getD old.availabilityStorage}
def executionSetGas (old : ExecutionValue) (value : Option GasUsedValue) : ExecutionValue :=
  {old with gasPresent := value.isSome, gasStorage := value.getD old.gasStorage}

inductive ExecutionField where
  | builtins | holes | steps | availability | gas
  deriving DecidableEq

def executionFields : List ExecutionField := [.builtins, .holes, .steps, .availability, .gas]
def executionFieldName : ExecutionField → GoString
  | .builtins => go!"BuiltinInstanceCounter"
  | .holes => go!"MemoryHoles"
  | .steps => go!"Steps"
  | .availability => go!"DataAvailability"
  | .gas => go!"TotalGasConsumed"
def executionNames : List GoString :=
  [go!"BuiltinInstanceCounter", go!"MemoryHoles", go!"Steps", go!"DataAvailability", go!"TotalGasConsumed"]

theorem executionField_names : executionFields.map executionFieldName = executionNames := rfl

inductive ExecutionFieldValid (d : Nat) : ExecutionField → ExecutionValue → List w8 → List w8 → ExecutionValue → Prop where
  | builtins {old : ExecutionValue} {item rest value}
      (valid : BuiltinsRecordValid d old.builtins item rest value) :
      ExecutionFieldValid d .builtins old item rest {old with builtins := value}
  | holes {old : ExecutionValue} {item rest value} (valid : ScalarValid old.holes item value) :
      ExecutionFieldValid d .holes old item rest {old with holes := value}
  | steps {old : ExecutionValue} {item rest value} (valid : ScalarValid old.steps item value) :
      ExecutionFieldValid d .steps old item rest {old with steps := value}
  | availability {old : ExecutionValue} {item rest value}
      (valid : AvailabilitySlotValid d old.availabilityStorage item rest value) :
      ExecutionFieldValid d .availability old item rest (executionSetAvailability old value)
  | gas {old : ExecutionValue} {item rest value}
      (valid : GasUsedSlotValid d old.gasStorage item rest value) :
      ExecutionFieldValid d .gas old item rest (executionSetGas old value)

inductive ExecutionFieldFailure where
  | builtins (reason : BuiltinsFailureReason)
  | scalar (reason : ExpectedFailureReason)
  | availability (reason : AvailabilityFailureReason)
  | gas (reason : GasUsedFailureReason)

/-- Nested record errors retain partial state and consumption. A failed slot
    preserves the previous presence bit even when its storage was updated. -/
inductive ExecutionFieldRejected (d : Nat) : ExecutionField → ExecutionValue → List w8 → ExecutionValue →
    Nat → Nat → Nat → ExecutionFieldFailure → Prop where
  | builtins {old : ExecutionValue} {wire value consumed offset entered reason}
      (failure : BuiltinsRecordRejected d old.builtins wire value consumed offset entered reason) :
      ExecutionFieldRejected d .builtins old wire {old with builtins := value}
        consumed offset (if entered then 1 else 0) (.builtins reason)
  | holes {old : ExecutionValue} {wire reason} (failure : ScalarFailure wire reason) :
      ExecutionFieldRejected d .holes old wire old 0 0 0 (.scalar reason)
  | steps {old : ExecutionValue} {wire reason} (failure : ScalarFailure wire reason) :
      ExecutionFieldRejected d .steps old wire old 0 0 0 (.scalar reason)
  | availability {old : ExecutionValue} {wire value consumed offset entered reason}
      (failure : AvailabilitySlotRejected d old.availabilityStorage wire value consumed offset entered reason) :
      ExecutionFieldRejected d .availability old wire {old with availabilityStorage := value}
        consumed offset (if entered then 1 else 0) (.availability reason)
  | gas {old : ExecutionValue} {wire value consumed offset entered reason}
      (failure : GasUsedSlotRejected d old.gasStorage wire value consumed offset entered reason) :
      ExecutionFieldRejected d .gas old wire {old with gasStorage := value}
        consumed offset (if entered then 1 else 0) (.gas reason)

theorem executionField_classify (d : Nat) (field : ExecutionField) (old : ExecutionValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ ExecutionFieldValid d field old item rest final) ∨
    (∃ final consumed offset extraDepth reason, ExecutionFieldRejected d field old wire final consumed offset extraDepth reason) := by
  cases field with
  | builtins =>
    rcases builtinsRecord_classify d old.builtins wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨value, consumed, offset, entered, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .builtins valid⟩
    · exact Or.inr ⟨_, consumed, offset, _, _, .builtins failure⟩
  | availability =>
    rcases availabilitySlot_classify d old.availabilityStorage wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨value, consumed, offset, entered, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .availability valid⟩
    · exact Or.inr ⟨_, consumed, offset, _, _, .availability failure⟩
  | gas =>
    rcases gasUsedSlot_classify d old.gasStorage wire depth with
      ⟨item, rest, value, split, valid⟩ | ⟨value, consumed, offset, entered, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .gas valid⟩
    · exact Or.inr ⟨_, consumed, offset, _, _, .gas failure⟩
  | holes =>
    rcases scalar_classify old.holes wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .holes valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .holes failure⟩
  | steps =>
    rcases scalar_classify old.steps wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .steps valid⟩
    · exact Or.inr ⟨_, 0, 0, 0, _, .steps failure⟩

theorem executionField_nonempty (d : Nat) (field : ExecutionField) (old final : ExecutionValue)
    (item rest : List w8) (valid : ExecutionFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | builtins valid => exact builtinsRecord_nonempty _ _ _ _ _ valid
  | availability valid => exact availabilitySlot_nonempty _ _ _ _ _ valid
  | gas valid => exact gasUsedSlot_nonempty _ _ _ _ _ valid
  | holes valid => cases valid <;> simp
  | steps valid => cases valid <;> simp

theorem executionField_prefix_unique (d : Nat) (field : ExecutionField) (old a b : ExecutionValue)
    (left right restA restB : List w8)
    (first : ExecutionFieldValid d field old left restA a) (second : ExecutionFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | builtins valid => cases second with
    | builtins valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := builtinsRecord_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | availability valid => cases second with
    | availability valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := availabilitySlot_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | gas valid => cases second with
    | gas valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := gasUsedSlot_prefix_unique _ _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | holes valid => cases second with
    | holes valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | steps valid => cases second with
    | steps valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem executionField_not_rejected (d : Nat) (field : ExecutionField) (old final retained : ExecutionValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : ExecutionFieldFailure)
    (valid : ExecutionFieldValid d field old item rest final)
    (failure : ExecutionFieldRejected d field old (item ++ rest) retained consumed offset extraDepth reason) : False := by
  cases valid with
  | builtins valid => cases failure with
    | builtins failure => exact builtinsRecord_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure
  | availability valid => cases failure with
    | availability failure => exact availabilitySlot_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure
  | gas valid => cases failure with
    | gas failure => exact gasUsedSlot_not_rejected _ _ _ _ _ _ _ _ _ _ valid failure
  | holes valid => cases failure with
    | holes failure => exact scalarValid_not_failure _ _ _ _ _ valid failure
  | steps valid => cases failure with
    | steps failure => exact scalarValid_not_failure _ _ _ _ _ valid failure

theorem executionField_rejected_unique (d : Nat) (field : ExecutionField) (old a b : ExecutionValue) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (reasonA reasonB : ExecutionFieldFailure)
    (first : ExecutionFieldRejected d field old wire a consumedA offsetA depthA reasonA)
    (second : ExecutionFieldRejected d field old wire b consumedB offsetB depthB reasonB) :
    a = b ∧ consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ reasonA = reasonB := by
  cases first with
  | builtins failure => cases second with
    | builtins failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := builtinsRecordRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | availability failure => cases second with
    | availability failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := availabilitySlotRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | gas failure => cases second with
    | gas failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := gasUsedSlotRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | holes failure => cases second with
    | holes failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩
  | steps failure => cases second with
    | steps failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl, rfl, rfl, rfl⟩

theorem executionField_rejected_bounds (d : Nat) (field : ExecutionField) (old retained : ExecutionValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ExecutionFieldFailure)
    (failure : ExecutionFieldRejected d field old wire retained consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | builtins failure => exact builtinsRecordRejected_bounds _ _ _ _ _ _ _ _ failure
  | availability failure => exact availabilitySlotRejected_bounds _ _ _ _ _ _ _ _ failure
  | gas failure => exact gasUsedSlotRejected_bounds _ _ _ _ _ _ _ _ failure
  | holes => omega
  | steps => omega

theorem executionField_rejected_depth (d : Nat) (field : ExecutionField) (old retained : ExecutionValue) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : ExecutionFieldFailure) (depth : d ≤ 32)
    (failure : ExecutionFieldRejected d field old wire retained consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | builtins failure => cases failure <;> simp_all <;> omega
  | availability failure => cases failure with
    | record hn hu failure => cases failure <;> simp_all <;> omega
  | gas failure => cases failure with
    | record hn hu failure => cases failure <;> simp_all <;> omega
  | holes => omega
  | steps => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.executionField_rejected_depth
