import ProofCheck.FieldReject

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Head's inline optimization preserves a proved slow-path error. This is a
    composition lemma; the caller supplies the proof of the actual slow helper. -/
theorem wp_head_reject (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31))
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
          #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  have hsmall : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := by change ¬ uint.Z (head &&& W8 31) < 24; omega
  simp only [hsmall, _root_.decide_false]
  wp_auto
  wp_apply hslow $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- keySlow propagates a head failure without touching the cursor or input. -/
theorem wp_keySlow_head_reject (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hhead :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
          #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  wp_auto
  wp_apply hhead $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- A head outside the short ASCII-key path reaches keySlow. The second premise
    also covers the incomplete two-byte text head 0x78 with no argument byte. -/
theorem wp_Key_unready (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hsmall : 24 ≤ uint.Z (head - W8 96))
    (hsecond : head - W8 96 ≠ W8 24 ∨ rest = [])
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"keySlow")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #slice.nil #false)
          #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false)
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hheadbound : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [List.length_cons, List.length_nil] at Hnext
    omega
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
    (head :: rest) dq head (by decide) $$ [Hbuf] with Hbuf
  · iframe Hbuf; ipureintro; rfl
  have hguard : ¬ (W64 (uint.Z (head - W8 96)) = W64 24 ∧
      sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos')) := by
    rcases hsecond with hn | he
    · intro ⟨heq, _⟩; apply hn; word
    · have hcount := congrArg List.length hbytes
      simp only [he, List.length_cons, List.length_nil,
        subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega)] at hcount
      intro ⟨_, hgt⟩
      have hdiff : sint.Z (v.buf'.len - v.pos') = 1 := by
        simp only [sint.nat] at hcount
        word
      rw [hdiff] at hgt
      contradiction
  by_cases heq : W64 (uint.Z (head - W8 96)) = W64 24
  · have hnotlen : ¬ sint.Z (W64 1) < sint.Z (v.buf'.len - v.pos') := fun h => hguard ⟨heq, h⟩
    simp only [heq, _root_.decide_true]
    wp_auto
    simp only [hnotlen, _root_.decide_false]
    wp_auto
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply hslow $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end
  · simp only [heq, _root_.decide_false]
    wp_auto
    have hnotshort : ¬ sint.Z (W64 (uint.Z (head - W8 96))) < sint.Z (W64 24) := by word
    simp only [hnotshort, _root_.decide_false]
    wp_auto
    ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
    · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
      rw [hbytes]
      iframe Hpre Hbuf Hpost
    wp_apply hslow $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end

/-- Compose the actual head and key wrappers, preserving an owned slow-head
    diagnostic. Closed contracts below supply the actual head rejection proofs. -/
theorem wp_Key_head_reject (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31))
    (hsecond : head - W8 96 ≠ W8 24 ∨ rest = [])
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  have hsmall : 24 ≤ uint.Z (head - W8 96) := by
    by_cases hm : head &&& W8 224 = W8 96
    · rw [← hm, byte_major_remainder]; exact hinfo
    · have hn := byte_nontext_key_slow head hm; omega
  exact wp_Key_unready c v xs rest head dq R E ⟨hp, by omega⟩ hbytes hsmall hsecond
    (wp_keySlow_head_reject c v xs dq R E
      (wp_head_reject c v xs head dq R E hp hlt hi hinfo hslow))

/-- Alternate lookup propagates a proved public-Key failure, without accessing
    field state. The public NextField wrapper separately accounts for left--. -/
theorem wp_otherField_Key_reject (c st : Loc) (v : cursor.Cursor) (xs : List w8)
    (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hkey :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  wp_auto
  wp_apply hkey $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- Public Key propagates the indefinite head diagnostic at the original offset. -/
theorem wp_Key_head_indefinite (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : 64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil }} := by
  have hsecond : head - W8 96 ≠ W8 24 ∨ rest = [] := by
    left; intro h; have hh : head = W8 120 := by word
    rw [hh] at hinfo
    change 27 < 24 at hinfo
    omega
  exact wp_Key_head_reject c v xs rest head dq
    iprop(globalAddr cursor.ErrUnsupported ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"indefinite-length item") slice.nil))
    hp hlt hi hbytes (by omega) hsecond
    (wp_headSlow_indefinite c v xs head dq kind hp hlt hi hinfo hindef hmajor)

/-- Public Key propagates the break head diagnostic at the original offset. -/
theorem wp_Key_head_break (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31)) (hindef : head &&& W8 31 = W8 31)
    (hmajor : head &&& W8 224 = W8 224) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil }} := by
  have hsecond : head - W8 96 ≠ W8 24 ∨ rest = [] := by
    left; intro h; have hh : head = W8 120 := by word
    rw [hh] at hinfo
    change 27 < 24 at hinfo
    omega
  exact wp_Key_head_reject c v xs rest head dq
    iprop(globalAddr cursor.ErrSyntax ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"break code outside an indefinite-length item") slice.nil))
    hp hlt hi hbytes (by omega) hsecond
    (wp_headSlow_break c v xs head dq kind hp hlt hi hinfo hindef hmajor)

/-- Public Key propagates the invalid head diagnostic at the original offset. -/
theorem wp_Key_head_invalid (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 27 < uint.Z (head &&& W8 31))
    (hindef : ¬ (head &&& W8 31 = W8 31 ∧
      64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160))
    (hbreak : ¬ (head &&& W8 31 = W8 31 ∧ head &&& W8 224 = W8 224)) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)] }} := by
  have hsecond : head - W8 96 ≠ W8 24 ∨ rest = [] := by
    left; intro h; have hh : head = W8 120 := by word
    rw [hh] at hinfo
    change 27 < 24 at hinfo
    omega
  exact wp_Key_head_reject c v xs rest head dq
    iprop(globalAddr cursor.ErrSyntax ↦□ kind)
    (fun err => iprop(∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind v.pos' (go!"invalid additional information %d for %s") args ∗
       args ↦* [interface.mkOk go.byte #(head &&& W8 31),
         interface.mkOk go.string #(headDescription head)]))
    hp hlt hi hbytes (by omega) hsecond
    (wp_headSlow_invalid c v xs head dq kind hp hlt hi hinfo hindef hbreak)

/-- Public Key propagates the truncated head diagnostic at the original offset. -/
theorem wp_Key_head_truncated (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hsecond : head - W8 96 ≠ W8 24 ∨ rest = [] := by
    by_cases heq : head - W8 96 = W8 24
    · right
      have hh : head = W8 120 := by word
      have hs : size = W64 1 := by simpa [hh] using hsize.symm
      have hcount := congrArg List.length hbytes
      simp only [List.length_cons,
        subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega)] at hcount
      have hzero : rest.length = 0 := by
        simp only [sint.nat] at hcount Hlen
        rw [hs] at htrunc
        word
      exact List.eq_nil_of_length_eq_zero hzero
    · exact Or.inl heq
  wp_apply wp_Key_head_reject c v xs rest head dq
    iprop(globalAddr cursor.ErrTruncated ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil))
    hp hlt hi hbytes hinfo.1 hsecond
    (wp_headSlow_truncated c v xs head size dq kind hp hlt hi hinfo hsize htrunc) $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_keySlow_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_unready
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_Key_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_head_indefinite
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_head_break
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_head_invalid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_head_truncated
