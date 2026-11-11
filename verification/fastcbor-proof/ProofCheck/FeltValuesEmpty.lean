import ProofCheck.FeltValuesBoundary
import ProofCheck.SliceAllocation

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The internal writer on an empty destination performs no input reads or
    element stores and preserves the cursor at every valid position. -/
theorem wp_readLimbs_empty (c : Loc) (v : cursor.Cursor) (out : GoSlice)
    (empty : out.len = W64 0)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hb : sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ RET #interface.nil; c ↦ v }} := by
  wp_start as Hc
  wp_auto
  simp only [hp.1, hp.2, hb, and_self, ite_true]
  wp_auto
  wp_for
  simp only [empty, Int.lt_irrefl, _root_.decide_false, Bool.false_eq_true, ite_false]
  wp_auto
  have hrestore : v.buf'.len - (v.buf'.len - v.pos') = v.pos' := by word
  isimp only [hrestore] at Hc
  wp_end

/-- Every supported encoding of the empty array returns an allocated nonnil
    felt slice, consumes only its head, and restores the caller's depth. -/
theorem wp_limbSlice_empty (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (dq : DFrac) (enc : HeadArgument head payload (W64 0))
    (major : head &&& W8 224 = W8 128) (hdepth : sint.Z v.depth' < 32)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (out : GoSlice), RET (PairV #out #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       out ↦* ([] : List (GoArray w64 4)) ∗ ownSliceCap (GoArray w64 4) out (DFrac.own 1) ∗
       ⌜out.ptr ≠ null⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := headArgument_width head payload (W64 0) enc
  have hn := array_head_not_nullish head major
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload rem head (W64 0) dq enc hp hbound hi hbytes
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z (W64 0) := by word
  simp only [hok, _root_.decide_false]
  wp_auto
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' hdepth
    $$ [Hc] with Hc
  · iframe Hc
  wp_apply wp_slice_make2_nonnil (V := GoArray w64 4) (W64 0) as %out ⟨Hout, Hcap, %Hnn⟩
  · ipureintro; decide
  isimp only [show sint.nat (W64 0) = 0 from rfl, List.replicate_zero] at Hout
  ihave %Hlen := ownSlice_len _ _ _ $$ Hout
  have hempty : out.len = W64 0 := by simp only [List.length_nil] at Hlen; word
  simp only [hempty, _root_.decide_true, ite_true]
  wp_auto
  wp_apply wp_readLimbs_empty c
    {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1}
    slice.nil rfl (by simp only; constructor <;> word) Hwf.2 $$ [Hc] with Hc
  · iframe Hc
  have hrestore : v.depth' + W64 1 - W64 1 = v.depth' := by word
  isimp only [hrestore] at Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_empty

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_empty
