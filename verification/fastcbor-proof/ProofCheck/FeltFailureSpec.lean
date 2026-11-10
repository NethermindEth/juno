import ProofCheck.SkipComplete

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

/-- A failed major-type expectation reports at the start of the requested item.
    Complete tags have their own unsupported diagnostic. Head errors take
    precedence over type checking. -/
inductive ExpectedFailureReason where
  | head (bad : w8) (reason : HeadError)
  | incomplete
  | eof
  | typeMismatch (bad : w8)
  | tag

inductive ExpectedFailure (want : w8) : List w8 → ExpectedFailureReason → Prop where
  | head {bad : w8} (tail : List w8) {reason : HeadError} (fault : HeadFault bad reason) :
      ExpectedFailure want (bad :: tail) (.head bad reason)
  | incomplete {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : ExpectedFailure want (head :: tail) .incomplete
  | eof : ExpectedFailure want [] .eof
  | typeMismatch {head : w8} {payload : List w8} (tail : List w8) {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 ≠ want)
      (notTag : head &&& W8 224 ≠ W8 192) :
      ExpectedFailure want ((head :: payload) ++ tail) (.typeMismatch head)
  | tag {head : w8} {payload : List w8} (tail : List w8) {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 ≠ want)
      (isTag : head &&& W8 224 = W8 192) : ExpectedFailure want ((head :: payload) ++ tail) .tag

def expectedFailureAddr (reason : ExpectedFailureReason) : Loc :=
  match reason with
  | .head _ reason => headFaultAddr (GF := GF) reason
  | .incomplete | .eof => globalAddr cursor.ErrTruncated
  | .typeMismatch _ => globalAddr cursor.ErrType
  | .tag => globalAddr cursor.ErrUnsupported

def ExpectedFailureError (err : Loc) (pos : w64) (want : w8)
    (reason : ExpectedFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .head bad reason => HeadFaultError err pos bad reason kind
  | .incomplete => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends inside the head") slice.nil)
  | .eof => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends before the item") slice.nil)
  | .typeMismatch bad => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"expected %s, found %s") args ∗
      args ↦* [interface.mkOk go.string #(headDescription want), interface.mkOk go.string #(headDescription bad)])
  | .tag => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"tag where %s is expected") args ∗
      args ↦* [interface.mkOk go.string #(headDescription want)])

inductive FeltFailureReason where
  | expected (want : w8) (reason : ExpectedFailureReason)
  | depth
  | skipped (reason : SkipFailureReason)

def feltFailureAddr (reason : FeltFailureReason) : Loc :=
  match reason with
  | .expected _ reason => expectedFailureAddr (GF := GF) reason
  | .depth => globalAddr cursor.ErrDepth
  | .skipped reason => skipFailureAddr (GF := GF) reason

def FeltFailureError (err : Loc) (pos : w64) (reason : FeltFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .expected want reason => ExpectedFailureError err pos want reason kind
  | .depth => SkipFailureError err pos .depth kind
  | .skipped reason => SkipFailureError err pos reason kind

/-- A felt failure, independent of shortcut selection. The first four children
    must be unsigned; any later children use the ignored-value contract. Unlike
    generic Skip containers, the felt reader has no preflight count/byte check. -/
inductive FeltFailure : Nat → List w8 → Nat → FeltFailureReason → Prop where
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      FeltFailure d wire 0 (.expected (W8 128) reason)
  | depth {head : w8} {payload : List w8} (tail : List w8) {count : w64}
      (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128) :
      FeltFailure 32 ((head :: payload) ++ tail) 0 .depth
  | limb {d : Nat} {head : w8} {payload tail : List w8} {count : w64}
      (items : List UnsignedWire) {reason : ExpectedFailureReason}
      (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128)
      (bound : d < 32) (short : items.length < 4) (more : items.length < uint.Z count)
      (failure : ExpectedFailure (W8 0) tail reason) :
      FeltFailure d ((head :: payload) ++ unsignedBytes items ++ tail)
        (1 + payload.length + (unsignedBytes items).length) (.expected (W8 0) reason)
  | extras {d offset : Nat} {head : w8} {payload tail : List w8} {count : w64}
      (items : List UnsignedWire) (extras : List (List w8)) {reason : SkipFailureReason}
      (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128)
      (bound : d < 32) (four : items.length = 4) (more : 4 + extras.length < uint.Z count)
      (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
      (failure : SkipFailure (d + 1) tail offset reason) :
      FeltFailure d ((head :: payload) ++ unsignedBytes items ++ extras.flatten ++ tail)
        (1 + payload.length + (unsignedBytes items).length + extras.flatten.length + offset) (.skipped reason)

/-- Every specified diagnostic is within the unread input or exactly at EOF. -/
theorem feltFailure_offset (d offset : Nat) (wire : List w8) (reason : FeltFailureReason)
    (failure : FeltFailure d wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | outer _ _ _ => omega
  | depth _ _ _ => omega
  | limb items enc major bound short more failure =>
    simp only [List.length_append, List.length_cons]; omega
  | @extras d offset head payload tail count items extras reason enc major bound four more valid failure =>
    have := skipFailure_offset (d + 1) offset tail reason failure
    simp only [List.length_append, List.length_cons]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltFailure_offset
