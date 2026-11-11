import ProofCheck.ClassTable
import ProofCheck.SierraEntriesKeys
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

abbrev sierraEntriesFieldKeys := classKeys .sierraEntryPointsByType
abbrev sierraEntriesFieldNames := classNames .sierraEntryPointsByType

def sierraEntriesKeyIndex : SierraEntriesField → w64
  | .constructors => W64 0
  | .external => W64 1
  | .l1Handler => W64 2

def sierraEntriesKeyCode : Option SierraEntriesField → w64
  | none => W64 (-2)
  | some field => sierraEntriesKeyIndex field

def sierraEntriesKeySeen (seen : w64) : Option SierraEntriesField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< sierraEntriesKeyIndex field)

def SierraEntriesKeyFresh (seen : w64) : Option SierraEntriesField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< sierraEntriesKeyIndex field) = W64 0

/-- Actual table names agree with the independent SierraEntries schema. -/
theorem sierraEntriesTable_names : sierraEntriesFieldNames = sierraEntriesNames := rfl

theorem sierraEntriesKey_lookup (field : SierraEntriesField) :
    sierraEntriesFieldNames[(sint.Z (sierraEntriesKeyIndex field)).toNat]? = some (sierraEntriesFieldName field) := by
  cases field <;> rfl

theorem sierraEntriesFoldedNames_distinct : (sierraEntriesFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .sierraEntryPointsByType

theorem sierraEntriesFieldNames_ascii : ∀ name ∈ sierraEntriesFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .sierraEntryPointsByType

/-- Unknown abstract fields exclude every actual table name. -/
theorem sierraEntriesKeyMeaning_unknown (key : GoString) (meaning : SierraEntriesKeyMeaning key none) :
    ∀ name ∈ sierraEntriesFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [sierraEntriesFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .constructors, meaning .external, meaning .l1Handler⟩

theorem sierraEntriesFieldHeads_ordered : FieldHeadsOrdered sierraEntriesFieldKeys :=
  classFieldHeads_ordered .sierraEntryPointsByType

theorem sierraEntriesFieldEncodings_nonempty : ∀ key ∈ sierraEntriesFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .sierraEntryPointsByType

theorem sierraEntriesFieldEncodings_apart : FieldEncodingsApart sierraEntriesFieldKeys :=
  classFieldEncodings_apart .sierraEntryPointsByType

theorem sierraEntriesCachedShortTable : CachedShortTable sierraEntriesFieldKeys :=
  classCachedShortTable .sierraEntryPointsByType

theorem sierraEntriesInlineFieldTable : InlineFieldTable sierraEntriesFieldKeys sierraEntriesFieldNames :=
  classInlineFieldTable .sierraEntryPointsByType

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesInlineFieldTable
