import ProofCheck.StateTable
import ProofCheck.StateDiffKeys
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

abbrev stateDiffFieldKeys := stateKeys .stateDiff
abbrev stateDiffFieldNames := stateNames .stateDiff

def stateDiffKeyIndex : StateDiffField → w64
  | .storage => W64 0
  | .nonces => W64 1
  | .deployed => W64 2
  | .declaredV0 => W64 3
  | .declaredV1 => W64 4
  | .replaced => W64 5
  | .migrated => W64 6

def stateDiffKeyCode : Option StateDiffField → w64
  | none => W64 (-2)
  | some field => stateDiffKeyIndex field

def stateDiffKeySeen (seen : w64) : Option StateDiffField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< stateDiffKeyIndex field)

def StateDiffKeyFresh (seen : w64) : Option StateDiffField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< stateDiffKeyIndex field) = W64 0

/-- Actual table names agree with the independent StateDiff schema. -/
theorem stateDiffTable_names : stateDiffFieldNames = stateDiffNames := rfl

theorem stateDiffKey_lookup (field : StateDiffField) :
    stateDiffFieldNames[(sint.Z (stateDiffKeyIndex field)).toNat]? = some (stateDiffFieldName field) := by
  cases field <;> rfl

theorem stateDiffFoldedNames_distinct : (stateDiffFieldNames.map (List.map fieldFoldByte)).Nodup :=
  stateNames_distinct .stateDiff

theorem stateDiffFieldNames_ascii : ∀ name ∈ stateDiffFieldNames, ASCIIBytes name :=
  stateFieldNames_ascii .stateDiff

/-- Unknown abstract fields exclude every actual table name. -/
theorem stateDiffKeyMeaning_unknown (key : GoString) (meaning : StateDiffKeyMeaning key none) :
    ∀ name ∈ stateDiffFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [stateDiffFieldNames, stateNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .storage, meaning .nonces, meaning .deployed, meaning .declaredV0, meaning .declaredV1, meaning .replaced, meaning .migrated⟩

theorem stateDiffFieldHeads_ordered : FieldHeadsOrdered stateDiffFieldKeys :=
  stateFieldHeads_ordered .stateDiff

theorem stateDiffFieldEncodings_nonempty : ∀ key ∈ stateDiffFieldKeys, key.Encoded' ≠ [] :=
  stateFieldEncodings_nonempty .stateDiff

theorem stateDiffFieldEncodings_apart : FieldEncodingsApart stateDiffFieldKeys :=
  stateFieldEncodings_apart .stateDiff

theorem stateDiffCachedShortTable : CachedShortTable stateDiffFieldKeys :=
  stateCachedShortTable .stateDiff

theorem stateDiffInlineFieldTable : InlineFieldTable stateDiffFieldKeys stateDiffFieldNames :=
  stateInlineFieldTable .stateDiff

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffInlineFieldTable
