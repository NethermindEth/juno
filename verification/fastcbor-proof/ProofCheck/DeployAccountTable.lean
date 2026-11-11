import ProofCheck.TransactionState
import ProofCheck.DeployAccountKeys
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

abbrev deployAccountFieldKeys := transactionKeys .deployAccountTransaction
abbrev deployAccountFieldNames := transactionNames .deployAccountTransaction

def deployAccountKeyIndex : DeployAccountField → w64
  | .hash => W64 0
  | .salt => W64 1
  | .contract => W64 2
  | .classHash => W64 3
  | .constructorCallData => W64 4
  | .version => W64 5
  | .maxFee => W64 6
  | .signature => W64 7
  | .nonce => W64 8
  | .resources => W64 9
  | .tip => W64 10
  | .paymaster => W64 11
  | .nonceMode => W64 12
  | .feeMode => W64 13

def deployAccountKeyCode : Option DeployAccountField → w64
  | none => W64 (-2)
  | some field => deployAccountKeyIndex field

def deployAccountKeySeen (seen : w64) : Option DeployAccountField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< deployAccountKeyIndex field)

def DeployAccountKeyFresh (seen : w64) : Option DeployAccountField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< deployAccountKeyIndex field) = W64 0

/-- Actual table names agree with the independent DeployAccountTransaction schema. -/
theorem deployAccountTable_names : deployAccountFieldNames = deployAccountNames := rfl

theorem deployAccountKey_lookup (field : DeployAccountField) :
    deployAccountFieldNames[(sint.Z (deployAccountKeyIndex field)).toNat]? = some (deployAccountFieldName field) := by
  cases field <;> rfl

theorem deployAccountFoldedNames_distinct : (deployAccountFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .deployAccountTransaction

theorem deployAccountFieldNames_ascii : ∀ name ∈ deployAccountFieldNames, ASCIIBytes name := by
  simp only [deployAccountFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem deployAccountKeyMeaning_unknown (key : GoString) (meaning : DeployAccountKeyMeaning key none) :
    ∀ name ∈ deployAccountFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [deployAccountFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .hash, meaning .salt, meaning .contract, meaning .classHash, meaning .constructorCallData, meaning .version, meaning .maxFee, meaning .signature, meaning .nonce, meaning .resources, meaning .tip, meaning .paymaster, meaning .nonceMode, meaning .feeMode⟩

theorem deployAccountFieldHeads_ordered : FieldHeadsOrdered deployAccountFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([99, 101, 102, 103, 105, 105, 107, 109, 110, 111, 111, 115, 115, 116] : List Int).Pairwise (· ≤ ·)
  decide

theorem deployAccountFieldEncodings_nonempty : ∀ key ∈ deployAccountFieldKeys, key.Encoded' ≠ [] := by
  simp only [deployAccountFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem deployAccountFieldEncodings_apart : FieldEncodingsApart deployAccountFieldKeys := by
  simp only [FieldEncodingsApart, deployAccountFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem deployAccountCachedShortTable : CachedShortTable deployAccountFieldKeys := by
  simp only [CachedShortTable, deployAccountFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem deployAccountInlineFieldTable : InlineFieldTable deployAccountFieldKeys deployAccountFieldNames := by
  simp only [InlineFieldTable, deployAccountFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.deployAccountInlineFieldTable
