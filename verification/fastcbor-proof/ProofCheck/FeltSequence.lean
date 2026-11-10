import ProofCheck.FeltWide

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- An unsigned CBOR item, separating its mathematical value from the chosen wire width. -/
structure UnsignedWire where
  head : w8
  payload : List w8
  value : w64
  major : head &&& W8 224 = W8 0
  argument : HeadArgument head payload value

def UnsignedWire.bytes (x : UnsignedWire) : List w8 := x.head :: x.payload

def unsignedBytes (xs : List UnsignedWire) : List w8 := xs.flatMap UnsignedWire.bytes

def paddedLimbs (xs : List UnsignedWire) : GoArray w64 4 :=
  array.mk 4 (xs.map UnsignedWire.value ++ List.replicate (4 - xs.length) (W64 0))

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Splitting a known unread prefix identifies the next unread suffix. -/
theorem input_suffix_after (xs front rest : List w8) (p n : Nat)
    (h : subslice p n xs = front ++ rest) :
    subslice (p + front.length) n xs = rest := by
  rw [← drop_subslice, h, List.drop_left]

/-- A nonempty unread prefix supplies its first byte at the current offset. -/
theorem input_suffix_head (xs tail : List w8) (p n : Nat) (head : w8)
    (h : subslice p n xs = head :: tail) : xs[p]? = some head := by
  have hi : (subslice p n xs)[0]? = some head := by rw [h]; rfl
  have hb := subslice_lookup_bound' hi
  rw [subslice_lookup _ _ _ xs hb, Nat.add_zero] at hi
  exact hi

