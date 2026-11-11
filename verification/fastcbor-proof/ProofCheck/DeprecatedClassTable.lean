import ProofCheck.ClassTable
import ProofCheck.DeprecatedClassKeys
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

abbrev deprecatedClassFieldKeys := classKeys .deprecatedCairoClass
abbrev deprecatedClassFieldNames := classNames .deprecatedCairoClass

def deprecatedClassKeyIndex : DeprecatedClassField → w64
  | .abi => W64 0
  | .externals => W64 1
  | .l1Handlers => W64 2
  | .constructors => W64 3
  | .program => W64 4

def deprecatedClassKeyCode : Option DeprecatedClassField → w64
  | none => W64 (-2)
  | some field => deprecatedClassKeyIndex field

def deprecatedClassKeySeen (seen : w64) : Option DeprecatedClassField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< deprecatedClassKeyIndex field)

def DeprecatedClassKeyFresh (seen : w64) : Option DeprecatedClassField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< deprecatedClassKeyIndex field) = W64 0

/-- Actual table names agree with the independent DeprecatedClass schema. -/
theorem deprecatedClassTable_names : deprecatedClassFieldNames = deprecatedClassNames := rfl

theorem deprecatedClassKey_lookup (field : DeprecatedClassField) :
    deprecatedClassFieldNames[(sint.Z (deprecatedClassKeyIndex field)).toNat]? = some (deprecatedClassFieldName field) := by
  cases field <;> rfl

theorem deprecatedClassFoldedNames_distinct : (deprecatedClassFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .deprecatedCairoClass

theorem deprecatedClassFieldNames_ascii : ∀ name ∈ deprecatedClassFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .deprecatedCairoClass

/-- Unknown abstract fields exclude every actual table name. -/
theorem deprecatedClassKeyMeaning_unknown (key : GoString) (meaning : DeprecatedClassKeyMeaning key none) :
    ∀ name ∈ deprecatedClassFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [deprecatedClassFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .abi, meaning .externals, meaning .l1Handlers, meaning .constructors, meaning .program⟩

theorem deprecatedClassFieldHeads_ordered : FieldHeadsOrdered deprecatedClassFieldKeys :=
  classFieldHeads_ordered .deprecatedCairoClass

theorem deprecatedClassFieldEncodings_nonempty : ∀ key ∈ deprecatedClassFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .deprecatedCairoClass

theorem deprecatedClassFieldEncodings_apart : FieldEncodingsApart deprecatedClassFieldKeys :=
  classFieldEncodings_apart .deprecatedCairoClass

theorem deprecatedClassCachedShortTable : CachedShortTable deprecatedClassFieldKeys :=
  classCachedShortTable .deprecatedCairoClass

theorem deprecatedClassInlineFieldTable : InlineFieldTable deprecatedClassFieldKeys deprecatedClassFieldNames :=
  classInlineFieldTable .deprecatedCairoClass

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedClassInlineFieldTable
