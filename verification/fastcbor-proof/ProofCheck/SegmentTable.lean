import ProofCheck.ClassTable
import ProofCheck.SegmentKeys
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

abbrev segmentFieldKeys := classKeys .segmentLengths
abbrev segmentFieldNames := classNames .segmentLengths

def segmentKeyIndex : SegmentField → w64
  | .children => W64 0
  | .length => W64 1

def segmentKeyCode : Option SegmentField → w64
  | none => W64 (-2)
  | some field => segmentKeyIndex field

def segmentKeySeen (seen : w64) : Option SegmentField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< segmentKeyIndex field)

def SegmentKeyFresh (seen : w64) : Option SegmentField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< segmentKeyIndex field) = W64 0

/-- Actual table names agree with the independent Segment schema. -/
theorem segmentTable_names : segmentFieldNames = segmentNames := rfl

theorem segmentKey_lookup (field : SegmentField) :
    segmentFieldNames[(sint.Z (segmentKeyIndex field)).toNat]? = some (segmentFieldName field) := by
  cases field <;> rfl

theorem segmentFoldedNames_distinct : (segmentFieldNames.map (List.map fieldFoldByte)).Nodup :=
  classNames_distinct .segmentLengths

theorem segmentFieldNames_ascii : ∀ name ∈ segmentFieldNames, ASCIIBytes name :=
  classFieldNames_ascii .segmentLengths

/-- Unknown abstract fields exclude every actual table name. -/
theorem segmentKeyMeaning_unknown (key : GoString) (meaning : SegmentKeyMeaning key none) :
    ∀ name ∈ segmentFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [segmentFieldNames, classNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .children, meaning .length⟩

theorem segmentFieldHeads_ordered : FieldHeadsOrdered segmentFieldKeys :=
  classFieldHeads_ordered .segmentLengths

theorem segmentFieldEncodings_nonempty : ∀ key ∈ segmentFieldKeys, key.Encoded' ≠ [] :=
  classFieldEncodings_nonempty .segmentLengths

theorem segmentFieldEncodings_apart : FieldEncodingsApart segmentFieldKeys :=
  classFieldEncodings_apart .segmentLengths

theorem segmentCachedShortTable : CachedShortTable segmentFieldKeys :=
  classCachedShortTable .segmentLengths

theorem segmentInlineFieldTable : InlineFieldTable segmentFieldKeys segmentFieldNames :=
  classInlineFieldTable .segmentLengths

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentInlineFieldTable
