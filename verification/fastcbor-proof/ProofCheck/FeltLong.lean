import ProofCheck.SkipValid

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


/-- Every encoded unsigned limb contains at least its head byte. -/
theorem unsignedBytes_lower_bound (items : List UnsignedWire) : items.length ≤ (unsignedBytes items).length := by
  induction items with
  | nil => simp [unsignedBytes]
  | cons item items ih =>
    simp only [unsignedBytes, List.flatMap_cons, UnsignedWire.bytes, List.length_append, List.length_cons] at *
    omega

/-- The actual slow loop stores four limbs and skips every additional recursively valid value. -/
theorem wp_limbsSlow_extended (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload (W64 (4 + extras.length))) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 (4 + extras.length)) enc
  have hu := unsignedBytes_lower_bound items
  have he := skipValid_items_bound (d + 1) extras valid
  have htotal : sint.Z v.pos' + 1 + payload.length + (unsignedBytes items).length + extras.flatten.length ≤ sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hcountfit : (4 : Int) + extras.length ≤ sint.Z v.buf'.len := by omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ extras.flatten ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (unsignedBytes items ++ extras.flatten ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _
      (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  rw [← hm]
  wp_apply wp_expect_encoded c v xs payload (unsignedBytes items ++ extras.flatten ++ rem) head (W64 (4 + extras.length)) dq
    enc hp.1 hbound hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' (by change sint.Z v.depth' < 32; omega)
    $$ [Hc] with Hc
  · iframe Hc
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      unsignedBytes items ++ extras.flatten ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) _ _ _ (by simpa only [List.append_assoc] using hbytes)
  ihave HI : (∃ (done todo : List UnsignedWire) (passed pending : List (List w8)) (pos oldi : w64),
      "Hrange" ∷ «$range_i_ptr» ↦ W64 (done.length + passed.length) ∗
      "Hi" ∷ i_ptr ↦ oldi ∗
      "Hlimbs" ∷ limbs_ptr ↦ paddedLimbs done ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hextras" ∷ ⌜extras = passed ++ pending⌝ ∗
      "%Horder" ∷ ⌜done.length < 4 → passed = []⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (unsignedBytes done).length + passed.flatten.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = unsignedBytes todo ++ pending.flatten ++ rem⌝ : IProp GF)
      $$ [«$range_i» i limbs Hc]
  · iexists [], items, [], extras, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨rfl, rfl, by simp, ⟨by word, by word⟩, by simpa [unsignedBytes] using hstart, hremaining⟩
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  have hlength : done.length + todo.length = 4 := by rw [← hcount, Hsplit, List.length_append]
  have hextralength : extras.length = passed.length + pending.length := by rw [Hextras, List.length_append]
  have hdone : done.length ≤ 4 := by omega
  by_cases hmore : done.length + passed.length < 4 + extras.length
  · have hcond : uint.Z (W64 (done.length + passed.length)) < uint.Z (W64 (4 + extras.length)) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    by_cases hfull : 4 ≤ done.length + passed.length
    · have htest : uint.Z (W64 4) ≤ uint.Z (W64 (done.length + passed.length)) := by word
      simp only [htest, _root_.decide_true]
      wp_auto
      have hfour : done.length = 4 := by
        by_cases hn : done.length = 4
        · exact hn
        have heq := Horder (by omega)
        simp only [heq, List.length_nil] at hfull
        omega
      have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
      cases pending with
      | nil => simp only [List.length_nil] at hextralength; omega
      | cons item pending =>
        have hmember : item ∈ extras := by rw [Hextras]; simp
        have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ (pending.flatten ++ rem) := by
          simpa only [htodo, unsignedBytes, List.flatMap_nil, List.nil_append, List.flatten_cons, List.append_assoc] using Hremaining
        wp_apply wp_Skip_valid c {v with pos' := pos, depth' := v.depth' + W64 1} xs item (pending.flatten ++ rem) dq
          (d + 1) (by change sint.Z (v.depth' + W64 1) = (d + 1 : Nat); word) (by omega) (valid item hmember) Hpos hitem
          $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
        · iframe Hc Hs
        ieval (change (c ↦ {v with pos' := pos + W64 item.length, depth' := v.depth' + W64 1} : IProp GF) ⊢ _) at Hc
        dsimp only at Hnext
        have hbyteslen : (passed ++ [item]).flatten.length = passed.flatten.length + item.length := by simp
        have hinc : W64 (done.length + passed.length) + W64 1 = W64 (done.length + (passed ++ [item]).length) := by
          simp only [List.length_append, List.length_singleton]; word
        wp_for_post
        isimp only [hinc] at Hrange
        iframe
        iexists done, todo, (passed ++ [item]), pending, (pos + W64 item.length), (W64 (done.length + passed.length))
        iframe
        ipureintro
        refine ⟨Hsplit, ?_, ?_, ⟨?_, Hnext.2.1⟩, ?_, ?_⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hextras
        · intro hn; omega
        · rw [Hnext.1]; omega
        · rw [Hnext.1, hbyteslen]; omega
        · simpa only [htodo, unsignedBytes, List.flatMap_nil, List.nil_append] using Hnext.2.2
    · have hnotfull : ¬ uint.Z (W64 4) ≤ uint.Z (W64 (done.length + passed.length)) := by word
      simp only [hnotfull, _root_.decide_false]
      wp_auto
      have hsmall : done.length < 4 := by omega
      have hpassed := Horder hsmall
      cases todo with
      | nil => simp only [List.length_nil] at hlength; omega
      | cons item todo =>
        have hindex : sint.Z (W64 (done.length + passed.length)) = done.length := by rw [hpassed]; simp only [List.length_nil]; word
        rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hindex]; exact ⟨by omega, by omega⟩))]
        wp_auto
        wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
            (unsignedBytes todo ++ pending.flatten ++ rem) item dq Hpos
            (by simpa [unsignedBytes, List.append_assoc] using Hremaining)
            $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
        · iframe Hc Hs
        icases array_acc limbs_ptr (done.length : Int) (DFrac.own 1) 4 (paddedLimbs done) (W64 0)
            (by omega) (by simpa using paddedLimbs_next done hsmall) $$ Hlimbs with ⟨Hword, Hrestore⟩
        simp only [hindex]
        wp_store
        wp_auto
        ihave Hlimbs := Hrestore $$ %item.value Hword
        isimp only [Int.toNat_natCast, paddedLimbs_step done item hsmall] at Hlimbs
        ieval (change (c ↦ {v with pos' := pos + W64 item.bytes.length, depth' := v.depth' + W64 1} : IProp GF) ⊢ _) at Hc
        dsimp only at Hnext
        have hbyteslen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by simp [unsignedBytes]
        have hinc : W64 (done.length + passed.length) + W64 1 = W64 ((done ++ [item]).length + passed.length) := by
          simp only [List.length_append, List.length_singleton]; word
        wp_for_post
        isimp only [hinc] at Hrange
        iframe
        iexists (done ++ [item]), todo, passed, pending, (pos + W64 item.bytes.length), (W64 (done.length + passed.length))
        iframe
        ipureintro
        refine ⟨?_, Hextras, ?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · intro _; exact hpassed
        · rw [Hnext.1]; omega
        · rw [Hnext.1, hbyteslen]; omega
  · have hcond : ¬ uint.Z (W64 (done.length + passed.length)) < uint.Z (W64 (4 + extras.length)) := by word
    simp only [hcond, hf, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hpending : pending = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
    have hpassedall : passed = extras := by simpa [hpending] using Hextras.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) := by
      rw [hdoneall, hpassedall] at Hposition
      word
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hdoneall, hposfinal, hrestoredepth] at Hc Hdst
    iapply HΦ
    iframe Hc Hdst Hs

/-- Non-inline-four heads route through the proved extended slow loop. -/
theorem wp_limbs_nonfast_extended (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload (W64 (4 + extras.length))) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hcount : items.length = 4) (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hnd : ¬ sint.Z (W64 32) ≤ sint.Z v.depth' := by change ¬ 32 ≤ sint.Z v.depth'; omega
  wp_auto
  simp only [hnd, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs
      ⟨hp.1, hp.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_auto
  by_cases hlong : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - v.pos')
  · simp only [hlong, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos'); word⟩)]
    wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
        ((head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    simp only [hne, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply wp_limbsSlow_extended c dst old v xs payload rem head items extras dq d hd hdepth enc hm hcount valid hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    wp_end
  · simp only [hlong, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply wp_limbsSlow_extended c dst old v xs payload rem head items extras dq d hd hdepth enc hm hcount valid hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    wp_end

/-- Public Limbs handles four words followed by any number of valid ignored values. -/
theorem wp_limbs_extended (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload (W64 (4 + extras.length))) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)} ∗
       v.buf' ↦*{dq} xs }} := by
  by_cases he : extras = []
  · subst extras
    simpa only [List.flatten_nil, List.length_nil, List.append_nil, Int.natCast_zero, Int.add_zero] using
      wp_limbs_small c dst old v xs payload rem head items dq
        (by rw [hcount]; exact enc) hm (by omega) hp (by omega)
        (by simpa only [List.flatten_nil, List.append_nil] using hbytes)
  · iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hu := unsignedBytes_lower_bound items
    have hs := skipValid_items_bound (d + 1) extras valid
    have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten) rem v.pos' v.buf'.len hp Hlen.1 hbytes
    have hfit : (4 : Int) + extras.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
      omega
    have hex : extras.length ≠ 0 := fun h => he (List.eq_nil_of_length_eq_zero h)
    have hne : head ≠ W8 132 := by
      intro hh
      subst head
      obtain ⟨_, hv⟩ := headArgument_inline_inv (W8 132) payload (W64 (4 + extras.length)) enc (by decide)
      change W64 (4 + extras.length) = W64 4 at hv
      word
    wp_apply wp_limbs_nonfast_extended c dst old v xs payload rem head items extras dq d hd hdepth enc hm hne hcount valid hp hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs

/-- The typed reader publishes the four retained limbs and consumes all extra values. -/
theorem wp_readFelt_extended (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (extras : List (List w8))
    (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (enc : HeadArgument head payload (W64 (4 + extras.length))) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length = 4) (valid : ∀ item ∈ extras, SkipValid (d + 1) item)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ extras.flatten ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)} ∗
       v.buf' ↦*{dq} xs ∗ ⌜p ≠ null ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)) =
           sint.Z v.pos' + (1 + payload.length + (unsignedBytes items).length + extras.flatten.length) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (unsignedBytes items).length + extras.flatten.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ unsignedBytes items ++ extras.flatten) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ extras.flatten ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_extended c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head items extras dq d hd hdepth
    enc hm hcount valid hp hbytes $$ [«$r0» Hc Hs] with ⟨Hv, Hc, Hs⟩
  · iframe «$r0» Hc Hs
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hv
  iapply HΦ
  iframe Hdst Hv Hc Hs
  ipureintro
  exact ⟨Hnn, by simpa only [Int.add_comm (payload.length : Int) 1] using Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedBytes_lower_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_extended

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_nonfast_extended

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_extended

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_extended
