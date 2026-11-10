import ProofCheck.SkipTaggedErrors

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


/-- At the depth limit, a further tagged child is rejected at the end of the
    outer tag head, before decoding that child. -/
theorem wp_skip_tag_depth (c : Loc) (v : cursor.Cursor) (xs payload tail rem : List w8)
    (head next : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument) (ht : next &&& W8 224 = W8 192)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = ((head :: payload) ++ (next :: tail)) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(W64 32)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length))
         (go!"nested more than %d levels") args ∗ args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := headArgument_width head payload argument enc
  have hnext := input_consumption xs ((head :: payload) ++ (next :: tail)) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + (next :: tail).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at hnext ⊢
    omega
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ (next :: tail) ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ ((next :: tail) ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ ((next :: tail) ++ rem)) _ _
      (by simpa only [List.append_assoc, List.cons_append, List.nil_append] using hbytes)
  have hne : head ≠ W8 132 := by intro heq; rw [heq] at hm; contradiction
  have hstart : 0 ≤ sint.Z (v.pos' + W64 (1 + payload.length)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) < sint.Z v.buf'.len := by
    simp only [List.length_cons] at hbound
    constructor <;> word
  have hchild : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = (next :: tail) ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) ((next :: tail) ++ rem) _ _
      (by simpa only [List.append_assoc] using hbytes)
  have hinext := input_suffix_head xs (tail ++ rem) (sint.nat (v.pos' + W64 (1 + payload.length)))
    (sint.nat v.buf'.len) next hchild
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  all_goals
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    simp only [hlt, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    wp_apply wp_head_encoded c v xs payload ((next :: tail) ++ rem) head argument dq enc hp.1 hheadbound hi hafter
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    simp only [hm]
    wp_auto
    have hnumber' : ¬ uint.Z argument ≤ uint.Z (W64 3) := by change ¬ uint.Z argument ≤ 3; omega
    simp only [hnumber', _root_.decide_false]
    wp_auto
    wp_apply wp_peek_at c {v with pos' := v.pos' + W64 (1 + payload.length)} xs next dq hstart hinext
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    simp only [ht, _root_.decide_true]
    wp_auto
    wp_apply wp_errDepth (v.pos' + W64 (1 + payload.length)) kind as %err Herr
    · iframe Hkind
    wp_end

/-- Peek reports EOF without an input load or cursor change. -/
theorem wp_peek_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Peek")) (Val #()))
    {{ (err : Loc), RET (PairV #(W8 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  simp only [heof, Int.le_refl, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"input ends before the item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  isimp only [heof] at Herr
  wp_end

/-- A supported tag with no content reports EOF after its head. Peek
    fails first; the actual recursive worker supplies the returned diagnostic. -/
theorem wp_skip_tag_eof (c : Loc) (v : cursor.Cursor) (xs payload : List w8)
    (head : w8) (argument depth : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hm : head &&& W8 224 = W8 192)
    (hnumber : 3 < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: payload) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.buf'.len} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.buf'.len (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := headArgument_width head payload argument enc
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢
    omega
  have heof : v.pos' + W64 (1 + payload.length) = v.buf'.len := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen
    word
  have hi := input_suffix_head xs payload (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ [] := by
    rw [hp1]
    simpa only [List.append_nil, List.length_cons, List.length_nil, Nat.zero_add] using input_suffix_after xs [head] payload _ _ hbytes
  have hne : head ≠ W8 132 := by intro heq; rw [heq] at hm; contradiction
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
        (head :: payload) dq head (by decide) $$ [Hbuf] with Hbuf
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
    wp_apply wp_head_encoded c v xs payload [] head argument dq enc hp.1 hheadbound hi hafter
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    simp only [hm]
    wp_auto
    have hnumber' : ¬ uint.Z argument ≤ uint.Z (W64 3) := by change ¬ uint.Z argument ≤ 3; omega
    simp only [hnumber', _root_.decide_false]
    wp_auto
    isimp only [heof] at Hc
    wp_apply wp_peek_eof c {v with pos' := v.buf'.len} kind rfl $$ [Hc] as %peekErr ⟨Hc, Hpeek⟩
    · iframe Hkind Hc
    wp_apply wp_skip_eof c {v with pos' := v.buf'.len} depth kind rfl Hwf $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tag_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_peek_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_tag_eof
