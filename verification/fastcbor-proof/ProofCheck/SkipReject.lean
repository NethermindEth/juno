import ProofCheck.SkipAtom

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


/-- Rejections that depend only on a complete atomic head and the available suffix. -/
inductive SkipAtomError where
  | simple
  | truncated
  | tag

def skipAtomErrorAddr (reason : SkipAtomError) : Loc :=
  match reason with
  | .simple => globalAddr cursor.ErrSyntax
  | .truncated => globalAddr cursor.ErrTruncated
  | .tag => globalAddr cursor.ErrUnsupported

def skipAtomErrorMessage (reason : SkipAtomError) : GoString :=
  match reason with
  | .simple => go!"simple value below 32 in two bytes"
  | .truncated => go!"byte or text string longer than the input"
  | .tag => go!"tag 0 to 3, whose content is not checked"

inductive SkipReject (head : w8) (argument : w64) (rem : List w8) : SkipAtomError → Prop where
  | simple (major : head &&& W8 224 = W8 224) (width : head &&& W8 31 = W8 24)
      (small : uint.Z argument < 32) : SkipReject head argument rem .simple
  | bytes (major : head &&& W8 224 = W8 64 ∨ head &&& W8 224 = W8 96)
      (short : rem.length < uint.Z argument) : SkipReject head argument rem .truncated
  | tag (major : head &&& W8 224 = W8 192) (small : uint.Z argument ≤ 3) :
      SkipReject head argument rem .tag

theorem skipReject_not_felt (head : w8) (argument : w64) (rem : List w8) (reason : SkipAtomError)
    (reject : SkipReject head argument rem reason) : head ≠ W8 132 := by
  intro heq
  subst head
  cases reject with
  | simple hm _ _ => contradiction
  | bytes hm _ => rcases hm with h | h <;> contradiction
  | tag hm _ => contradiction

/-- The worker reports a head-dependent error at the end of that head, preserving input and depth. -/
theorem wp_skip_reject (c : Loc) (v : cursor.Cursor) (depth : w64)
    (xs payload rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (reason : SkipAtomError) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (reject : SkipReject head argument rem reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hwidth := headArgument_width head payload argument enc
  have hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    rw [hp1]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  have hne := skipReject_not_felt head argument rem reason reject
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
        ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
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
    wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hheadbound hi hafter
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    cases reject with
    | simple hm hi24 hsmall =>
      isimp only [skipAtomErrorAddr] at Hkind
      isimp only [skipAtomErrorMessage] at HΦ
      simp only [hm]
      wp_auto
      simp only [hi24, _root_.decide_true]
      wp_auto
      have hs : uint.Z argument < uint.Z (W64 32) := hsmall
      simp only [hs, _root_.decide_true]
      wp_auto
      wp_apply wp_fail c {v with pos' := v.pos' + W64 (1 + payload.length)} kind
        (go!"simple value below 32 in two bytes") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hc
      wp_end
    | bytes hm hshort =>
      isimp only [skipAtomErrorAddr] at Hkind
      isimp only [skipAtomErrorMessage] at HΦ
      have hremaining : (rem.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
        have hcount := congrArg List.length hbytes
        simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
          List.length_append, List.length_cons] at hcount
        simp only [sint.nat, sint.Z] at Hlen hcount hp ⊢
        omega
      rcases hm with hm | hm
      all_goals
        simp only [hm]
        wp_auto
        wp_method_call
        wp_call
        wp_auto
        have hbad : uint.Z (v.buf'.len - (v.pos' + W64 (1 + payload.length))) < uint.Z argument := by word
        simp only [hbad, _root_.decide_true]
        wp_auto
        wp_apply wp_fail c {v with pos' := v.pos' + W64 (1 + payload.length)} kind
          (go!"byte or text string longer than the input") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
        · iframe Hc
        wp_end
    | tag hm hsmall =>
      isimp only [skipAtomErrorAddr] at Hkind
      isimp only [skipAtomErrorMessage] at HΦ
      simp only [hm]
      wp_auto
      have hs : uint.Z argument ≤ uint.Z (W64 3) := hsmall
      simp only [hs, _root_.decide_true]
      wp_auto
      wp_apply wp_fail c {v with pos' := v.pos' + W64 (1 + payload.length)} kind
        (go!"tag 0 to 3, whose content is not checked") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hc
      wp_end

/-- Public Skip rolls back the cursor while preserving the worker's diagnostic offset. -/
theorem wp_Skip_reject (c : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (reason : SkipAtomError) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (reject : SkipReject head argument rem reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       skipAtomErrorAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length)) (skipAtomErrorMessage reason) slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_skip_reject c v v.depth' xs payload rem head argument dq reason kind enc reject hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- An absent item is reported without changing any cursor field. -/
theorem wp_skip_eof (c : Loc) (v : cursor.Cursor) (depth : w64) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len)
    (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [heof]; exact ⟨hwf.1, Int.le_refl _, hwf.2⟩))]
  wp_auto
  by_cases hd : sint.Z depth < sint.Z (W64 32) <;> simp only [hd, _root_.decide_true, _root_.decide_false] <;> wp_auto
  all_goals
    simp only [slice.slice, heof, BitVec.sub_self]
    try wp_auto
    simp only [heof, Int.lt_irrefl, _root_.decide_false]
    wp_auto
    wp_apply wp_head_eof c v kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    isimp only [heof] at Herr
    wp_end

/-- Public Skip retains the EOF offset and unchanged cursor. -/
theorem wp_Skip_eof (c : Loc) (v : cursor.Cursor) (kind : GoInterface)
    (heof : v.pos' = v.buf'.len)
    (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  wp_apply wp_skip_eof c v v.depth' kind heof hwf $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hkind Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipReject_not_felt
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_reject

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_eof
