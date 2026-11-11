import ProofCheck.TransactionState
import ProofCheck.L1HandlerKeys
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

abbrev l1HandlerFieldKeys := transactionKeys .l1HandlerTransaction
abbrev l1HandlerFieldNames := transactionNames .l1HandlerTransaction

def l1HandlerKeyIndex : L1HandlerField → w64
  | .hash => W64 0
  | .contract => W64 1
  | .entry => W64 2
  | .nonce => W64 3
  | .callData => W64 4
  | .version => W64 5

def l1HandlerKeyCode : Option L1HandlerField → w64
  | none => W64 (-2)
  | some field => l1HandlerKeyIndex field

def l1HandlerKeySeen (seen : w64) : Option L1HandlerField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< l1HandlerKeyIndex field)

def L1HandlerKeyFresh (seen : w64) : Option L1HandlerField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< l1HandlerKeyIndex field) = W64 0

/-- Actual table names agree with the independent L1HandlerTransaction schema. -/
theorem l1HandlerTable_names : l1HandlerFieldNames = l1HandlerNames := rfl

theorem l1HandlerKey_lookup (field : L1HandlerField) :
    l1HandlerFieldNames[(sint.Z (l1HandlerKeyIndex field)).toNat]? = some (l1HandlerFieldName field) := by
  cases field <;> rfl

theorem l1HandlerFoldedNames_distinct : (l1HandlerFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .l1HandlerTransaction

theorem l1HandlerFieldNames_ascii : ∀ name ∈ l1HandlerFieldNames, ASCIIBytes name := by
  simp only [l1HandlerFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem l1HandlerKeyMeaning_unknown (key : GoString) (meaning : L1HandlerKeyMeaning key none) :
    ∀ name ∈ l1HandlerFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [l1HandlerFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .hash, meaning .contract, meaning .entry, meaning .nonce, meaning .callData, meaning .version⟩

theorem l1HandlerFieldHeads_ordered : FieldHeadsOrdered l1HandlerFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([101, 103, 104, 111, 111, 114] : List Int).Pairwise (· ≤ ·)
  decide

theorem l1HandlerFieldEncodings_nonempty : ∀ key ∈ l1HandlerFieldKeys, key.Encoded' ≠ [] := by
  simp only [l1HandlerFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem l1HandlerFieldEncodings_apart : FieldEncodingsApart l1HandlerFieldKeys := by
  simp only [FieldEncodingsApart, l1HandlerFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem l1HandlerCachedShortTable : CachedShortTable l1HandlerFieldKeys := by
  simp only [CachedShortTable, l1HandlerFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem l1HandlerInlineFieldTable : InlineFieldTable l1HandlerFieldKeys l1HandlerFieldNames := by
  simp only [InlineFieldTable, l1HandlerFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.l1HandlerInlineFieldTable
