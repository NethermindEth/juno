import ProofCheck.TransactionState
import ProofCheck.ResourceBoundsKeys
import ProofCheck.FieldRead
import ProofCheck.FindWords

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

abbrev resourceBoundsFieldKeys := transactionKeys .resourceBounds
abbrev resourceBoundsFieldNames := transactionNames .resourceBounds

def resourceBoundsKeyIndex : ResourceBoundsField → w64
  | .amount => W64 0
  | .price => W64 1

def resourceBoundsKeyCode : Option ResourceBoundsField → w64
  | none => W64 (-2)
  | some field => resourceBoundsKeyIndex field

def resourceBoundsKeySeen (seen : w64) : Option ResourceBoundsField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< resourceBoundsKeyIndex field)

def ResourceBoundsKeyFresh (seen : w64) : Option ResourceBoundsField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< resourceBoundsKeyIndex field) = W64 0

/-- Actual table names agree with the independent ResourceBounds schema. -/
theorem resourceBoundsTable_names : resourceBoundsFieldNames = resourceBoundsNames := rfl

theorem resourceBoundsKey_lookup (field : ResourceBoundsField) :
    resourceBoundsFieldNames[(sint.Z (resourceBoundsKeyIndex field)).toNat]? = some (resourceBoundsFieldName field) := by
  cases field <;> rfl

theorem resourceBoundsFoldedNames_distinct : (resourceBoundsFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .resourceBounds

theorem resourceBoundsFieldNames_ascii : ∀ name ∈ resourceBoundsFieldNames, ASCIIBytes name := by
  simp only [resourceBoundsFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem resourceBoundsKeyMeaning_unknown (key : GoString) (meaning : ResourceBoundsKeyMeaning key none) :
    ∀ name ∈ resourceBoundsFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [resourceBoundsFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .amount, meaning .price⟩

theorem resourceBoundsFieldHeads_ordered : FieldHeadsOrdered resourceBoundsFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([105, 111] : List Int).Pairwise (· ≤ ·)
  decide

theorem resourceBoundsFieldEncodings_nonempty : ∀ key ∈ resourceBoundsFieldKeys, key.Encoded' ≠ [] := by
  simp only [resourceBoundsFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem resourceBoundsFieldEncodings_apart : FieldEncodingsApart resourceBoundsFieldKeys := by
  simp only [FieldEncodingsApart, resourceBoundsFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem resourceBoundsCachedShortTable : CachedShortTable resourceBoundsFieldKeys := by
  simp only [CachedShortTable, resourceBoundsFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem resourceBoundsInlineFieldTable : InlineFieldTable resourceBoundsFieldKeys resourceBoundsFieldNames := by
  simp only [InlineFieldTable, resourceBoundsFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsInlineFieldTable
