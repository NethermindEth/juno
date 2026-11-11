import ProofCheck.FeltValueSpec
import ProofCheck.TransactionState

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The projection stores one four-word value, independently of its concrete
    layout. Names identify the application field rather than a generated offset. -/
abbrev TransactionHashValue := GoArray w64 4

def transactionHashZero : TransactionHashValue := zero_val (GoArray w64 4)

def transactionHashNames : List GoString := [go!"TransactionHash"]

inductive TransactionHashField where
  | hash

def transactionHashFields : List TransactionHashField := [.hash]

def transactionHashFieldName : TransactionHashField → GoString
  | .hash => go!"TransactionHash"

theorem transactionHashField_names : transactionHashFields.map transactionHashFieldName = transactionHashNames := rfl

theorem transactionHashFields_length : transactionHashFields.length = transactionHashNames.length := rfl

theorem transactionHashNames_table : transactionHashNames = transactionNames .transactionHashProjection := rfl

theorem transactionHashField_exists (index : Nat) (bound : index < transactionHashNames.length) :
    ∃ field, transactionHashFields[index]? = some field := by
  have zero : index = 0 := by change index < 1 at bound; omega
  subst index
  exact ⟨.hash, rfl⟩

/-- Null/undefined preserves the prior by-value hash. Successful arrays replace
    the four words according to the independent limb-storage contract. -/
inductive TransactionHashFieldValid (d : Nat) : TransactionHashField → TransactionHashValue →
    List w8 → TransactionHashValue → Prop where
  | hash {old wire value} (valid : FeltValueValid d old wire value) :
      TransactionHashFieldValid d .hash old wire value

inductive TransactionHashFieldFailure where
  | felt (reason : FeltFailureReason)

inductive TransactionHashFieldRejected (d : Nat) : TransactionHashField → List w8 → Nat →
    TransactionHashFieldFailure → Prop where
  | hash {wire offset reason} (failure : FeltFailure d wire offset reason) :
      TransactionHashFieldRejected d .hash wire offset (.felt reason)

theorem transactionHashField_classify (d : Nat) (field : TransactionHashField)
    (old : TransactionHashValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ TransactionHashFieldValid d field old item final) ∨
    (∃ offset reason, TransactionHashFieldRejected d field wire offset reason) := by
  cases field with
  | hash =>
    rcases feltValue_classify d old wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .hash valid⟩
    · exact Or.inr ⟨offset, _, .hash failure⟩

theorem transactionHashField_nonempty (d : Nat) (field : TransactionHashField)
    (old final : TransactionHashValue) (wire : List w8)
    (valid : TransactionHashFieldValid d field old wire final) : 0 < wire.length := by
  cases valid with
  | hash valid => exact feltValueValid_nonempty _ _ _ _ valid

theorem transactionHashField_prefix_unique (d : Nat) (field : TransactionHashField)
    (old a b : TransactionHashValue) (left right restA restB : List w8)
    (first : TransactionHashFieldValid d field old left a)
    (second : TransactionHashFieldValid d field old right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | hash valid => cases second with
    | hash valid2 => exact feltValueValid_prefix_unique d old a b left right restA restB valid valid2 same

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_names
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFields_length
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashNames_table
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_exists
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashField_prefix_unique
