import ProofCheck.ClassTable
import ProofCheck.DeprecatedEntryPointKeys
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

abbrev deprecatedEntryPointFieldKeys := classKeys .deprecatedEntryPoint
abbrev deprecatedEntryPointFieldNames := classNames .deprecatedEntryPoint

def deprecatedEntryPointKeyIndex : DeprecatedEntryPointField → w64
  | .selector => W64 0
  | .offset => W64 1

def deprecatedEntryPointKeyCode : Option DeprecatedEntryPointField → w64
  | none => W64 (-2)
  | some field => deprecatedEntryPointKeyIndex field

def deprecatedEntryPointKeySeen (seen : w64) : Option DeprecatedEntryPointField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< deprecatedEntryPointKeyIndex field)

def DeprecatedEntryPointKeyFresh (seen : w64) : Option DeprecatedEntryPointField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< deprecatedEntryPointKeyIndex field) = W64 0

/-- Actual table names agree with the independent DeprecatedEntryPoint schema. -/
theorem deprecatedEntryPointTable_names : deprecatedEntryPointFieldNames = deprecatedEntryPointNames := rfl

theorem deprecatedEntryPointKey_lookup (field : DeprecatedEntryPointField) :
    deprecatedEntryPointFieldNames[(sint.Z (deprecatedEntryPointKeyIndex field)).toNat]? = some (deprecatedEntryPointFieldName field) := by
  cases field <;> rfl

theorem deprecatedEntryPointFoldedNames_distinct : (deprecatedEntryPointFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .deprecatedEntryPoint

theorem deprecatedEntryPointFieldNames_ascii : ∀ name ∈ deprecatedEntryPointFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .deprecatedEntryPoint

/-- Unknown abstract fields exclude every actual table name. -/
theorem deprecatedEntryPointKeyMeaning_unknown (key : GoString) (meaning : DeprecatedEntryPointKeyMeaning key none) :
    ∀ name ∈ deprecatedEntryPointFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [deprecatedEntryPointFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .selector, meaning .offset⟩

theorem deprecatedEntryPointFieldHeads_ordered : FieldHeadsOrdered deprecatedEntryPointFieldKeys :=
  classFieldHeads_ordered .deprecatedEntryPoint

theorem deprecatedEntryPointFieldEncodings_nonempty : ∀ key ∈ deprecatedEntryPointFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .deprecatedEntryPoint

theorem deprecatedEntryPointFieldEncodings_apart : FieldEncodingsApart deprecatedEntryPointFieldKeys :=
  classFieldEncodings_apart .deprecatedEntryPoint

theorem deprecatedEntryPointCachedShortTable : CachedShortTable deprecatedEntryPointFieldKeys :=
  classCachedShortTable .deprecatedEntryPoint

theorem deprecatedEntryPointInlineFieldTable : InlineFieldTable deprecatedEntryPointFieldKeys deprecatedEntryPointFieldNames :=
  classInlineFieldTable .deprecatedEntryPoint

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointInlineFieldTable
