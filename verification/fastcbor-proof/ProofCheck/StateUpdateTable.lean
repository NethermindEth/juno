import ProofCheck.StateTable
import ProofCheck.StateUpdateKeys
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

abbrev stateUpdateFieldKeys := stateKeys .stateUpdate
abbrev stateUpdateFieldNames := stateNames .stateUpdate

def stateUpdateKeyIndex : StateUpdateField → w64
  | .blockHash => W64 0
  | .newRoot => W64 1
  | .oldRoot => W64 2
  | .diff => W64 3

def stateUpdateKeyCode : Option StateUpdateField → w64
  | none => W64 (-2)
  | some field => stateUpdateKeyIndex field

def stateUpdateKeySeen (seen : w64) : Option StateUpdateField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< stateUpdateKeyIndex field)

def StateUpdateKeyFresh (seen : w64) : Option StateUpdateField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< stateUpdateKeyIndex field) = W64 0

/-- Actual table names agree with the independent StateUpdate schema. -/
theorem stateUpdateTable_names : stateUpdateFieldNames = stateUpdateNames := rfl

theorem stateUpdateKey_lookup (field : StateUpdateField) :
    stateUpdateFieldNames[(sint.Z (stateUpdateKeyIndex field)).toNat]? = some (stateUpdateFieldName field) := by
  cases field <;> rfl

theorem stateUpdateFoldedNames_distinct : (stateUpdateFieldNames.map (List.map fieldFoldByte)).Nodup :=
  stateNames_distinct .stateUpdate

theorem stateUpdateFieldNames_ascii : ∀ name ∈ stateUpdateFieldNames, ASCIIBytes name :=
  stateFieldNames_ascii .stateUpdate

/-- Unknown abstract fields exclude every actual table name. -/
theorem stateUpdateKeyMeaning_unknown (key : GoString) (meaning : StateUpdateKeyMeaning key none) :
    ∀ name ∈ stateUpdateFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [stateUpdateFieldNames, stateNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .blockHash, meaning .newRoot, meaning .oldRoot, meaning .diff⟩

theorem stateUpdateFieldHeads_ordered : FieldHeadsOrdered stateUpdateFieldKeys :=
  stateFieldHeads_ordered .stateUpdate

theorem stateUpdateFieldEncodings_nonempty : ∀ key ∈ stateUpdateFieldKeys, key.Encoded' ≠ [] :=
  stateFieldEncodings_nonempty .stateUpdate

theorem stateUpdateFieldEncodings_apart : FieldEncodingsApart stateUpdateFieldKeys :=
  stateFieldEncodings_apart .stateUpdate

theorem stateUpdateCachedShortTable : CachedShortTable stateUpdateFieldKeys :=
  stateCachedShortTable .stateUpdate

theorem stateUpdateInlineFieldTable : InlineFieldTable stateUpdateFieldKeys stateUpdateFieldNames :=
  stateInlineFieldTable .stateUpdate

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateInlineFieldTable
