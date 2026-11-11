import ProofCheck.StateDiffPointerSpec
import ProofCheck.WireFailurePrefix
import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Stored StateUpdate values. State-root consistency and state-transition
    validation are outside this decoding relation. -/
structure StateUpdateValue where
  blockHash : Option (GoArray w64 4)
  newRoot : Option (GoArray w64 4)
  oldRoot : Option (GoArray w64 4)
  diff : Option StateDiffValue

def stateUpdateZero : StateUpdateValue := ⟨none, none, none, none⟩

inductive StateUpdateField where
  | blockHash | newRoot | oldRoot | diff
  deriving DecidableEq

def stateUpdateFields : List StateUpdateField := [.blockHash, .newRoot, .oldRoot, .diff]
def stateUpdateFieldName : StateUpdateField → GoString
  | .blockHash => go!"BlockHash"
  | .newRoot => go!"NewRoot"
  | .oldRoot => go!"OldRoot"
  | .diff => go!"StateDiff"
def stateUpdateNames : List GoString := [go!"BlockHash", go!"NewRoot", go!"OldRoot", go!"StateDiff"]

theorem stateUpdateField_names : stateUpdateFields.map stateUpdateFieldName = stateUpdateNames := rfl

inductive StateUpdateFieldValid (d : Nat) : StateUpdateField → StateUpdateValue → List w8 → List w8 → StateUpdateValue → Prop where
  | blockHash {old : StateUpdateValue} {item rest value} (valid : FeltValid d item value) :
      StateUpdateFieldValid d .blockHash old item rest {old with blockHash := value}
  | newRoot {old : StateUpdateValue} {item rest value} (valid : FeltValid d item value) :
      StateUpdateFieldValid d .newRoot old item rest {old with newRoot := value}
  | oldRoot {old : StateUpdateValue} {item rest value} (valid : FeltValid d item value) :
      StateUpdateFieldValid d .oldRoot old item rest {old with oldRoot := value}
  | diff {old : StateUpdateValue} {item rest value} (valid : StateDiffPointerValid d item rest value) :
      StateUpdateFieldValid d .diff old item rest {old with diff := value}

inductive StateUpdateFieldFailure where
  | felt (reason : FeltFailureReason)
  | diff (retained : StateDiffValue) (reason : StateDiffFailureReason)

/-- All four fields preserve their old pointers on failure. A failed StateDiff
    allocation remains private; its partial value and cursor depth are retained. -/
inductive StateUpdateFieldRejected (d : Nat) : StateUpdateField → List w8 →
    Nat → Nat → Nat → StateUpdateFieldFailure → Prop where
  | blockHash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      StateUpdateFieldRejected d .blockHash wire 0 offset 0 (.felt reason)
  | newRoot {wire offset reason} (failure : FeltFailure d wire offset reason) :
      StateUpdateFieldRejected d .newRoot wire 0 offset 0 (.felt reason)
  | oldRoot {wire offset reason} (failure : FeltFailure d wire offset reason) :
      StateUpdateFieldRejected d .oldRoot wire 0 offset 0 (.felt reason)
  | diff {wire retained consumed offset extraDepth reason}
      (failure : StateDiffPointerRejected d wire retained consumed offset extraDepth reason) :
      StateUpdateFieldRejected d .diff wire consumed offset extraDepth (.diff retained reason)

