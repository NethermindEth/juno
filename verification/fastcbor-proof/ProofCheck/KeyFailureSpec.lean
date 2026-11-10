import ProofCheck.FieldHead
import ProofCheck.FeltComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

inductive KeyFailureReason where
  | head (bad : w8) (reason : HeadError)
  | incomplete
  | eof
  | integerRange
  | tag
  | typeMismatch (bad : w8)
  | textTruncated

def keyFailureAddr (reason : KeyFailureReason) : Loc :=
  match reason with
  | .head _ reason => headFaultAddr (GF := GF) reason
  | .incomplete | .eof | .textTruncated => globalAddr cursor.ErrTruncated
  | .integerRange | .typeMismatch _ => globalAddr cursor.ErrType
  | .tag => globalAddr cursor.ErrUnsupported

def keyFailureKind (reason : KeyFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .head bad reason => skipFailureKind (.head bad reason) kinds.toSkipErrorKinds
  | .incomplete | .eof | .textTruncated => kinds.truncatedKind
  | .integerRange | .typeMismatch _ => kinds.typeKind
  | .tag => kinds.unsupportedKind

def KeyFailureError (err : Loc) (pos : w64) (reason : KeyFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .head bad reason => HeadFaultError err pos bad reason kind
  | .incomplete => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends inside the head") slice.nil)
  | .eof => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends before the item") slice.nil)
  | .integerRange => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"integer key does not fit int64") slice.nil)
  | .textTruncated => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"key longer than the input") slice.nil)
  | .tag => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"tag where %s is expected") args ∗
      args ↦* [interface.mkOk go.string #(go!"a text or integer key")])
  | .typeMismatch bad => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"expected %s, found %s") args ∗
      args ↦* [interface.mkOk go.string #(go!"a text or integer key"), interface.mkOk go.string #(headDescription bad)])

/-- Key errors whose production paths are currently closed. Every diagnostic is
    at the original key position and the cursor is restored. Invalid UTF-8 is
    a further error family awaiting the general validation contract. -/
inductive KeyFailure : List w8 → KeyFailureReason → Prop where
  | head {bad : w8} (tail : List w8) {reason : HeadError} (fault : HeadFault bad reason) :
      KeyFailure (bad :: tail) (.head bad reason)
  | incomplete {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : KeyFailure (head :: tail) .incomplete
  | eof : KeyFailure [] .eof
  | integerRange {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
      (bound : 2^63 ≤ uint.Z argument) : KeyFailure ((head :: payload) ++ tail) .integerRange
  | tag {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192) :
      KeyFailure ((head :: payload) ++ tail) .tag
  | typeMismatch {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (text : head &&& W8 224 ≠ W8 96)
      (uint : head &&& W8 224 ≠ W8 0) (negative : head &&& W8 224 ≠ W8 32)
      (tag : head &&& W8 224 ≠ W8 192) : KeyFailure ((head :: payload) ++ tail) (.typeMismatch head)
  | textTruncated {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (short : tail.length < uint.Z argument) : KeyFailure ((head :: payload) ++ tail) .textTruncated

theorem keyErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : KeyFailureReason) :
    FeltErrorGlobals kinds ⊢ (keyFailureAddr (GF := GF) reason ↦□ keyFailureKind reason kinds : IProp GF) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  iintro ⟨⟨#Hsyntax, #Htruncated, #Hunsupported, #Hdepth⟩, #Htype⟩
  cases reason with
  | head bad reason =>
    cases reason <;> isimp only [keyFailureAddr, keyFailureKind, headFaultAddr, skipFailureKind] <;>
      (first | iexact Hsyntax | iexact Hunsupported)
  | incomplete | eof | textTruncated =>
    isimp only [keyFailureAddr, keyFailureKind]
    iexact Htruncated
  | integerRange | typeMismatch _ =>
    isimp only [keyFailureAddr, keyFailureKind]
    iexact Htype
  | tag =>
    isimp only [keyFailureAddr, keyFailureKind]
    iexact Hunsupported

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.keyErrorGlobals_lookup
