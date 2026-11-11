import ProofCheck.ProjectionTagSpec
import ProofCheck.IntegerKey

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

/-- Tag uses the actual shared expectation implementation for every head width. -/
theorem wp_tag_wire (c : Loc) (v : cursor.Cursor) (xs payload rest : List w8)
    (head : w8) (tag : w64) (dq : DFrac) (enc : HeadArgument head payload tag)
    (major : head &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Tag")) (Val #()))
    {{ RET (PairV #tag #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hi := input_suffix_head xs (payload ++ rest) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have ha : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rest := by
    have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ rest) _ _ hbytes
  wp_auto
  rw [← major]
  wp_apply wp_expect_encoded c v xs payload rest head tag dq enc hp.1 hbound hi ha
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro
  simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

/-- A failed Tag call propagates the actual expectation diagnostic with rollback. -/
theorem wp_tag_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 192) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Tag")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 192) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_failure c v xs wire (W8 192) dq reason kind (by decide) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- EOF and every non-tag first byte are deliberately deferred to the record. -/
theorem wp_projectionTag_untagged (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (kind : GoInterface) (plain : ¬ projectionHasTag wire)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ RET #interface.nil; c ↦ v ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  cases wire with
  | nil =>
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    wp_apply wp_peek_eof c v kind heof $$ [Hc] as %discarded ⟨Hc, Hdiscarded⟩
    · iframe Hkind Hc
    wp_end
  | cons head tail =>
    have major : head &&& W8 224 ≠ W8 192 := fun h => plain ⟨head, tail, rfl, h⟩
    wp_apply wp_peek_prefix c v xs tail head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    simp only [major, _root_.decide_false]
    wp_auto
    wp_end

/-- A malformed first tag is rejected before the reserved-tag/content checks. -/
theorem wp_projectionTag_malformed (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (tagged : projectionHasTag wire) (failure : ExpectedFailure (W8 192) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 192) reason kind }} := by
  obtain ⟨head, tail, rfl, major⟩ := tagged
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs tail head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [major]
  wp_auto
  wp_apply wp_tag_failure c v xs (head :: tail) dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_tag_wire
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_tag_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_untagged
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_malformed
