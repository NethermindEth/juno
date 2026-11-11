import ProofCheck.ReceiptState
import ProofCheck.ReceiptKeys
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

abbrev receiptFieldKeys := receiptKeys .transactionReceipt
abbrev receiptFieldNames := receiptNames .transactionReceipt

def receiptKeyIndex : ReceiptField → w64
  | .fee => W64 0
  | .unit => W64 1
  | .events => W64 2
  | .resources => W64 3
  | .message => W64 4
  | .messages => W64 5
  | .hash => W64 6
  | .reverted => W64 7
  | .reason => W64 8

def receiptKeyCode : Option ReceiptField → w64
  | none => W64 (-2)
  | some field => receiptKeyIndex field

def receiptKeySeen (seen : w64) : Option ReceiptField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< receiptKeyIndex field)

def ReceiptKeyFresh (seen : w64) : Option ReceiptField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< receiptKeyIndex field) = W64 0

/-- Actual table names agree with the independent TransactionReceipt schema. -/
theorem receiptTable_names : receiptFieldNames = fullReceiptNames := rfl

theorem receiptKey_lookup (field : ReceiptField) :
    receiptFieldNames[(sint.Z (receiptKeyIndex field)).toNat]? = some (receiptFieldName field) := by
  cases field <;> rfl

theorem receiptFoldedNames_distinct : (receiptFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .transactionReceipt

theorem receiptFieldNames_ascii : ∀ name ∈ receiptFieldNames, ASCIIBytes name := by
  simp only [receiptFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem receiptKeyMeaning_unknown (key : GoString) (meaning : ReceiptKeyMeaning key none) :
    ∀ name ∈ receiptFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [receiptFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .fee, meaning .unit, meaning .events, meaning .resources, meaning .message, meaning .messages, meaning .hash, meaning .reverted, meaning .reason⟩

theorem receiptFieldHeads_ordered : FieldHeadsOrdered receiptFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([99, 102, 103, 104, 108, 109, 109, 111, 114] : List Int).Pairwise (· ≤ ·)
  decide

theorem receiptFieldEncodings_nonempty : ∀ key ∈ receiptFieldKeys, key.Encoded' ≠ [] := by
  simp only [receiptFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem receiptFieldEncodings_apart : FieldEncodingsApart receiptFieldKeys := by
  simp only [FieldEncodingsApart, receiptFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem receiptCachedShortTable : CachedShortTable receiptFieldKeys := by
  simp only [CachedShortTable, receiptFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem receiptInlineFieldTable : InlineFieldTable receiptFieldKeys receiptFieldNames := by
  simp only [InlineFieldTable, receiptFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptInlineFieldTable
