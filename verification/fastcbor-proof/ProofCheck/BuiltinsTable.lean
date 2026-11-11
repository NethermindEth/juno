import ProofCheck.ReceiptState
import ProofCheck.BuiltinsKeys
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

abbrev builtinsFieldKeys := receiptKeys .builtinInstanceCounter
abbrev builtinsFieldNames := receiptNames .builtinInstanceCounter

def builtinsKeyIndex : BuiltinsField → w64
  | .pedersen => W64 0
  | .rangeCheck => W64 1
  | .bitwise => W64 2
  | .output => W64 3
  | .ecsda => W64 4
  | .ecOp => W64 5
  | .keccak => W64 6
  | .poseidon => W64 7
  | .segmentArena => W64 8
  | .addMod => W64 9
  | .mulMod => W64 10
  | .rangeCheck96 => W64 11

def builtinsKeyCode : Option BuiltinsField → w64
  | none => W64 (-2)
  | some field => builtinsKeyIndex field

def builtinsKeySeen (seen : w64) : Option BuiltinsField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< builtinsKeyIndex field)

def BuiltinsKeyFresh (seen : w64) : Option BuiltinsField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< builtinsKeyIndex field) = W64 0

/-- Actual table names agree with the independent BuiltinInstanceCounter schema. -/
theorem builtinsTable_names : builtinsFieldNames = builtinsNames := rfl

theorem builtinsKey_lookup (field : BuiltinsField) :
    builtinsFieldNames[(sint.Z (builtinsKeyIndex field)).toNat]? = some (builtinsFieldName field) := by
  cases field <;> rfl

theorem builtinsFoldedNames_distinct : (builtinsFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .builtinInstanceCounter

theorem builtinsFieldNames_ascii : ∀ name ∈ builtinsFieldNames, ASCIIBytes name := by
  simp only [builtinsFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem builtinsKeyMeaning_unknown (key : GoString) (meaning : BuiltinsKeyMeaning key none) :
    ∀ name ∈ builtinsFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [builtinsFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .pedersen, meaning .rangeCheck, meaning .bitwise, meaning .output, meaning .ecsda, meaning .ecOp, meaning .keccak, meaning .poseidon, meaning .segmentArena, meaning .addMod, meaning .mulMod, meaning .rangeCheck96⟩

theorem builtinsFieldHeads_ordered : FieldHeadsOrdered builtinsFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([100, 101, 102, 102, 102, 102, 103, 104, 104, 106, 108, 108] : List Int).Pairwise (· ≤ ·)
  decide

theorem builtinsFieldEncodings_nonempty : ∀ key ∈ builtinsFieldKeys, key.Encoded' ≠ [] := by
  simp only [builtinsFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem builtinsFieldEncodings_apart : FieldEncodingsApart builtinsFieldKeys := by
  simp only [FieldEncodingsApart, builtinsFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem builtinsCachedShortTable : CachedShortTable builtinsFieldKeys := by
  simp only [CachedShortTable, builtinsFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem builtinsInlineFieldTable : InlineFieldTable builtinsFieldKeys builtinsFieldNames := by
  simp only [InlineFieldTable, builtinsFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsInlineFieldTable
