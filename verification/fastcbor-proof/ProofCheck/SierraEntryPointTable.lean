import ProofCheck.ClassTable
import ProofCheck.SierraEntryPointKeys
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

abbrev sierraEntryPointFieldKeys := classKeys .sierraEntryPoint
abbrev sierraEntryPointFieldNames := classNames .sierraEntryPoint

def sierraEntryPointKeyIndex : SierraEntryPointField → w64
  | .index => W64 0
  | .selector => W64 1

def sierraEntryPointKeyCode : Option SierraEntryPointField → w64
  | none => W64 (-2)
  | some field => sierraEntryPointKeyIndex field

def sierraEntryPointKeySeen (seen : w64) : Option SierraEntryPointField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< sierraEntryPointKeyIndex field)

def SierraEntryPointKeyFresh (seen : w64) : Option SierraEntryPointField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< sierraEntryPointKeyIndex field) = W64 0

/-- Actual table names agree with the independent SierraEntryPoint schema. -/
theorem sierraEntryPointTable_names : sierraEntryPointFieldNames = sierraEntryPointNames := rfl

theorem sierraEntryPointKey_lookup (field : SierraEntryPointField) :
    sierraEntryPointFieldNames[(sint.Z (sierraEntryPointKeyIndex field)).toNat]? = some (sierraEntryPointFieldName field) := by
  cases field <;> rfl

theorem sierraEntryPointFoldedNames_distinct : (sierraEntryPointFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .sierraEntryPoint

theorem sierraEntryPointFieldNames_ascii : ∀ name ∈ sierraEntryPointFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .sierraEntryPoint

/-- Unknown abstract fields exclude every actual table name. -/
theorem sierraEntryPointKeyMeaning_unknown (key : GoString) (meaning : SierraEntryPointKeyMeaning key none) :
    ∀ name ∈ sierraEntryPointFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [sierraEntryPointFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .index, meaning .selector⟩

theorem sierraEntryPointFieldHeads_ordered : FieldHeadsOrdered sierraEntryPointFieldKeys :=
  classFieldHeads_ordered .sierraEntryPoint

theorem sierraEntryPointFieldEncodings_nonempty : ∀ key ∈ sierraEntryPointFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .sierraEntryPoint

theorem sierraEntryPointFieldEncodings_apart : FieldEncodingsApart sierraEntryPointFieldKeys :=
  classFieldEncodings_apart .sierraEntryPoint

theorem sierraEntryPointCachedShortTable : CachedShortTable sierraEntryPointFieldKeys :=
  classCachedShortTable .sierraEntryPoint

theorem sierraEntryPointInlineFieldTable : InlineFieldTable sierraEntryPointFieldKeys sierraEntryPointFieldNames :=
  classInlineFieldTable .sierraEntryPoint

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntryPointInlineFieldTable
