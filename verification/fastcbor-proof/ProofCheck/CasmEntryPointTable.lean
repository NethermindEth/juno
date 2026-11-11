import ProofCheck.ClassTable
import ProofCheck.CasmEntryPointKeys
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

abbrev casmEntryPointFieldKeys := classKeys .casmEntryPoint
abbrev casmEntryPointFieldNames := classNames .casmEntryPoint

def casmEntryPointKeyIndex : CasmEntryPointField → w64
  | .offset => W64 0
  | .builtins => W64 1
  | .selector => W64 2

def casmEntryPointKeyCode : Option CasmEntryPointField → w64
  | none => W64 (-2)
  | some field => casmEntryPointKeyIndex field

def casmEntryPointKeySeen (seen : w64) : Option CasmEntryPointField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< casmEntryPointKeyIndex field)

def CasmEntryPointKeyFresh (seen : w64) : Option CasmEntryPointField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< casmEntryPointKeyIndex field) = W64 0

/-- Actual table names agree with the independent CasmEntryPoint schema. -/
theorem casmEntryPointTable_names : casmEntryPointFieldNames = casmEntryPointNames := rfl

theorem casmEntryPointKey_lookup (field : CasmEntryPointField) :
    casmEntryPointFieldNames[(sint.Z (casmEntryPointKeyIndex field)).toNat]? = some (casmEntryPointFieldName field) := by
  cases field <;> rfl

theorem casmEntryPointFoldedNames_distinct : (casmEntryPointFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .casmEntryPoint

theorem casmEntryPointFieldNames_ascii : ∀ name ∈ casmEntryPointFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .casmEntryPoint

/-- Unknown abstract fields exclude every actual table name. -/
theorem casmEntryPointKeyMeaning_unknown (key : GoString) (meaning : CasmEntryPointKeyMeaning key none) :
    ∀ name ∈ casmEntryPointFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [casmEntryPointFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .offset, meaning .builtins, meaning .selector⟩

theorem casmEntryPointFieldHeads_ordered : FieldHeadsOrdered casmEntryPointFieldKeys :=
  classFieldHeads_ordered .casmEntryPoint

theorem casmEntryPointFieldEncodings_nonempty : ∀ key ∈ casmEntryPointFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .casmEntryPoint

theorem casmEntryPointFieldEncodings_apart : FieldEncodingsApart casmEntryPointFieldKeys :=
  classFieldEncodings_apart .casmEntryPoint

theorem casmEntryPointCachedShortTable : CachedShortTable casmEntryPointFieldKeys :=
  classCachedShortTable .casmEntryPoint

theorem casmEntryPointInlineFieldTable : InlineFieldTable casmEntryPointFieldKeys casmEntryPointFieldNames :=
  classInlineFieldTable .casmEntryPoint

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointInlineFieldTable
