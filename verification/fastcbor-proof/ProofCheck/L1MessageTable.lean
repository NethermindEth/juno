import ProofCheck.ReceiptState
import ProofCheck.L1MessageKeys
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

abbrev l1MessageFieldKeys := receiptKeys .l1ToL2Message
abbrev l1MessageFieldNames := receiptNames .l1ToL2Message

def l1MessageKeyIndex : L1MessageField → w64
  | .sender => W64 0
  | .nonce => W64 1
  | .payload => W64 2
  | .selector => W64 3
  | .recipient => W64 4

def l1MessageKeyCode : Option L1MessageField → w64
  | none => W64 (-2)
  | some field => l1MessageKeyIndex field

def l1MessageKeySeen (seen : w64) : Option L1MessageField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< l1MessageKeyIndex field)

def L1MessageKeyFresh (seen : w64) : Option L1MessageField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< l1MessageKeyIndex field) = W64 0

/-- Actual table names agree with the independent L1ToL2Message schema. -/
theorem l1MessageTable_names : l1MessageFieldNames = l1MessageNames := rfl

theorem l1MessageKey_lookup (field : L1MessageField) :
    l1MessageFieldNames[(sint.Z (l1MessageKeyIndex field)).toNat]? = some (l1MessageFieldName field) := by
  cases field <;> rfl

theorem l1MessageFoldedNames_distinct : (l1MessageFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .l1ToL2Message

theorem l1MessageFieldNames_ascii : ∀ name ∈ l1MessageFieldNames, ASCIIBytes name := by
  simp only [l1MessageFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem l1MessageKeyMeaning_unknown (key : GoString) (meaning : L1MessageKeyMeaning key none) :
    ∀ name ∈ l1MessageFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [l1MessageFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .sender, meaning .nonce, meaning .payload, meaning .selector, meaning .recipient⟩

theorem l1MessageFieldHeads_ordered : FieldHeadsOrdered l1MessageFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([98, 100, 101, 103, 104] : List Int).Pairwise (· ≤ ·)
  decide

theorem l1MessageFieldEncodings_nonempty : ∀ key ∈ l1MessageFieldKeys, key.Encoded' ≠ [] := by
  simp only [l1MessageFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem l1MessageFieldEncodings_apart : FieldEncodingsApart l1MessageFieldKeys := by
  simp only [FieldEncodingsApart, l1MessageFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem l1MessageCachedShortTable : CachedShortTable l1MessageFieldKeys := by
  simp only [CachedShortTable, l1MessageFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem l1MessageInlineFieldTable : InlineFieldTable l1MessageFieldKeys l1MessageFieldNames := by
  simp only [InlineFieldTable, l1MessageFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageInlineFieldTable
