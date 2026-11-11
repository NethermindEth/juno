import ProofCheck.WireDeterminism
import ProofCheck.ExpectedFailureUnique
import ProofCheck.WireFailurePrefix

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent 12-counter application value. Nullish fields preserve the
    corresponding old counter; every unsigned wire width is accepted. -/
structure BuiltinsValue where
  pedersen : w64
  rangeCheck : w64
  bitwise : w64
  output : w64
  ecsda : w64
  ecOp : w64
  keccak : w64
  poseidon : w64
  segmentArena : w64
  addMod : w64
  mulMod : w64
  rangeCheck96 : w64

def builtinsZero : BuiltinsValue := ⟨W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0, W64 0⟩

inductive BuiltinsField where
  | pedersen | rangeCheck | bitwise | output | ecsda | ecOp | keccak | poseidon | segmentArena | addMod | mulMod | rangeCheck96
  deriving DecidableEq

def builtinsFields : List BuiltinsField := [.pedersen, .rangeCheck, .bitwise, .output, .ecsda, .ecOp, .keccak, .poseidon, .segmentArena, .addMod, .mulMod, .rangeCheck96]
def builtinsFieldName : BuiltinsField → GoString
  | .pedersen => go!"Pedersen"
  | .rangeCheck => go!"RangeCheck"
  | .bitwise => go!"Bitwise"
  | .output => go!"Output"
  | .ecsda => go!"Ecsda"
  | .ecOp => go!"EcOp"
  | .keccak => go!"Keccak"
  | .poseidon => go!"Poseidon"
  | .segmentArena => go!"SegmentArena"
  | .addMod => go!"AddMod"
  | .mulMod => go!"MulMod"
  | .rangeCheck96 => go!"RangeCheck96"
def builtinsNames : List GoString := [go!"Pedersen", go!"RangeCheck", go!"Bitwise", go!"Output", go!"Ecsda", go!"EcOp", go!"Keccak", go!"Poseidon", go!"SegmentArena", go!"AddMod", go!"MulMod", go!"RangeCheck96"]
def builtinsGet (field : BuiltinsField) (old : BuiltinsValue) : w64 :=
  match field with
  | .pedersen => old.pedersen
  | .rangeCheck => old.rangeCheck
  | .bitwise => old.bitwise
  | .output => old.output
  | .ecsda => old.ecsda
  | .ecOp => old.ecOp
  | .keccak => old.keccak
  | .poseidon => old.poseidon
  | .segmentArena => old.segmentArena
  | .addMod => old.addMod
  | .mulMod => old.mulMod
  | .rangeCheck96 => old.rangeCheck96
def builtinsSet (field : BuiltinsField) (old : BuiltinsValue) (value : w64) : BuiltinsValue :=
  match field with
  | .pedersen => {old with pedersen := value}
  | .rangeCheck => {old with rangeCheck := value}
  | .bitwise => {old with bitwise := value}
  | .output => {old with output := value}
  | .ecsda => {old with ecsda := value}
  | .ecOp => {old with ecOp := value}
  | .keccak => {old with keccak := value}
  | .poseidon => {old with poseidon := value}
  | .segmentArena => {old with segmentArena := value}
  | .addMod => {old with addMod := value}
  | .mulMod => {old with mulMod := value}
  | .rangeCheck96 => {old with rangeCheck96 := value}

theorem builtinsSet_original (field : BuiltinsField) (old : BuiltinsValue) :
    builtinsSet field old (builtinsGet field old) = old := by cases field <;> rfl

theorem builtinsField_names : builtinsFields.map builtinsFieldName = builtinsNames := rfl

inductive BuiltinsFieldValid (d : Nat) : BuiltinsField → BuiltinsValue → List w8 → List w8 → BuiltinsValue → Prop where
  | scalar {field old item rest value} (valid : ScalarValid (builtinsGet field old) item value) :
      BuiltinsFieldValid d field old item rest (builtinsSet field old value)

inductive BuiltinsFieldFailure where
  | scalar (reason : ExpectedFailureReason)

inductive BuiltinsFieldRejected (d : Nat) : BuiltinsField → List w8 → Nat → BuiltinsFieldFailure → Prop where
  | scalar {field wire reason} (failure : ScalarFailure wire reason) :
      BuiltinsFieldRejected d field wire 0 (.scalar reason)

theorem builtinsField_classify (d : Nat) (field : BuiltinsField) (old : BuiltinsValue) (wire : List w8)
    (_depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ BuiltinsFieldValid d field old item rest final) ∨
    (∃ offset reason, BuiltinsFieldRejected d field wire offset reason) := by
  rcases scalar_classify (builtinsGet field old) wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨item, rest, _, split, .scalar valid⟩
  · exact Or.inr ⟨0, _, .scalar failure⟩

theorem builtinsField_nonempty (d : Nat) (field : BuiltinsField) (old final : BuiltinsValue)
    (item rest : List w8) (valid : BuiltinsFieldValid d field old item rest final) : 0 < item.length := by
  cases valid with
  | scalar valid => cases valid <;> simp

theorem builtinsField_prefix_unique (d : Nat) (field : BuiltinsField) (old a b : BuiltinsValue)
    (left right restA restB : List w8)
    (first : BuiltinsFieldValid d field old left restA a) (second : BuiltinsFieldValid d field old right restB b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | scalar valid => cases second with
    | scalar valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique _ _ _ _ _ _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

theorem builtinsField_not_rejected (d : Nat) (field : BuiltinsField) (old final : BuiltinsValue)
    (item rest : List w8) (offset : Nat) (reason : BuiltinsFieldFailure)
    (valid : BuiltinsFieldValid d field old item rest final)
    (failure : BuiltinsFieldRejected d field (item ++ rest) offset reason) : False := by
  cases valid with
  | scalar valid => cases failure with
    | scalar failure => exact scalarValid_not_failure _ _ _ _ _ valid failure

theorem builtinsField_rejected_unique (d : Nat) (field : BuiltinsField) (wire : List w8)
    (offsetA offsetB : Nat) (a b : BuiltinsFieldFailure)
    (first : BuiltinsFieldRejected d field wire offsetA a) (second : BuiltinsFieldRejected d field wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  cases first with
  | scalar failure => cases second with
    | scalar failure2 =>
      have eq := scalarFailure_reason_unique _ _ _ failure failure2
      cases eq
      exact ⟨rfl, rfl⟩

theorem builtinsField_rejected_bound (d : Nat) (field : BuiltinsField) (wire : List w8)
    (offset : Nat) (reason : BuiltinsFieldFailure) (failure : BuiltinsFieldRejected d field wire offset reason) :
    offset ≤ wire.length := by cases failure; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsSet_original
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_rejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsField_rejected_bound
