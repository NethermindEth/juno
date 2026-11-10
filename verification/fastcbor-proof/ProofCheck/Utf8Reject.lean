import ProofCheck.Utf8Tables

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The real Go UTF-8 validator rejects every illegal starter, independently of
    the remaining bytes and without invoking its generic word fast path. -/
theorem wp_utf8_bad_lead (data : GoSlice) (b : w8) (rest : List w8) (dq : DFrac)
    (bad : Utf8BadLead b) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} (b :: rest) }}
      (App (Val (@! unicode.utf8.Valid)) (Val #data))
    {{ RET #false; data ↦*{dq} (b :: rest) }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave Htables := utf8_initialized_tables (GF := GF) $$ []
  · solve_pkg_init
  isimp only [Utf8Initialized] at Htables
  icases Htables with ⟨#Hfirst, #Haccept⟩
  have hfull : slice.fullSlice data w8 (W64 0) data.len data.len =
      slice.mk data.ptr data.len data.len := by
    simp only [slice.fullSlice, sliceIndexRef, show sint.Z (W64 0) = 0 from rfl,
      go.arrayIndexRef_0, show data.len - W64 0 = data.len by word]
  have hpositive : 0 < sint.Z data.len := by
    simp only [List.length_cons, sint.nat, sint.Z] at Hlen ⊢
    omega
  have hascii : ¬ uint.Z b < 128 := by unfold Utf8BadLead at bad; omega
  have hbounds : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.len ∧ sint.Z data.len ≤ sint.Z data.cap :=
    ⟨by decide, Hwf.1, Int.le_refl _, Hwf.2⟩
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbounds)]
  simp only [hfull]
  wp_auto
  wp_for
  simp only [show sint.Z (W64 0) = 0 from rfl, hpositive, _root_.decide_true, ite_true]
  wp_auto
  have hindex : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) < sint.Z data.len := ⟨by decide, hpositive⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
  have href : sliceIndexRef w8 (sint.Z (W64 0)) (slice.mk data.ptr data.len data.len) =
      sliceIndexRef w8 0 data := rfl
  rw [href]
  wp_apply wp_load_slice_index (t := go.byte) data 0 (b :: rest) dq b (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; rfl
  simp only [show uint.Z (W8 128) = 128 from rfl, hascii, _root_.decide_false, ite_false]
  wp_auto
  have hbyte : 0 ≤ sint.Z (W64 (uint.Z b)) ∧ sint.Z (W64 (uint.Z b)) < 256 := by word
  have hcast : sint.Z (W64 (uint.Z b)) = uint.Z b := by word
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbyte), hcast]
  icases array_acc (globalAddr unicode.utf8.first) (uint.Z b) DFrac.discard 256
    utf8FirstTable (W8 241) (by word) (utf8FirstTable_bad_lead b bad) $$ Hfirst
    with ⟨#Hclass, _⟩
  icases array_acc (globalAddr unicode.utf8.acceptRanges) (15 : Int) DFrac.discard 16
    utf8AcceptTable (unicode.utf8.acceptRange.mk (W8 0) (W8 0)) (by decide)
    utf8AcceptTable_invalid $$ Haccept with ⟨#Hrange, _⟩
  wp_auto
  rw [show sint.Z (W64 (uint.Z (W8 241 >>> W64 4))) = 15 by decide]
  wp_auto
  wp_for_post
  wp_end

/-- Rejection of a borrowed text range restores ownership of the entire input. -/
theorem wp_utf8_bad_lead_range (data : GoSlice) (xs : List w8) (b : w8)
    (rest : List w8) (low high : w64) (dq : DFrac) (bad : Utf8BadLead b)
    (hbound : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat low) (sint.nat high) xs = b :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} xs }}
      (App (Val (@! unicode.utf8.Valid)) (Val #(slice.slice data w8 low high)))
    {{ RET #false; data ↦*{dq} xs }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice low high data dq xs hbound).1 $$ Hs with ⟨Hpre, Hbody, Hpost⟩
  isimp only [hbytes] at Hbody
  wp_apply wp_utf8_bad_lead (slice.slice data w8 low high) b rest dq bad $$ [Hbody] with Hbody
  · iframe Hbody
  iapply HΦ
  iapply (ownSlice_slice low high data dq xs hbound).2
  rw [hbytes]
  iframe Hpre Hbody Hpost

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_bad_lead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_utf8_bad_lead_range
