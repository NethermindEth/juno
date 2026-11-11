import ProofCheck.ClassTable
import ProofCheck.CasmClassKeys
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

abbrev casmClassFieldKeys := classKeys .casmClass
abbrev casmClassFieldNames := classNames .casmClass

def casmClassKeyIndex : CasmClassField → w64
  | .bytecode => W64 0
  | .pythonicHints => W64 1
  | .compilerVersion => W64 2
  | .hints => W64 3
  | .prime => W64 4
  | .external => W64 5
  | .l1Handler => W64 6
  | .constructor => W64 7
  | .segments => W64 8

def casmClassKeyCode : Option CasmClassField → w64
  | none => W64 (-2)
  | some field => casmClassKeyIndex field

def casmClassKeySeen (seen : w64) : Option CasmClassField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< casmClassKeyIndex field)

def CasmClassKeyFresh (seen : w64) : Option CasmClassField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< casmClassKeyIndex field) = W64 0

/-- Actual table names agree with the independent CasmClass schema. -/
theorem casmClassTable_names : casmClassFieldNames = casmClassNames := rfl

theorem casmClassKey_lookup (field : CasmClassField) :
    casmClassFieldNames[(sint.Z (casmClassKeyIndex field)).toNat]? = some (casmClassFieldName field) := by
  cases field <;> rfl

theorem casmClassFoldedNames_distinct : (casmClassFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .casmClass

theorem casmClassFieldNames_ascii : ∀ name ∈ casmClassFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .casmClass

/-- Unknown abstract fields exclude every actual table name. -/
theorem casmClassKeyMeaning_unknown (key : GoString) (meaning : CasmClassKeyMeaning key none) :
    ∀ name ∈ casmClassFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [casmClassFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .bytecode, meaning .pythonicHints, meaning .compilerVersion, meaning .hints, meaning .prime, meaning .external, meaning .l1Handler, meaning .constructor, meaning .segments⟩

theorem casmClassFieldHeads_ordered : FieldHeadsOrdered casmClassFieldKeys :=
  classFieldHeads_ordered .casmClass

theorem casmClassFieldEncodings_nonempty : ∀ key ∈ casmClassFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .casmClass

theorem casmClassFieldEncodings_apart : FieldEncodingsApart casmClassFieldKeys :=
  classFieldEncodings_apart .casmClass

theorem casmClassCachedShortTable : CachedShortTable casmClassFieldKeys :=
  classCachedShortTable .casmClass

theorem casmClassInlineFieldTable : InlineFieldTable casmClassFieldKeys casmClassFieldNames :=
  classInlineFieldTable .casmClass

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.casmClassInlineFieldTable
