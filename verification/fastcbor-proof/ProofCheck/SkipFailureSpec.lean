import ProofCheck.SkipContextBoundary

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Error categories and diagnostic contents of the independent ignored-value contract. -/
inductive SkipFailureReason where
  | head (bad : w8) (reason : HeadError)
  | incomplete
  | atomic (reason : SkipAtomError)
  | eof
  | count
  | depth

def skipFailureAddr (reason : SkipFailureReason) : Loc :=
  match reason with
  | .head _ reason => headFaultAddr (GF := GF) reason
  | .atomic reason => skipAtomErrorAddr (GF := GF) reason
  | .incomplete | .eof | .count => globalAddr cursor.ErrTruncated
  | .depth => globalAddr cursor.ErrDepth

def SkipFailureError (err : Loc) (pos : w64) (reason : SkipFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .head bad reason => HeadFaultError err pos bad reason kind
  | .atomic reason => iprop(err ↦ cursor.DecodeError.mk kind pos (skipAtomErrorMessage reason) slice.nil)
  | .incomplete => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends inside the head") slice.nil)
  | .eof => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"input ends before the item") slice.nil)
  | .count => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"count exceeds the input") slice.nil)
  | .depth => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind pos (go!"nested more than %d levels") args ∗
      args ↦* [interface.mkOk go.int #(W64 32)])

/-- Failure at the current item. Offsets distinguish untouched malformed heads,
    complete-head diagnostics, and EOF; precedence conditions are explicit. -/
inductive SkipFailureLeaf : Nat → List w8 → Nat → SkipFailureReason → Prop where
  | head {d : Nat} {bad : w8} (tail : List w8) {reason : HeadError} (fault : HeadFault bad reason) :
      SkipFailureLeaf d (bad :: tail) 0 (.head bad reason)
  | incomplete {d : Nat} {head : w8} (tail : List w8) {size : w64}
      (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
      (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
      (short : tail.length < sint.Z size) : SkipFailureLeaf d (head :: tail) 0 .incomplete
  | atomic {d : Nat} {head : w8} {payload tail : List w8} {argument : w64} {reason : SkipAtomError}
      (enc : HeadArgument head payload argument) (reject : SkipReject head argument tail reason) :
      SkipFailureLeaf d ((head :: payload) ++ tail) (1 + payload.length) (.atomic reason)
  | eof (d : Nat) : SkipFailureLeaf d [] 0 .eof
  | count {d : Nat} {head : w8} {payload tail : List w8} {argument perCount : w64}
      (enc : HeadArgument head payload argument)
      (major : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
      (depth : d < 32) (short : tail.length < uint.Z argument * uint.Z perCount) :
      SkipFailureLeaf d ((head :: payload) ++ tail) (1 + payload.length) .count
  | containerDepth {head : w8} {payload tail : List w8} {argument perCount : w64}
      (enc : HeadArgument head payload argument)
      (major : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2)) :
      SkipFailureLeaf 32 ((head :: payload) ++ tail) (1 + payload.length) .depth
  | tagEOF {d : Nat} {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) : SkipFailureLeaf d (head :: payload) (1 + payload.length) .eof
  | tagDepth {head next : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (nextTag : next &&& W8 224 = W8 192) :
      SkipFailureLeaf 32 ((head :: payload) ++ (next :: tail)) (1 + payload.length) .depth

/-- A failure leaf reached through supported enclosing contexts and valid siblings. -/
inductive SkipFailure (d : Nat) : List w8 → Nat → SkipFailureReason → Prop where
  | within {front tail : List w8} {leaf offset : Nat} {reason : SkipFailureReason}
      (context : SkipFailureContext tail d front leaf) (failure : SkipFailureLeaf leaf tail offset reason) :
      SkipFailure d (front ++ tail) (front.length + offset) reason

/-- Leaf diagnostics are always at or before the end of their remaining input. -/
theorem skipFailureLeaf_offset (d offset : Nat) (wire : List w8) (reason : SkipFailureReason)
    (failure : SkipFailureLeaf d wire offset reason) : offset ≤ wire.length := by
  cases failure <;> simp only [List.length_append, List.length_cons, List.length_nil] <;> omega

/-- Enclosing contexts preserve the diagnostic's in-input or EOF position. -/
theorem skipFailure_offset (d offset : Nat) (wire : List w8) (reason : SkipFailureReason)
    (failure : SkipFailure d wire offset reason) : offset ≤ wire.length := by
  cases failure with
  | @within front tail leaf offset reason context failure =>
    have h := skipFailureLeaf_offset leaf offset tail reason failure
    simp only [List.length_append]; omega

/-- Closed malformed-head propagation through the shared failure contexts. -/
theorem wp_Skip_context_head (c : Loc) (v : cursor.Cursor)
    (xs front tail : List w8) (bad : w8) (dq : DFrac) (d leaf : Nat) (reason : HeadError) (kind : GoInterface)
    (context : SkipFailureContext (bad :: tail) d front leaf) (fault : HeadFault bad reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = front ++ (bad :: tail)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err (v.pos' + W64 front.length) bad reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs front (bad :: tail) v.pos' v.buf'.len hp Hlen.1 hbytes
  have hpos : 0 ≤ sint.Z (v.pos' + W64 front.length) ∧
      sint.Z (v.pos' + W64 front.length) ≤ sint.Z v.buf'.len := by constructor <;> word
  have hinfo := headFault_info bad reason fault
  have short : SkipNonWideUint (bad :: tail) := by
    intro hh
    have heq : bad = W8 27 := by simpa using hh
    subst bad; contradiction
  have rejected : SkipWideRejected leaf (bad :: tail) := by
    intro hd hl shape
    have heq : bad = W8 132 := by simpa using shape.1
    subst bad; contradiction
  wp_apply wp_Skip_context_reject c v xs front (bad :: tail) dq (v.pos' + W64 front.length) d leaf
    iprop(headFaultAddr (GF := GF) reason ↦□ kind) (fun err => HeadFaultError err (v.pos' + W64 front.length) bad reason kind)
    context short rejected hd hdepth hp hbytes
    (wp_skip_fault c {v with pos' := v.pos' + W64 front.length} (W64 leaf) xs tail bad dq reason kind fault hpos Hnext.2.2)
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureLeaf_offset
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_offset
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_context_head
