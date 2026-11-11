import ProofCheck.BytesIntoRead

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

/-- A complete payload of the wrong length restores the cursor and never
    writes the destination. The diagnostic owns both actual length arguments. -/
theorem wp_bytesInto_size (c : Loc) (dst : GoSlice) (old : List w8) (v : cursor.Cursor)
    (xs payload body rest : List w8) (head : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 body.length)) (major : head &&& W8 224 = W8 64)
    (size : body.length ≠ old.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesInto")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"byte string of %d bytes, destination of %d") args ∗
       args ↦* [interface.mkOk go.int #(W64 body.length), interface.mkOk go.int #(W64 old.length)] }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %HdstLen := ownSlice_len _ _ _ $$ Hdst
  have Hnext := input_consumption xs ((head :: payload) ++ body) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
  wp_auto
  have hread := wp_payload_encoded (GF := GF) c v xs payload body rest head dq enc hp hbytes
  simp only [major] at hread
  wp_apply hread $$ [Hc Hs] with ⟨Hc, Hs, %Hpayload⟩
  · iframe Hc Hs
  have hbody : v.pos' + W64 (1 + payload.length + body.length) - (v.pos' + W64 (1 + payload.length)) = W64 body.length := by word
  have hdst : dst.len = W64 old.length := by word
  have different : ¬ W64 body.length = dst.len := by word
  simp only [hbody, different, _root_.decide_false]
  wp_auto
  wp_apply wp_slice_literal (V := GoInterface)
    [interface.mkOk go.int #(W64 body.length), interface.mkOk go.int #(W64 old.length)]
  isplitr
  · ipureintro
    simp only [hbody, hdst]
    rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_apply wp_fail c v kind (go!"byte string of %d bytes, destination of %d") _ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesInto_size
