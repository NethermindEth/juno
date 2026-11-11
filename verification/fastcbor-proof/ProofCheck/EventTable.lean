import ProofCheck.ReceiptState
import ProofCheck.EventKeys
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

abbrev eventFieldKeys := receiptKeys .event
abbrev eventFieldNames := receiptNames .event

def eventKeyIndex : EventField → w64
  | .sender => W64 0
  | .keys => W64 1
  | .data => W64 2

def eventKeyCode : Option EventField → w64
  | none => W64 (-2)
  | some field => eventKeyIndex field

def eventKeySeen (seen : w64) : Option EventField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< eventKeyIndex field)

def EventKeyFresh (seen : w64) : Option EventField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< eventKeyIndex field) = W64 0

/-- Actual table names agree with the independent Event schema. -/
theorem eventTable_names : eventFieldNames = eventNames := rfl

theorem eventKey_lookup (field : EventField) :
    eventFieldNames[(sint.Z (eventKeyIndex field)).toNat]? = some (eventFieldName field) := by
  cases field <;> rfl

theorem eventFoldedNames_distinct : (eventFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .event

theorem eventFieldNames_ascii : ∀ name ∈ eventFieldNames, ASCIIBytes name := by
  simp only [eventFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem eventKeyMeaning_unknown (key : GoString) (meaning : EventKeyMeaning key none) :
    ∀ name ∈ eventFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [eventFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .sender, meaning .keys, meaning .data⟩

theorem eventFieldHeads_ordered : FieldHeadsOrdered eventFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([100, 100, 100] : List Int).Pairwise (· ≤ ·)
  decide

theorem eventFieldEncodings_nonempty : ∀ key ∈ eventFieldKeys, key.Encoded' ≠ [] := by
  simp only [eventFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem eventFieldEncodings_apart : FieldEncodingsApart eventFieldKeys := by
  simp only [FieldEncodingsApart, eventFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem eventCachedShortTable : CachedShortTable eventFieldKeys := by
  simp only [CachedShortTable, eventFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem eventInlineFieldTable : InlineFieldTable eventFieldKeys eventFieldNames := by
  simp only [InlineFieldTable, eventFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.eventKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.eventFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.eventFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.eventKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.eventFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.eventFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.eventFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.eventCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.eventInlineFieldTable