/-- Unsigned decoding consumes exactly one encoded item and preserves the remaining suffix. -/
theorem wp_uint64_wire (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (item : UnsignedWire) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item.bytes ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ RET (PairV #item.value #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 item.bytes.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.bytes.length) = sint.Z v.pos' + item.bytes.length ∧
        sint.Z (v.pos' + W64 item.bytes.length) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 item.bytes.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width item.head item.payload item.value item.argument
  have hcount := congrArg List.length hbytes
  have hbound : sint.Z v.pos' + 1 + item.payload.length ≤ sint.Z v.buf'.len := by
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), UnsignedWire.bytes, List.length_append, List.length_cons] at hcount
    simp only [sint.nat, sint.Z] at Hlen hcount hp ⊢
    omega
  have hi := input_suffix_head xs (item.payload ++ rest) (sint.nat v.pos')
    (sint.nat v.buf'.len) item.head hbytes
  have ha : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = item.payload ++ rest := by
    have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hp1]
    exact input_suffix_after xs [item.head] (item.payload ++ rest) _ _ hbytes
  wp_apply wp_uint64_encoded c v xs item.payload rest item.head item.value dq item.argument item.major
    hp.1 hbound hi ha $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hsize : (item.bytes.length : Int) = 1 + item.payload.length := by simp [UnsignedWire.bytes]; omega
  have hposition : sint.Z (v.pos' + W64 item.bytes.length) = sint.Z v.pos' + item.bytes.length := by word
  have hnext : subslice (sint.nat (v.pos' + W64 item.bytes.length)) (sint.nat v.buf'.len) xs = rest := by
    have hnat : sint.nat (v.pos' + W64 item.bytes.length) = sint.nat v.pos' + item.bytes.length := by word
    rw [hnat]
    exact input_suffix_after xs item.bytes rest _ _ hbytes
  iapply HΦ
  rw [hsize]
  iframe Hc Hs
  ipureintro
  constructor
  · simpa only [hsize] using hposition
  constructor
  · word
  · simpa only [hsize] using hnext

/-- The next unfilled limb is zero. -/
theorem paddedLimbs_next (done : List UnsignedWire) (h : done.length < 4) :
    (paddedLimbs done).arr[done.length]? = some (W64 0) := by
  have hpad : 4 - done.length = (3 - done.length) + 1 := by omega
  simp [paddedLimbs, List.getElem?_append_right, hpad, List.replicate_succ]

/-- One successful limb read extends the decoded prefix and keeps the rest zero. -/
theorem paddedLimbs_step (done : List UnsignedWire) (item : UnsignedWire) (h : done.length < 4) :
    array.mk 4 ((paddedLimbs done).arr.set done.length item.value) = paddedLimbs (done ++ [item]) := by
  have hpad : 4 - done.length = (3 - done.length) + 1 := by omega
  have hpad' : 4 - (done.length + 1) = 3 - done.length := by omega
  simp [paddedLimbs, List.set_append_right, hpad, hpad', List.replicate_succ, List.append_assoc]

/-- The slow felt reader handles zero through four arbitrary-width unsigned limbs and zero-fills the rest. -/
theorem wp_limbsSlow_small (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (dq : DFrac)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hcount : items.length ≤ 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"limbsSlow")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload (W64 items.length) enc
  have htotal : sint.Z v.pos' + 1 + payload.length + (unsignedBytes items).length ≤ sint.Z v.buf'.len := by
    have hlength := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
      List.length_append, List.length_cons] at hlength
    simp only [sint.nat, sint.Z] at Hlen hlength hp ⊢
    omega
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by omega
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (unsignedBytes items ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.singleton_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  rw [← hm]
  wp_apply wp_expect_encoded c v xs payload (unsignedBytes items ++ rem) head (W64 items.length) dq
    enc hp.1 hbound hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' hdepth
    $$ [Hc] with Hc
  · iframe Hc
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      unsignedBytes items ++ rem := by
    have hn : sint.nat (v.pos' + W64 (1 + payload.length)) = sint.nat v.pos' + (head :: payload).length := by
      simp only [List.length_cons]; word
    rw [hn]
    exact input_suffix_after xs (head :: payload) _ _ _ (by simpa only [List.append_assoc] using hbytes)
  ihave HI : (∃ (done todo : List UnsignedWire) (pos oldi : w64),
      "Hrange" ∷ «$range_i_ptr» ↦ W64 done.length ∗
      "Hi" ∷ i_ptr ↦ oldi ∗
      "Hlimbs" ∷ limbs_ptr ↦ paddedLimbs done ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (unsignedBytes done).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = unsignedBytes todo ++ rem⌝ : IProp GF)
      $$ [«$range_i» i limbs Hc]
  · iexists [], items, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨rfl, ⟨by word, by word⟩, by simpa [unsignedBytes] using hstart, hremaining⟩
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ 4 := by omega
  by_cases hmore : done.length < items.length
  · have hcond : uint.Z (W64 done.length) < uint.Z (W64 items.length) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hnotfull : ¬ uint.Z (W64 4) ≤ uint.Z (W64 done.length) := by word
    simp only [hnotfull, _root_.decide_false]
    wp_auto
    cases todo with
    | nil => simp only [List.length_nil, Nat.add_zero] at hlength; omega
    | cons item todo =>
      have hsmall : done.length < 4 := by omega
      have hindex : sint.Z (W64 done.length) = done.length := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hindex]; exact ⟨by omega, by omega⟩))]
      wp_auto
      wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
          (unsignedBytes todo ++ rem) item dq Hpos
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
      have hbyteslen : (unsignedBytes (done ++ [item])).length = (unsignedBytes done).length + item.bytes.length := by
        simp [unsignedBytes]
      have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by simp only [List.length_append, List.length_singleton]; word
      wp_for_post
      isimp only [hinc] at Hrange
      iframe
      iexists (done ++ [item]), todo, (pos + W64 item.bytes.length), (W64 done.length)
      iframe
      ipureintro
      refine ⟨?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · rw [Hnext.1]; omega
      · rw [Hnext.1, hbyteslen]; omega
  · have hcond : ¬ uint.Z (W64 done.length) < uint.Z (W64 items.length) := by word
    simp only [hcond, hf, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false]
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    have hdoneall : done = items := by simpa [htodo] using Hsplit.symm
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (unsignedBytes items).length) := by
      rw [hdoneall] at Hposition
      word
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hdoneall, hposfinal, hrestoredepth] at Hc Hdst
    iapply HΦ
    iframe Hc Hdst Hs

/-- The mixed-width shortcut declines every array head other than the inline four-element head. -/
theorem wp_shortLimbs_otherHead (dst : Loc) (data : GoSlice) (xs : List w8) (head : w8) (dq : DFrac)
    (hi : xs[0]? = some head) (hne : head ≠ W8 132) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ data ↦*{dq} xs }}
      (App (App (Val (@! cursor.shortLimbs)) (Val #data)) (Val #dst))
    {{ RET #(W64 0); data ↦*{dq} xs }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hxs : 0 < xs.length := (List.getElem?_eq_some_iff.mp hi).choose
  have hlen : 0 < sint.Z data.len := by simp only [sint.nat, sint.Z] at Hlen ⊢; omega
  have hn : data.len ≠ W64 0 := by intro h; rw [h] at hlen; contradiction
  wp_auto
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by exact hlen⟩)]
  wp_apply wp_load_slice_index data _ xs dq head (by decide) $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hne, _root_.decide_false]
  wp_auto
  wp_end

/-- Every short array and every extended-width four-element head reaches the proved slow loop. -/
theorem wp_limbs_nonfast_small (c dst : Loc) (old : GoArray w64 4) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (dq : DFrac)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hcount : items.length ≤ 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Limbs")) (Val #dst))
    {{ RET #interface.nil; dst ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)} ∗
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
        ((head :: payload) ++ unsignedBytes items ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    simp only [hne, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ unsignedBytes items ++ rem) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply wp_limbsSlow_small c dst old v xs payload rem head items dq enc hm hcount hp hdepth hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    wp_end
  · simp only [hlong, _root_.decide_false]
    wp_auto
    wp_apply wp_shortLimbs_otherHead dst (slice.slice v.buf' w8 v.pos' v.buf'.len)
      ((head :: payload) ++ unsignedBytes items ++ rem) head dq rfl hne $$ [Hbuf] with Hbuf
    · iframe Hbuf
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply wp_limbsSlow_small c dst old v xs payload rem head items dq enc hm hcount hp hdepth hbytes
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    wp_end

/-- Short arrays and extended four-element heads produce a fresh, owned typed felt. -/
theorem wp_readFelt_nonfast_small (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (items : List UnsignedWire) (dq : DFrac)
    (enc : HeadArgument head payload (W64 items.length)) (hm : head &&& W8 224 = W8 128)
    (hne : head ≠ W8 132) (hcount : items.length ≤ 4)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hdepth : sint.Z v.depth' < 32)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ unsignedBytes items ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ p ↦ paddedLimbs items ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (unsignedBytes items).length)} ∗
       v.buf' ↦*{dq} xs ∗ ⌜p ≠ null⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ unsignedBytes items ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_limbs_nonfast_small c «$r0_ptr» (zero_val (GoArray w64 4)) v xs payload rem head items dq
    enc hm hne hcount hp hdepth hbytes $$ [«$r0» Hc Hs] with ⟨Hv, Hc, Hs⟩
  · iframe «$r0» Hc Hs
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hv
  iapply HΦ
  iframe Hdst Hv Hc Hs
  ipureintro; exact Hnn

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.input_suffix_after
#print axioms Perennial.github_com.NethermindEth.juno.core.input_suffix_head
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_wire

#print axioms Perennial.github_com.NethermindEth.juno.core.paddedLimbs_next

#print axioms Perennial.github_com.NethermindEth.juno.core.paddedLimbs_step

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbsSlow_small

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_shortLimbs_otherHead

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbs_nonfast_small

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_nonfast_small
