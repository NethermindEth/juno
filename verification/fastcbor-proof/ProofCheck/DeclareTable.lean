import ProofCheck.TransactionState
import ProofCheck.DeclareKeys
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

abbrev declareFieldKeys := transactionKeys .declareTransaction
abbrev declareFieldNames := transactionNames .declareTransaction

def declareKeyIndex : DeclareField → w64
  | .hash => W64 0
  | .classHash => W64 1
  | .sender => W64 2
  | .maxFee => W64 3
  | .signature => W64 4
  | .nonce => W64 5
  | .version => W64 6
  | .compiledClassHash => W64 7
  | .resources => W64 8
  | .tip => W64 9
  | .paymaster => W64 10
  | .deployment => W64 11
  | .nonceMode => W64 12
  | .feeMode => W64 13

def declareKeyCode : Option DeclareField → w64
  | none => W64 (-2)
  | some field => declareKeyIndex field

def declareKeySeen (seen : w64) : Option DeclareField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< declareKeyIndex field)

def DeclareKeyFresh (seen : w64) : Option DeclareField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< declareKeyIndex field) = W64 0

/-- Actual table names agree with the independent DeclareTransaction schema. -/
theorem declareTable_names : declareFieldNames = declareNames := rfl

theorem declareKey_lookup (field : DeclareField) :
    declareFieldNames[(sint.Z (declareKeyIndex field)).toNat]? = some (declareFieldName field) := by
  cases field <;> rfl

theorem declareFoldedNames_distinct : (declareFieldNames.map (List.map fieldFoldByte)).Nodup :=
  transactionNames_distinct .declareTransaction

theorem declareFieldNames_ascii : ∀ name ∈ declareFieldNames, ASCIIBytes name := by
  simp only [declareFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem declareKeyMeaning_unknown (key : GoString) (meaning : DeclareKeyMeaning key none) :
    ∀ name ∈ declareFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [declareFieldNames, transactionNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .hash, meaning .classHash, meaning .sender, meaning .maxFee, meaning .signature, meaning .nonce, meaning .version, meaning .compiledClassHash, meaning .resources, meaning .tip, meaning .paymaster, meaning .deployment, meaning .nonceMode, meaning .feeMode⟩

theorem declareFieldHeads_ordered : FieldHeadsOrdered declareFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([99, 101, 102, 103, 105, 105, 107, 109, 109, 110, 111, 113, 116, 117] : List Int).Pairwise (· ≤ ·)
  decide

theorem declareFieldEncodings_nonempty : ∀ key ∈ declareFieldKeys, key.Encoded' ≠ [] := by
  simp only [declareFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem declareFieldEncodings_apart : FieldEncodingsApart declareFieldKeys := by
  simp only [FieldEncodingsApart, declareFieldKeys, transactionKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem declareCachedShortTable : CachedShortTable declareFieldKeys := by
  simp only [CachedShortTable, declareFieldKeys, transactionKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem declareInlineFieldTable : InlineFieldTable declareFieldKeys declareFieldNames := by
  simp only [InlineFieldTable, declareFieldKeys, transactionKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.declareTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.declareKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.declareFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.declareCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.declareInlineFieldTable
