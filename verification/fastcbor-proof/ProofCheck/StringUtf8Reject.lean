import ProofCheck.Payload
import ProofCheck.Utf8Reject

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

/-- A complete text payload with an illegal starter is rejected by the actual
    validator. The cursor rolls back and the diagnostic records its original position. -/
theorem wp_string_bad_lead (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (major : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"String")) (Val #()))
    {{ (err : Loc), RET (PairV #(go!"") #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"text string is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  rw [← major]
  wp_apply wp_payload_encoded c v xs payload (b :: tail) rem head dq enc hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hrange⟩
  · iframe Hc Hs
  wp_apply wp_utf8_bad_lead_range v.buf' xs b tail
    (v.pos' + W64 (1 + payload.length))
    (v.pos' + W64 (1 + payload.length + (b :: tail).length)) dq bad
    ⟨Hrange.1, Hrange.2.1, Hrange.2.2.1⟩ Hrange.2.2.2.1 $$ [Hs] with Hs
  · iframe Hs
  wp_apply wp_fail c v kind (go!"text string is not valid UTF-8") slice.nil
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- The typed reader preserves the old string on UTF-8 rejection. -/
theorem wp_readString_bad_lead (c dst : Loc) (old : GoString) (v : cursor.Cursor)
    (xs payload tail rem : List w8) (head b : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (major : head &&& W8 224 = W8 96) (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"text string is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ (b :: tail) ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn0 : head ≠ W8 246 := by intro h; subst head; contradiction
  have hn1 : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn0 hn1 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_string_bad_lead c v xs payload tail rem head b dq kind enc major bad hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_string_bad_lead
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_bad_lead
