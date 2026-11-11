import ProofCheck.ReceiptState
import ProofCheck.ExecutionKeys
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

abbrev executionFieldKeys := receiptKeys .executionResources
abbrev executionFieldNames := receiptNames .executionResources

def executionKeyIndex : ExecutionField → w64
  | .builtins => W64 0
  | .holes => W64 1
  | .steps => W64 2
  | .availability => W64 3
  | .gas => W64 4

def executionKeyCode : Option ExecutionField → w64
  | none => W64 (-2)
  | some field => executionKeyIndex field

def executionKeySeen (seen : w64) : Option ExecutionField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< executionKeyIndex field)

def ExecutionKeyFresh (seen : w64) : Option ExecutionField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< executionKeyIndex field) = W64 0

/-- Actual table names agree with the independent ExecutionResources schema. -/
theorem executionTable_names : executionFieldNames = executionNames := rfl

theorem executionKey_lookup (field : ExecutionField) :
    executionFieldNames[(sint.Z (executionKeyIndex field)).toNat]? = some (executionFieldName field) := by
  cases field <;> rfl

theorem executionFoldedNames_distinct : (executionFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .executionResources

theorem executionFieldNames_ascii : ∀ name ∈ executionFieldNames, ASCIIBytes name := by
  simp only [executionFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem executionKeyMeaning_unknown (key : GoString) (meaning : ExecutionKeyMeaning key none) :
    ∀ name ∈ executionFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [executionFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .builtins, meaning .holes, meaning .steps, meaning .availability, meaning .gas⟩

theorem executionFieldHeads_ordered : FieldHeadsOrdered executionFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([101, 107, 112, 112, 118] : List Int).Pairwise (· ≤ ·)
  decide

theorem executionFieldEncodings_nonempty : ∀ key ∈ executionFieldKeys, key.Encoded' ≠ [] := by
  simp only [executionFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem executionFieldEncodings_apart : FieldEncodingsApart executionFieldKeys := by
  simp only [FieldEncodingsApart, executionFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem executionCachedShortTable : CachedShortTable executionFieldKeys := by
  simp only [CachedShortTable, executionFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem executionInlineFieldTable : InlineFieldTable executionFieldKeys executionFieldNames := by
  simp only [InlineFieldTable, executionFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.executionTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.executionKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.executionKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.executionFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.executionCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.executionInlineFieldTable
