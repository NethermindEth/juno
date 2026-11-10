import ProofCheck.FeltDispatch

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


/-- The nonrecursive values accepted by Skip. Text payloads deliberately need not be UTF-8. -/
inductive SkipAtom (head : w8) (argument : w64) : List w8 → Prop where
  | integer (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32) : SkipAtom head argument []
  | simple (major : head &&& W8 224 = W8 224)
      (preferred : head &&& W8 31 ≠ W8 24 ∨ 32 ≤ uint.Z argument) : SkipAtom head argument []
  | bytes (major : head &&& W8 224 = W8 64 ∨ head &&& W8 224 = W8 96)
      (body : List w8) (length : uint.Z argument = body.length) : SkipAtom head argument body

theorem skipAtom_not_felt (head : w8) (argument : w64) (body : List w8)
    (atom : SkipAtom head argument body) : head ≠ W8 132 := by
  intro heq
  subst head
  cases atom with
  | integer hm => rcases hm with h | h <;> contradiction
  | simple hm _ => contradiction
  | bytes hm _ _ => rcases hm with h | h <;> contradiction

/-- The recursive worker consumes any complete atomic item, preserving input and cursor depth. -/
theorem wp_skip_atom (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs payload body rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument) (atom : SkipAtom head argument body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ RET #interface.nil;
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := headArgument_width head payload argument enc
  have hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at hnext
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ body ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ (body ++ rem)) _ _ (by simpa only [List.append_assoc, List.cons_append, List.nil_append] using hbytes)
  have hne := skipAtom_not_felt head argument body atom
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  by_cases hd : sint.Z depth < sint.Z (W64 32) <;> simp only [hd, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      by_cases hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos') <;>
        simp only [hl, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    try
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
      icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
      isimp only [hbytes] at Hbuf
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        ((head :: payload) ++ body ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; rfl
      simp only [hne, _root_.decide_false]
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
        rw [hbytes]
        iframe Hpre Hbuf Hpost
  all_goals
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    simp only [hlt, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    wp_apply wp_head_encoded c v xs payload (body ++ rem) head argument dq enc hp.1 hheadbound hi hafter
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    cases atom with
    | integer hm =>
      rcases hm with hm | hm
      all_goals
        simp only [hm]
        wp_auto
        wp_end
    | simple hm hs =>
      simp only [hm]
      wp_auto
      rcases hs with hs | hs
      · simp only [hs, _root_.decide_false]
        wp_auto
        wp_end
      · by_cases hi24 : head &&& W8 31 = W8 24
        · have hbad : ¬ uint.Z argument < uint.Z (W64 32) := by change ¬ uint.Z argument < 32; omega
          simp only [hi24, _root_.decide_true]
          wp_auto
          simp only [hbad, _root_.decide_false]
          wp_auto
          wp_end
        · simp only [hi24, _root_.decide_false]
          wp_auto
          wp_end
    | bytes hm body harg =>
      rcases hm with hm | hm
      all_goals
        simp only [hm]
        wp_auto
        wp_method_call
        wp_call
        wp_auto
        have hfits : ¬ uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z argument := by word
        simp only [hfits, _root_.decide_false]
        wp_auto
        have hpos : v.pos' + W64 (1 + payload.length) + argument = v.pos' + W64 (1 + payload.length + body.length) := by word
        isimp only [hpos] at Hc
        wp_end

/-- Public Skip preserves the exact trailing input for every accepted atomic encoding. -/
theorem wp_Skip_atom (c : Loc) (v : cursor.Cursor)
    (xs payload body rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument) (atom : SkipAtom head argument body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ RET #interface.nil;
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + (1 + payload.length + body.length) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1] at Hnext
  wp_auto
  wp_apply wp_skip_atom c v v.depth' xs payload body rem head argument dq enc atom hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipAtom_not_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_atom

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_atom
