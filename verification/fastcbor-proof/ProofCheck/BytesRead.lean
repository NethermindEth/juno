import ProofCheck.StringFailure
import ProofCheck.InitializationErrors
import ProofCheck.SliceAllocation

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

/-- Allocate and copy a definite byte payload. Even empty output is nonnil;
    the new owned storage is separate from the unchanged source input. -/
theorem wp_bytes_encoded (c : Loc) (v : cursor.Cursor)
    (xs payload body rest : List w8) (head : w8) (dq : DFrac)
    (enc : HeadArgument head payload (W64 body.length)) (major : head &&& W8 224 = W8 64)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (dst : GoSlice), RET (PairV #dst #interface.nil); dst ↦* body ∗ ⌜dst.ptr ≠ null⌝ ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  have hread := wp_payload_encoded (GF := GF) c v xs payload body rest head dq enc hp hbytes
  simp only [major] at hread
  wp_apply hread $$ [Hc Hs] with ⟨Hc, Hs, %Hpayload⟩
  · iframe Hc Hs
  have hbodyBound : (body.length : Int) ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hbodyLen : sint.Z (slice.slice v.buf' w8 (v.pos' + W64 (1 + payload.length))
      (v.pos' + W64 (1 + payload.length + body.length))).len = body.length := by
    simp only [slice.slice]
    word
  wp_apply wp_slice_make2_nonnil (V := w8) (st := go.SliceType go.byte) _
    as %dst ⟨Hdst, Hcap, %Hnonnull⟩
  · ipureintro; dsimp only [slice.slice] at hbodyLen; omega
  have hn : sint.nat (slice.slice v.buf' w8 (v.pos' + W64 (1 + payload.length))
      (v.pos' + W64 (1 + payload.length + body.length))).len = body.length := by
    simp only [sint.nat, sint.Z] at hbodyLen ⊢
    omega
  dsimp only [slice.slice] at hn
  isimp only [hn] at Hdst
  have bounds := And.intro Hpayload.1 (And.intro Hpayload.2.1 Hpayload.2.2.1)
  icases (ownSlice_slice (v.pos' + W64 (1 + payload.length)) (v.pos' + W64 (1 + payload.length + body.length))
    v.buf' dq xs bounds).1 $$ Hs with ⟨Hpre, Hbody, Hpost⟩
  isimp only [Hpayload.2.2.2.1] at Hbody
  wp_apply wp_slice_copy (st := go.SliceType go.byte) dst (List.replicate body.length (zero_val w8)) _ body dq $$ [Hdst Hbody] as %copied ⟨%Hcopied, Hdst, Hbody⟩
  · iframe Hdst Hbody
  have updated : body.take (List.replicate body.length (zero_val w8)).length ++
      (List.replicate body.length (zero_val w8)).drop body.length = body := by simp
  isimp only [updated] at Hdst
  iapply HΦ
  iframe Hdst Hc
  isplitl []
  · ipureintro; exact Hnonnull
  isplitl [Hpre Hbody Hpost]
  · iapply (ownSlice_slice (v.pos' + W64 (1 + payload.length)) (v.pos' + W64 (1 + payload.length + body.length))
      v.buf' dq xs bounds).2
    rw [Hpayload.2.2.2.1]
    iframe Hpre Hbody Hpost
  ipureintro
  simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

/-- Invalid or wrong-type payload heads return nil and restore the cursor. -/
theorem wp_bytes_expected (c : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 64) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 64) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_payload_expected_failure c v xs wire (W8 64) dq reason kind (by decide) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- A truncated payload returns nil and restores the cursor before allocation. -/
theorem wp_bytes_truncated (c : Loc) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
    (short : rest.length < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte or text string longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  have hread := wp_payload_truncated (GF := GF) c v xs payload rest head argument dq kind enc hp hbytes short
  simp only [major] at hread
  wp_apply hread $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_encoded
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_expected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_truncated