theorem stateUpdateField_classify (d : Nat) (field : StateUpdateField) (old : StateUpdateValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateUpdateFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, StateUpdateFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | blockHash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .blockHash valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .blockHash failure⟩
  | newRoot =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .newRoot valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .newRoot failure⟩
  | oldRoot =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .oldRoot valid⟩
    · exact Or.inr ⟨0, offset, 0, _, .oldRoot failure⟩
  | diff =>
    rcases stateDiffPointer_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .diff valid⟩
    · exact Or.inr ⟨consumed, offset, extraDepth, _, .diff failure⟩

theorem stateUpdateField_nonempty (d : Nat) (field : StateUpdateField) (old final : StateUpdateValue)
    (item rest : List w8) (valid : StateUpdateFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | blockHash valid => cases valid <;> simp
  | newRoot valid => cases valid <;> simp
  | oldRoot valid => cases valid <;> simp
  | diff valid => exact stateDiffPointer_nonempty _ _ _ _ valid

theorem stateUpdateField_prefix_unique (d : Nat) (field : StateUpdateField) (old a b : StateUpdateValue)
    (left right restA restB : List w8)
    (first : StateUpdateFieldValid d field old left restA a) (second : StateUpdateFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | blockHash valid => cases second with
    | blockHash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | newRoot valid => cases second with
    | newRoot valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | oldRoot valid => cases second with
    | oldRoot valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | diff valid => cases second with
    | diff valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := stateDiffPointer_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem stateUpdateField_not_rejected (d : Nat) (field : StateUpdateField) (old final : StateUpdateValue)
    (item rest : List w8) (consumed offset extraDepth : Nat) (reason : StateUpdateFieldFailure)
    (valid : StateUpdateFieldValid d field old item rest final)
    (failure : StateUpdateFieldRejected d field (item ++ rest) consumed offset extraDepth reason) (bound : (item ++ rest).length < 2^64) : False := by
  cases valid with
  | blockHash valid => cases failure with
    | blockHash failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | newRoot valid => cases failure with
    | newRoot failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | oldRoot valid => cases failure with
    | oldRoot failure => exact feltValid_not_failure _ _ _ _ _ _ valid failure
  | diff valid => cases failure with
    | diff failure => exact stateDiffPointer_not_rejected _ _ _ _ _ _ _ _ _ valid failure bound

theorem stateUpdateField_rejected_unique (d : Nat) (field : StateUpdateField) (wire : List w8)
    (consumedA consumedB offsetA offsetB depthA depthB : Nat) (a b : StateUpdateFieldFailure)
    (first : StateUpdateFieldRejected d field wire consumedA offsetA depthA a)
    (second : StateUpdateFieldRejected d field wire consumedB offsetB depthB b) (bound : wire.length < 2^64) :
    consumedA = consumedB ∧ offsetA = offsetB ∧ depthA = depthB ∧ a = b := by
  cases first with
  | blockHash failure => cases second with
    | blockHash failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | newRoot failure => cases second with
    | newRoot failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | oldRoot failure => cases second with
    | oldRoot failure2 =>
      obtain ⟨rfl, rfl⟩ := feltFailure_unique _ _ _ _ _ _ failure failure2
      exact ⟨rfl, rfl, rfl, rfl⟩
  | diff failure => cases second with
    | diff failure2 =>
      obtain ⟨rfl, rfl, rfl, rfl, rfl⟩ := stateDiffPointerRejected_unique _ _ _ _ _ _ _ _ _ _ _ _ failure failure2 bound
      exact ⟨rfl, rfl, rfl, rfl⟩

theorem stateUpdateField_rejected_bounds (d : Nat) (field : StateUpdateField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : StateUpdateFieldFailure)
    (failure : StateUpdateFieldRejected d field wire consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | blockHash failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | newRoot failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | oldRoot failure => exact ⟨Nat.zero_le _, feltFailure_offset _ _ _ _ failure⟩
  | diff failure => exact stateDiffPointerRejected_bounds _ _ _ _ _ _ _ failure

theorem stateUpdateField_rejected_depth (d : Nat) (field : StateUpdateField) (wire : List w8)
    (consumed offset extraDepth : Nat) (reason : StateUpdateFieldFailure) (depth : d ≤ 32)
    (failure : StateUpdateFieldRejected d field wire consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | blockHash failure => omega
  | newRoot failure => omega
  | oldRoot failure => omega
  | diff failure => exact stateDiffPointerRejected_depth _ _ _ _ _ _ _ depth failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_rejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateField_rejected_depth
