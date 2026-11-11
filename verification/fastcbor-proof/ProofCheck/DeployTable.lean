import ProofCheck.TransactionState
import ProofCheck.DeployKeys
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

abbrev deployFieldKeys := transactionKeys .deployTransaction
abbrev deployFieldNames := transactionNames .deployTransaction

def deployKeyIndex : DeployField → w64
  | .hash => W64 0
  | .salt => W64 1
  | .contract => W64 2
  | .classHash => W64 3
  | .constructorCallData => W64 4
  | .version => W64 5

def deployKeyCode : Option DeployField → w64
  | none => W64 (-2)
  | some field => deployKeyIndex field

def deployKeySeen (seen : w64) : Option DeployField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< deployKeyIndex field)

def DeployKeyFresh (seen : w64) : Option DeployField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< deployKeyIndex field) = W64 0

/-- Actual table names agree with the independent DeployTransaction schema. -/
theorem deployTable_names : deployFieldNames = deployNames := rfl

theorem deployKey_lookup (field : DeployField) :
    deployFieldNames[(sint.Z (deployKeyIndex field)).toNat]? = some (deployFieldName field) := by
  cases field <;> rfl

theorem deployFoldedNames_distinct : (deployFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .deployTransaction

theorem deployFieldNames_ascii : ∀ name ∈ deployFieldNames, ASCIIBytes name := by
  simp only [deployFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem deployKeyMeaning_unknown (key : GoString) (meaning : DeployKeyMeaning key none) :
    ∀ name ∈ deployFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [deployFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .hash, meaning .salt, meaning .contract, meaning .classHash, meaning .constructorCallData, meaning .version⟩

theorem deployFieldHeads_ordered : FieldHeadsOrdered deployFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([103, 105, 111, 111, 115, 115] : List Int).Pairwise (· ≤ ·)
  decide

theorem deployFieldEncodings_nonempty : ∀ key ∈ deployFieldKeys, key.Encoded' ≠ [] := by
  simp only [deployFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem deployFieldEncodings_apart : FieldEncodingsApart deployFieldKeys := by
  simp only [FieldEncodingsApart, deployFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem deployCachedShortTable : CachedShortTable deployFieldKeys := by
  simp only [CachedShortTable, deployFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem deployInlineFieldTable : InlineFieldTable deployFieldKeys deployFieldNames := by
  simp only [InlineFieldTable, deployFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deployTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.deployFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.deployFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.deployKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.deployFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.deployFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deployFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.deployCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.deployInlineFieldTable
