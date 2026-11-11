import ProofCheck.ClassTable
import ProofCheck.SierraClassKeys
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

abbrev sierraClassFieldKeys := classKeys .sierraClass
abbrev sierraClassFieldNames := classNames .sierraClass

def sierraClassKeyIndex : SierraClassField → w64
  | .abi => W64 0
  | .abiHash => W64 1
  | .entries => W64 2
  | .program => W64 3
  | .programHash => W64 4
  | .version => W64 5
  | .compiled => W64 6

def sierraClassKeyCode : Option SierraClassField → w64
  | none => W64 (-2)
  | some field => sierraClassKeyIndex field

def sierraClassKeySeen (seen : w64) : Option SierraClassField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< sierraClassKeyIndex field)

def SierraClassKeyFresh (seen : w64) : Option SierraClassField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< sierraClassKeyIndex field) = W64 0

/-- Actual table names agree with the independent SierraClass schema. -/
theorem sierraClassTable_names : sierraClassFieldNames = sierraClassNames := rfl

theorem sierraClassKey_lookup (field : SierraClassField) :
    sierraClassFieldNames[(sint.Z (sierraClassKeyIndex field)).toNat]? = some (sierraClassFieldName field) := by
  cases field <;> rfl

theorem sierraClassFoldedNames_distinct : (sierraClassFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .sierraClass

theorem sierraClassFieldNames_ascii : ∀ name ∈ sierraClassFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .sierraClass

/-- Unknown abstract fields exclude every actual table name. -/
theorem sierraClassKeyMeaning_unknown (key : GoString) (meaning : SierraClassKeyMeaning key none) :
    ∀ name ∈ sierraClassFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [sierraClassFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .abi, meaning .abiHash, meaning .entries, meaning .program, meaning .programHash, meaning .version, meaning .compiled⟩

theorem sierraClassFieldHeads_ordered : FieldHeadsOrdered sierraClassFieldKeys :=
  classFieldHeads_ordered .sierraClass

theorem sierraClassFieldEncodings_nonempty : ∀ key ∈ sierraClassFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .sierraClass

theorem sierraClassFieldEncodings_apart : FieldEncodingsApart sierraClassFieldKeys :=
  classFieldEncodings_apart .sierraClass

theorem sierraClassCachedShortTable : CachedShortTable sierraClassFieldKeys :=
  classCachedShortTable .sierraClass

theorem sierraClassInlineFieldTable : InlineFieldTable sierraClassFieldKeys sierraClassFieldNames :=
  classInlineFieldTable .sierraClass

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraClassInlineFieldTable
