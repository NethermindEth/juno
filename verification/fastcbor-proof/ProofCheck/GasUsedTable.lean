import ProofCheck.ReceiptState
import ProofCheck.GasUsedKeys
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

abbrev gasUsedFieldKeys := receiptKeys .gasConsumed
abbrev gasUsedFieldNames := receiptNames .gasConsumed

def gasUsedKeyIndex : GasUsedField → w64
  | .gas => W64 0
  | .dataGas => W64 1
  | .l2Gas => W64 2

def gasUsedKeyCode : Option GasUsedField → w64
  | none => W64 (-2)
  | some field => gasUsedKeyIndex field

def gasUsedKeySeen (seen : w64) : Option GasUsedField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< gasUsedKeyIndex field)

def GasUsedKeyFresh (seen : w64) : Option GasUsedField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< gasUsedKeyIndex field) = W64 0

/-- Actual table names agree with the independent GasConsumed schema. -/
theorem gasUsedTable_names : gasUsedFieldNames = gasUsedNames := rfl

theorem gasUsedKey_lookup (field : GasUsedField) :
    gasUsedFieldNames[(sint.Z (gasUsedKeyIndex field)).toNat]? = some (gasUsedFieldName field) := by
  cases field <;> rfl

theorem gasUsedFoldedNames_distinct : (gasUsedFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .gasConsumed

theorem gasUsedFieldNames_ascii : ∀ name ∈ gasUsedFieldNames, ASCIIBytes name := by
  simp only [gasUsedFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem gasUsedKeyMeaning_unknown (key : GoString) (meaning : GasUsedKeyMeaning key none) :
    ∀ name ∈ gasUsedFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [gasUsedFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .gas, meaning .dataGas, meaning .l2Gas⟩

theorem gasUsedFieldHeads_ordered : FieldHeadsOrdered gasUsedFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([101, 101, 105] : List Int).Pairwise (· ≤ ·)
  decide

theorem gasUsedFieldEncodings_nonempty : ∀ key ∈ gasUsedFieldKeys, key.Encoded' ≠ [] := by
  simp only [gasUsedFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem gasUsedFieldEncodings_apart : FieldEncodingsApart gasUsedFieldKeys := by
  simp only [FieldEncodingsApart, gasUsedFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem gasUsedCachedShortTable : CachedShortTable gasUsedFieldKeys := by
  simp only [CachedShortTable, gasUsedFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem gasUsedInlineFieldTable : InlineFieldTable gasUsedFieldKeys gasUsedFieldNames := by
  simp only [InlineFieldTable, gasUsedFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.gasUsedInlineFieldTable
