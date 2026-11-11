import ProofCheck.TransactionState
import ProofCheck.InvokeKeys
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

abbrev invokeFieldKeys := transactionKeys .invokeTransaction
abbrev invokeFieldNames := transactionNames .invokeTransaction

def invokeKeyIndex : InvokeField → w64
  | .hash => W64 0
  | .callData => W64 1
  | .signature => W64 2
  | .maxFee => W64 3
  | .contract => W64 4
  | .version => W64 5
  | .entry => W64 6
  | .nonce => W64 7
  | .sender => W64 8
  | .resources => W64 9
  | .tip => W64 10
  | .paymaster => W64 11
  | .deployment => W64 12
  | .nonceMode => W64 13
  | .feeMode => W64 14
  | .proofFacts => W64 15

def invokeKeyCode : Option InvokeField → w64
  | none => W64 (-2)
  | some field => invokeKeyIndex field

def invokeKeySeen (seen : w64) : Option InvokeField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< invokeKeyIndex field)

def InvokeKeyFresh (seen : w64) : Option InvokeField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< invokeKeyIndex field) = W64 0

/-- Actual table names agree with the independent InvokeTransaction schema. -/
theorem invokeTable_names : invokeFieldNames = invokeNames := rfl

theorem invokeKey_lookup (field : InvokeField) :
    invokeFieldNames[(sint.Z (invokeKeyIndex field)).toNat]? = some (invokeFieldName field) := by
  cases field <;> rfl

theorem invokeFoldedNames_distinct : (invokeFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .invokeTransaction

theorem invokeFieldNames_ascii : ∀ name ∈ invokeFieldNames, ASCIIBytes name := by
  simp only [invokeFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem invokeKeyMeaning_unknown (key : GoString) (meaning : InvokeKeyMeaning key none) :
    ∀ name ∈ invokeFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [invokeFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .hash, meaning .callData, meaning .signature, meaning .maxFee, meaning .contract, meaning .version, meaning .entry, meaning .nonce, meaning .sender, meaning .resources, meaning .tip, meaning .paymaster, meaning .deployment, meaning .nonceMode, meaning .feeMode, meaning .proofFacts⟩

theorem invokeFieldHeads_ordered : FieldHeadsOrdered invokeFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([99, 101, 102, 103, 104, 105, 106, 107, 109, 109, 110, 111, 111, 114, 116, 117] : List Int).Pairwise (· ≤ ·)
  decide

theorem invokeFieldEncodings_nonempty : ∀ key ∈ invokeFieldKeys, key.Encoded' ≠ [] := by
  simp only [invokeFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem invokeFieldEncodings_apart : FieldEncodingsApart invokeFieldKeys := by
  simp only [FieldEncodingsApart, invokeFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem invokeCachedShortTable : CachedShortTable invokeFieldKeys := by
  simp only [CachedShortTable, invokeFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem invokeInlineFieldTable : InlineFieldTable invokeFieldKeys invokeFieldNames := by
  simp only [InlineFieldTable, invokeFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.invokeInlineFieldTable
