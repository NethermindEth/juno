import ProofCheck.ReceiptState
import ProofCheck.L2MessageKeys
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

abbrev l2MessageFieldKeys := receiptKeys .l2ToL1Message
abbrev l2MessageFieldNames := receiptNames .l2ToL1Message

def l2MessageKeyIndex : L2MessageField → w64
  | .sender => W64 0
  | .payload => W64 1
  | .recipient => W64 2

def l2MessageKeyCode : Option L2MessageField → w64
  | none => W64 (-2)
  | some field => l2MessageKeyIndex field

def l2MessageKeySeen (seen : w64) : Option L2MessageField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< l2MessageKeyIndex field)

def L2MessageKeyFresh (seen : w64) : Option L2MessageField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< l2MessageKeyIndex field) = W64 0

/-- Actual table names agree with the independent L2ToL1Message schema. -/
theorem l2MessageTable_names : l2MessageFieldNames = l2MessageNames := rfl

theorem l2MessageKey_lookup (field : L2MessageField) :
    l2MessageFieldNames[(sint.Z (l2MessageKeyIndex field)).toNat]? = some (l2MessageFieldName field) := by
  cases field <;> rfl

theorem l2MessageFoldedNames_distinct : (l2MessageFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .l2ToL1Message

theorem l2MessageFieldNames_ascii : ∀ name ∈ l2MessageFieldNames, ASCIIBytes name := by
  simp only [l2MessageFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem l2MessageKeyMeaning_unknown (key : GoString) (meaning : L2MessageKeyMeaning key none) :
    ∀ name ∈ l2MessageFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [l2MessageFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .sender, meaning .payload, meaning .recipient⟩

theorem l2MessageFieldHeads_ordered : FieldHeadsOrdered l2MessageFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([98, 100, 103] : List Int).Pairwise (· ≤ ·)
  decide

theorem l2MessageFieldEncodings_nonempty : ∀ key ∈ l2MessageFieldKeys, key.Encoded' ≠ [] := by
  simp only [l2MessageFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem l2MessageFieldEncodings_apart : FieldEncodingsApart l2MessageFieldKeys := by
  simp only [FieldEncodingsApart, l2MessageFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem l2MessageCachedShortTable : CachedShortTable l2MessageFieldKeys := by
  simp only [CachedShortTable, l2MessageFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem l2MessageInlineFieldTable : InlineFieldTable l2MessageFieldKeys l2MessageFieldNames := by
  simp only [InlineFieldTable, l2MessageFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageInlineFieldTable
