import ProofCheck.SkipSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]


/-- The production skip shortcut reads only the five shape bytes and advances by 37. -/
theorem wp_skip_wide_shape (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs buf : List w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hdepth : sint.Z depth < 32) (hbound : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = buf)
    (shape : SkipWideShape buf) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ RET #interface.nil; c ↦ {v with pos' := v.pos' + W64 37} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hp2 : sint.Z v.pos' ≤ sint.Z v.buf'.len := by omega
  have hd : sint.Z depth < sint.Z (W64 32) := hdepth
  have hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos') := by word
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hp2, Hwf.2⟩)]
  wp_auto
  simp only [hd, _root_.decide_true]
  wp_auto
  simp only [hl, _root_.decide_true]
  wp_auto
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp, hp2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  obtain ⟨h0, h1, h10, h19, h28⟩ := shape
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _ buf dq (W8 132)
    (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; exact h0
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _ buf dq (W8 27)
    (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; exact h1
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _ buf dq (W8 27)
    (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; exact h10
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _ buf dq (W8 27)
    (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; exact h19
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - v.pos'); word⟩)]
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _ buf dq (W8 27)
    (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; exact h28
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp, hp2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_end

/-- Public Skip's wide path consumes exactly one value admitted by the recursive specification. -/
theorem wp_Skip_wide_valid (c : Loc) (v : cursor.Cursor) (xs wire rem : List w8) (dq : DFrac)
    (depth : Nat) (hdepth : sint.Z v.depth' = depth) (hd : depth < 32)
    (valid : SkipValid depth wire) (shape : SkipWideShape (wire ++ rem))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ RET #interface.nil; c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hsize := skipValid_wide_length depth wire rem valid shape
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hb : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len := by rw [hsize] at Hnext; omega
  wp_auto
  wp_apply wp_skip_wide_shape c v v.depth' xs (wire ++ rem) dq hp.1 (by omega) hb hbytes shape
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  isimp only [hsize] at HΦ
  rw [hsize] at Hnext
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_wide_shape
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_wide_valid
