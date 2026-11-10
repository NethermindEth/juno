import ProofCheck.FieldMatch

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

/-- Complete unsigned and negative integer keys within int64 are consumed and reported as non-text. -/
theorem wp_keySlow_integer (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : uint.Z argument < 2 ^ 63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ RET (PairV (PairV #slice.nil #false) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload argument enc
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_auto
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  rcases hm with hm | hm
  all_goals
    simp only [hm]
    wp_auto
    have hgood : ¬ uint.Z (W64 9223372036854775807) < uint.Z argument := by word
    simp only [hgood, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hc Hs
    ipureintro
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1,
      Int.add_assoc] using Hnext

/-- Both integer major types reject arguments outside int64 and restore the original cursor. -/
theorem wp_keySlow_integer_overflow (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : 2 ^ 63 ≤ uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"integer key does not fit int64") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hw := headArgument_width head payload argument enc
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_auto
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  rcases hm with hm | hm
  all_goals
    simp only [hm]
    wp_auto
    have hbad : uint.Z (W64 9223372036854775807) < uint.Z argument := by word
    simp only [hbad, _root_.decide_true]
    wp_auto
    wp_apply wp_fail c v kind (go!"integer key does not fit int64") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hc
    wp_end

/-- Other major types cannot be mistaken for either short-text head by Key's byte subtraction. -/
theorem byte_nontext_key_slow (head : w8) (hm : head &&& W8 224 ≠ W8 96) :
    32 ≤ uint.Z (head - W8 96) := by
  have h : ∀ head : w8, head &&& W8 224 ≠ W8 96 → 32 ≤ uint.Z (head - W8 96) := by decide
  exact h head hm

/-- The public Key dispatch reaches the integer contract without consuming a text fast path. -/
theorem wp_Key_integer (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : uint.Z argument < 2 ^ 63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ RET (PairV (PairV #slice.nil #false) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hntext : head &&& W8 224 ≠ W8 96 := by
    rcases hm with h | h <;> rw [h] <;> decide
  have hslow := byte_nontext_key_slow head hntext
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  have hlenpos : sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlenpos, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlenpos⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
    ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by word
  simp only [hnot24, _root_.decide_false]
  wp_auto
  have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by word
  simp only [hnotshort, _root_.decide_false]
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_keySlow_integer c v xs payload rem head argument dq enc hm hrange hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro
  exact Hnext

/-- The public Key dispatch reaches the integer contract without consuming a text fast path. -/
theorem wp_Key_integer_overflow (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : 2 ^ 63 ≤ uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"integer key does not fit int64") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    omega
  have hntext : head &&& W8 224 ≠ W8 96 := by
    rcases hm with h | h <;> rw [h] <;> decide
  have hslow := byte_nontext_key_slow head hntext
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  have hlenpos : sint.Z (W64 0) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hlenpos, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hlenpos⟩)]
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_load_slice_index (slice.slice v.buf' w8 v.pos' v.buf'.len) _
    ((head :: payload) ++ rem) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hnot24 : W64 (uint.Z (head - W8 96)) ≠ W64 24 := by word
  simp only [hnot24, _root_.decide_false]
  wp_auto
  have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by word
  simp only [hnotshort, _root_.decide_false]
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbuf Hpost
  wp_apply wp_keySlow_integer_overflow c v xs payload rem head argument dq kind enc hm hrange hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end

/-- Integer-key classification composes through the actual alternate-field path. -/
theorem wp_otherField_integer (c st : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : uint.Z argument < 2 ^ 63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ RET (PairV #(W64 (-2)) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_integer c v xs payload rem head argument dq enc hm hrange hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro
  exact Hnext

/-- Integer-key classification composes through the actual alternate-field path. -/
theorem wp_otherField_integer_overflow (c st : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (hrange : 2 ^ 63 ≤ uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"integer key does not fit int64") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_Key_integer_overflow c v xs payload rem head argument dq kind enc hm hrange hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hc Hs Hkind
  wp_end


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_integer
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_integer_overflow

#print axioms Perennial.github_com.NethermindEth.juno.core.byte_nontext_key_slow

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_integer

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_integer_overflow

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_integer

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_integer_overflow
