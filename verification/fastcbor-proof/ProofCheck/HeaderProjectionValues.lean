import ProofCheck.ProjectionState
import ProofCheck.Uint64PointerSpec
import ProofCheck.FeltDeterminism

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The five manifested Header projections; receipt projections have separate
    value shapes and will instantiate their own record contracts. -/
inductive HeaderProjectionKind where
  | hash | root | count | timestamp | hashAndRoot

def headerProjectionSchema : HeaderProjectionKind → ProjectionSchema
  | .hash => .headerHash
  | .root => .headerGlobalStateRoot
  | .count => .headerTransactionCount
  | .timestamp => .headerTimestamp
  | .hashAndRoot => .headerHashAndStateRoot

def headerProjectionNames (kind : HeaderProjectionKind) : List GoString :=
  projectionNames (headerProjectionSchema kind)

inductive HeaderProjectionField where
  | hash | root | count | timestamp

def headerProjectionFields : HeaderProjectionKind → List HeaderProjectionField
  | .hash => [.hash]
  | .root => [.root]
  | .count => [.count]
  | .timestamp => [.timestamp]
  | .hashAndRoot => [.hash, .root]

def headerProjectionFieldName : HeaderProjectionField → GoString
  | .hash => go!"Hash"
  | .root => go!"GlobalStateRoot"
  | .count => go!"TransactionCount"
  | .timestamp => go!"Timestamp"

/-- Metadata order agrees with the independent application field identities. -/
theorem headerProjectionField_names (kind : HeaderProjectionKind) :
    (headerProjectionFields kind).map headerProjectionFieldName = headerProjectionNames kind := by
  cases kind <;> rfl

/-- Shared abstract value slots let the field policy state exact preservation
    of every other slot. Concrete ownership will constrain unused slots to zero. -/
structure HeaderProjectionValue where
  hash : Option (GoArray w64 4)
  root : Option (GoArray w64 4)
  count : w64
  timestamp : Option w64

def headerProjectionZero : HeaderProjectionValue := ⟨none, none, W64 0, none⟩

/-- Each metadata index denotes exactly the field selected by its wire name. -/
theorem headerProjectionFields_length (kind : HeaderProjectionKind) :
    (headerProjectionFields kind).length = (headerProjectionNames kind).length := by cases kind <;> rfl

theorem headerProjectionField_exists (kind : HeaderProjectionKind) (index : Nat)
    (bound : index < (headerProjectionNames kind).length) :
    ∃ field, (headerProjectionFields kind)[index]? = some field := by
  have inside : index < (headerProjectionFields kind).length := by rw [headerProjectionFields_length]; exact bound
  exact ⟨(headerProjectionFields kind)[index], List.getElem?_eq_getElem inside⟩

inductive HeaderProjectionFieldValid (d : Nat) : HeaderProjectionField → HeaderProjectionValue →
    List w8 → HeaderProjectionValue → Prop where
  | hash {old : HeaderProjectionValue} {wire value} (valid : FeltValid d wire value) :
      HeaderProjectionFieldValid d .hash old wire {old with hash := value}
  | root {old : HeaderProjectionValue} {wire value} (valid : FeltValid d wire value) :
      HeaderProjectionFieldValid d .root old wire {old with root := value}
  | count {old : HeaderProjectionValue} {wire value} (valid : ScalarValid old.count wire value) :
      HeaderProjectionFieldValid d .count old wire {old with count := value}
  | timestamp {old : HeaderProjectionValue} {wire value} (valid : Uint64PointerValid wire value) :
      HeaderProjectionFieldValid d .timestamp old wire {old with timestamp := value}

inductive HeaderProjectionFieldFailure where
  | felt (reason : FeltFailureReason)
  | scalar (reason : ExpectedFailureReason)

/-- These four selected-field helpers roll back their cursor on failure and do
    not publish a replacement value. The diagnostic may point inside a felt. -/
inductive HeaderProjectionFieldRejected (d : Nat) : HeaderProjectionField → List w8 → Nat →
    HeaderProjectionFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      HeaderProjectionFieldRejected d .hash wire offset (.felt reason)
  | root {wire offset reason} (failure : FeltFailure d wire offset reason) :
      HeaderProjectionFieldRejected d .root wire offset (.felt reason)
  | count {wire reason} (failure : ScalarFailure wire reason) :
      HeaderProjectionFieldRejected d .count wire 0 (.scalar reason)
  | timestamp {wire reason} (failure : ScalarFailure wire reason) :
      HeaderProjectionFieldRejected d .timestamp wire 0 (.scalar reason)

/-- Every selected Header projection field has a complete independent outcome. -/
theorem headerProjectionField_classify (d : Nat) (field : HeaderProjectionField)
    (old : HeaderProjectionValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderProjectionFieldValid d field old item final) ∨
    (∃ offset reason, HeaderProjectionFieldRejected d field wire offset reason) := by
  cases field with
  | hash =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨offset, _, .hash failure⟩
  | root =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .root valid⟩
    · exact Or.inr ⟨offset, _, .root failure⟩
  | count =>
    rcases scalar_classify old.count wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .count valid⟩
    · exact Or.inr ⟨0, _, .count failure⟩
  | timestamp =>
    rcases uint64Pointer_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .timestamp valid⟩
    · exact Or.inr ⟨0, _, .timestamp failure⟩

theorem headerProjectionField_nonempty (d : Nat) (field : HeaderProjectionField)
    (old final : HeaderProjectionValue) (wire : List w8)
    (valid : HeaderProjectionFieldValid d field old wire final) : 0 < wire.length := by
  cases valid with
  | hash valid => cases valid <;> simp
  | root valid => cases valid <;> simp
  | count valid => cases valid <;> simp
  | timestamp valid => exact uint64PointerValid_nonempty _ _ valid

theorem headerProjectionField_prefix_unique (d : Nat) (field : HeaderProjectionField)
    (old a b : HeaderProjectionValue) (left right restA restB : List w8)
    (first : HeaderProjectionFieldValid d field old left a)
    (second : HeaderProjectionFieldValid d field old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid =>
    cases second with
    | hash valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | root valid =>
    cases second with
    | root valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := feltValid_prefix_unique d _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | count valid =>
    cases second with
    | count valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := scalarValid_prefix_unique old.count _ _ left right restA restB valid valid2 same
      exact ⟨rfl, rfl, rfl⟩
  | timestamp valid =>
    cases second with
    | timestamp valid2 =>
      obtain ⟨rfl, rfl, rfl⟩ := uint64PointerValid_prefix_unique left right restA restB _ _ valid valid2 same
      exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionFields_length
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_exists
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_prefix_unique

#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_names
