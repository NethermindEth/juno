import ProofCheck.SkipElements

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


/-- The non-shortcut array/map path composes the proved container loop. -/
theorem wp_skip_container (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument perCount depth : w64) (items : List (List w8)) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload argument)
    (hm : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
    (hd : sint.Z depth = d) (hdepth : d < 32)
    (hcount : uint.Z argument * uint.Z perCount = items.length)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item)
    (childSpec : ∀ item ∈ items, ∀ (v : cursor.Cursor) (rest : List w8),
      0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len →
      subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest →
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(depth + W64 1)))
      {{ RET #interface.nil; c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs }})
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = ((head :: payload) ++ items.flatten) ++ rem)
    (shape : ¬ SkipWideShape (((head :: payload) ++ items.flatten) ++ rem)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ RET #interface.nil;
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + items.flatten.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hitems := skipValid_items_bound (d + 1) items valid
  have hwidth := headArgument_width head payload argument enc
  have hnext := input_consumption xs ((head :: payload) ++ items.flatten) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + items.flatten.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at hnext
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ items.flatten ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (items.flatten ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ (items.flatten ++ rem)) _ _
      (by simpa only [List.append_assoc, List.cons_append, List.nil_append] using hbytes)
  have hstart : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len := by constructor <;> word
  have hchildren : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = items.flatten ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) (items.flatten ++ rem) _ _
      (by simpa only [List.append_assoc] using hbytes)
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
  by_cases hdep : sint.Z depth < sint.Z (W64 32) <;>
    simp only [hdep, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      by_cases hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos') <;>
        simp only [hl, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      have hblen : 37 ≤ (((head :: payload) ++ items.flatten) ++ rem).length := by
        have heq := Hbuflen.1
        change (((head :: payload) ++ items.flatten) ++ rem).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
        change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hl
        omega
      list_elem (((head :: payload) ++ items.flatten) ++ rem) 0 as byte0
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (((head :: payload) ++ items.flatten) ++ rem) dq byte0 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte0_lookup
      by_cases heq0 : byte0 = W8 132 <;> simp only [heq0, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      have hblen : 37 ≤ (((head :: payload) ++ items.flatten) ++ rem).length := by
        have heq := Hbuflen.1
        change (((head :: payload) ++ items.flatten) ++ rem).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
        change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hl
        omega
      list_elem (((head :: payload) ++ items.flatten) ++ rem) 1 as byte1
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (((head :: payload) ++ items.flatten) ++ rem) dq byte1 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte1_lookup
      by_cases heq1 : byte1 = W8 27 <;> simp only [heq1, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      have hblen : 37 ≤ (((head :: payload) ++ items.flatten) ++ rem).length := by
        have heq := Hbuflen.1
        change (((head :: payload) ++ items.flatten) ++ rem).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
        change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hl
        omega
      list_elem (((head :: payload) ++ items.flatten) ++ rem) 10 as byte10
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (((head :: payload) ++ items.flatten) ++ rem) dq byte10 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte10_lookup
      by_cases heq10 : byte10 = W8 27 <;> simp only [heq10, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      have hblen : 37 ≤ (((head :: payload) ++ items.flatten) ++ rem).length := by
        have heq := Hbuflen.1
        change (((head :: payload) ++ items.flatten) ++ rem).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
        change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hl
        omega
      list_elem (((head :: payload) ++ items.flatten) ++ rem) 19 as byte19
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (((head :: payload) ++ items.flatten) ++ rem) dq byte19 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte19_lookup
      by_cases heq19 : byte19 = W8 27 <;> simp only [heq19, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      have hblen : 37 ≤ (((head :: payload) ++ items.flatten) ++ rem).length := by
        have heq := Hbuflen.1
        change (((head :: payload) ++ items.flatten) ++ rem).length = (sint.Z (v.buf'.len - v.pos')).toNat at heq
        change (37 : Int) ≤ sint.Z (v.buf'.len - v.pos') at hl
        omega
      list_elem (((head :: payload) ++ items.flatten) ++ rem) 28 as byte28
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        (((head :: payload) ++ items.flatten) ++ rem) dq byte28 (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact Hbyte28_lookup
      by_cases heq28 : byte28 = W8 27 <;> simp only [heq28, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      exfalso
      apply shape
      exact ⟨by simpa only [heq0] using Hbyte0_lookup,
        by simpa only [heq1] using Hbyte1_lookup, by simpa only [heq10] using Hbyte10_lookup,
        by simpa only [heq19] using Hbyte19_lookup, by simpa only [heq28] using Hbyte28_lookup⟩
  all_goals
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    simp only [hlt, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    wp_apply wp_head_encoded c v xs payload (items.flatten ++ rem) head argument dq enc hp.1 hheadbound hi hafter
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    rcases hm with ⟨hm, rfl⟩ | ⟨hm, rfl⟩
    · simp only [hm]
      wp_auto
      wp_method_call
      wp_call
      wp_auto
      have hlimit : uint.Z argument ≤ uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) := by word
      wp_apply wp_skipElements_valid c {v with pos' := v.pos' + W64 (1 + payload.length)} xs rem items dq
        argument (v.buf'.len - (v.pos' + W64 (1 + payload.length))) (W64 1) depth d hd hdepth (by simp) hcount hlimit valid childSpec hstart hchildren
        $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      have hpos : v.pos' + W64 (1 + payload.length) + W64 items.flatten.length =
          v.pos' + W64 (1 + payload.length + items.flatten.length) := by word
      isimp only [hpos] at Hc
      wp_end
    · simp only [hm]
      wp_auto
      wp_method_call
      wp_call
      wp_auto
      have hlimit : uint.Z argument ≤ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 2) := by word
      wp_apply wp_skipElements_valid c {v with pos' := v.pos' + W64 (1 + payload.length)} xs rem items dq
        argument ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 2) (W64 2) depth d hd hdepth (by simp) hcount hlimit valid childSpec hstart hchildren
        $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      have hpos : v.pos' + W64 (1 + payload.length) + W64 items.flatten.length =
          v.pos' + W64 (1 + payload.length + items.flatten.length) := by word
      isimp only [hpos] at Hc
      wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_container
