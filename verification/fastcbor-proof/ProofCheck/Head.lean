import ProofCheck.Felt
import Perennial.Proof.encoding.binary.Results

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

/-- The byte's low five bits are the remainder after removing its major-type bits.
This finite identity is reduced by the Lean kernel, without native evaluation axioms. -/
theorem byte_major_remainder (b : w8) : b - (b &&& W8 224) = b &&& W8 31 := by
  have h : ∀ b : w8, b - (b &&& W8 224) = b &&& W8 31 := by decide
  exact h b

/-- The actual standard-library two-byte decoder, with an arbitrary trailing slice. -/
theorem wp_binaryBigEndian_u16 (data : GoSlice) (a b : w8) (rem : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} (a :: b :: rem) }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint16")) (Val #data))
    {{ RET #(W16 (uint.Z b) ||| (W16 (uint.Z a) <<< W64 8)); data ↦*{dq} (a :: b :: rem) }} := by
  wp_start as Hb
  ihave #Hpkg : isPkgInit (PROP := IProp GF) pkg_id.encoding.binary $$ []
  · iPkgInit
  ihave #Hinit := encoding.binary.isInit_access $$ Hpkg
  icases Hinit with ⟨-, #Hbe⟩
  ihave %Hnn := typedPointsto_not_null _ _ _ $$ Hbe
  wp_apply (encoding.binary.wp_load_struct_nofields (V := encoding.binary.bigEndian) _ Hnn) as -
  wp_method_call
  wp_call
  wp_auto
  ihave %Hlen := ownSlice_len _ _ _ $$ Hb
  have hb : sint.Z (W64 1) < sint.Z data.len := by
    change (1 : Int) < sint.Z data.len
    simp only [sint.nat, sint.Z, List.length_cons] at Hlen ⊢
    have := (List.length rem).zero_le
    omega
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hb⟩)]
  wp_apply wp_load_slice_index data _ (a :: b :: rem) dq b (by decide) $$ [Hb] with Hb
  · iframe Hb; ipureintro; rfl
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hb⟩)]
  wp_apply wp_load_slice_index data _ (a :: b :: rem) dq b (by decide) $$ [Hb] with Hb
  · iframe Hb; ipureintro; rfl
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by word⟩)]
  wp_apply wp_load_slice_index data _ (a :: b :: rem) dq a (by decide) $$ [Hb] with Hb
  · iframe Hb; ipureintro; rfl
  wp_end

/-- A one-byte argument at any valid cursor offset; the major type is unrestricted. -/
theorem wp_headSlow_byte (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 2 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hb : xs[(sint.Z v.pos' + 1).toNat]? = some b)
    (hinfo : head &&& W8 31 = W8 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #(W64 (uint.Z b))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 2} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by word
  wp_auto
  simp only [hlt, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hinfo]
  wp_auto
  have hleft : ¬ sint.Z (v.buf'.len - (v.pos' + W64 1)) < sint.Z (W64 1) := by word
  have hsize : W64 1 <<< W64 (uint.Z (W8 24 - W8 24)) = W64 1 := rfl
  isimp only [hsize] at size
  simp only [hsize, hleft, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by rw [hs]; omega, by rw [hs]; omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq b (by rw [hs]; omega) $$ [Hs] with Hs
  · iframe Hs; ipureintro; simpa only [hs] using hb
  have hadd : v.pos' + W64 1 + W64 1 = v.pos' + W64 2 := by word
  simp only [hadd]
  wp_end

/-- A 2-byte big-endian argument, with arbitrary prefix, suffix and cursor state. -/
theorem wp_headSlow_u16 (c : Loc) (v : cursor.Cursor) (xs rem : List w8) (a b : w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 3 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = a :: b :: rem)
    (hinfo : head &&& W8 31 = W8 25) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #(W64 (uint.Z (W16 (uint.Z b) ||| (W16 (uint.Z a) <<< W64 8))))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 3} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlt : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by word
  wp_auto
  simp only [hlt, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hinfo]
  wp_auto
  have hleft : ¬ sint.Z (v.buf'.len - (v.pos' + W64 1)) < sint.Z (W64 2) := by word
  have hsize : W64 1 <<< W64 (uint.Z (W8 25 - W8 24)) = W64 2 := rfl
  isimp only [hsize] at size
  simp only [hsize, hleft, _root_.decide_false]
  wp_auto
  have hslice : 0 ≤ sint.Z (v.pos' + W64 1) ∧
      sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap := by
    rw [hs]; exact ⟨by omega, by omega, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslice)]
  icases (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply wp_binaryBigEndian_u16 (slice.slice v.buf' w8 (v.pos' + W64 1) v.buf'.len)
      a b rem dq $$ [Hbytes] with Hbytes
  · iframe Hbytes
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbytes Hpost]
  · iapply (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbytes Hpost
  have hadd : v.pos' + W64 1 + W64 2 = v.pos' + W64 3 := by word
  simp only [hadd]
  wp_end

/-- A 4-byte big-endian argument, with arbitrary prefix, suffix and cursor state. -/
theorem wp_headSlow_u32 (c : Loc) (v : cursor.Cursor) (xs bs rem : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 5 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = bs ++ rem)
    (hlen : bs.length = 4) (hinfo : head &&& W8 31 = W8 26) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #(W64 (uint.Z (leToU32 bs.reverse)))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 5} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlt : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by word
  wp_auto
  simp only [hlt, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hinfo]
  wp_auto
  have hleft : ¬ sint.Z (v.buf'.len - (v.pos' + W64 1)) < sint.Z (W64 4) := by word
  have hsize : W64 1 <<< W64 (uint.Z (W8 26 - W8 24)) = W64 4 := rfl
  isimp only [hsize] at size
  simp only [hsize, hleft, _root_.decide_false]
  wp_auto
  have hslice : 0 ≤ sint.Z (v.pos' + W64 1) ∧
      sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap := by
    rw [hs]; exact ⟨by omega, by omega, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslice)]
  icases (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_BigEndian_Uint32 (slice.slice v.buf' w8 (v.pos' + W64 1) v.buf'.len)
      bs dq rem hlen $$ [Hbytes] with Hbytes
  · iframe Hbytes
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbytes Hpost]
  · iapply (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbytes Hpost
  have hadd : v.pos' + W64 1 + W64 4 = v.pos' + W64 5 := by word
  simp only [hadd]
  wp_end

/-- A 8-byte big-endian argument, with arbitrary prefix, suffix and cursor state. -/
theorem wp_headSlow_u64 (c : Loc) (v : cursor.Cursor) (xs bs rem : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 9 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = bs ++ rem)
    (hlen : bs.length = 8) (hinfo : head &&& W8 31 = W8 27) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #(leToU64 bs.reverse)) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 9} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlt : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by word
  wp_auto
  simp only [hlt, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hinfo]
  wp_auto
  have hleft : ¬ sint.Z (v.buf'.len - (v.pos' + W64 1)) < sint.Z (W64 8) := by word
  have hsize : W64 1 <<< W64 (uint.Z (W8 27 - W8 24)) = W64 8 := rfl
  isimp only [hsize] at size
  simp only [hsize, hleft, _root_.decide_false]
  wp_auto
  have hslice : 0 ≤ sint.Z (v.pos' + W64 1) ∧
      sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap := by
    rw [hs]; exact ⟨by omega, by omega, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hslice)]
  icases (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_BigEndian_Uint64 (slice.slice v.buf' w8 (v.pos' + W64 1) v.buf'.len)
      bs dq rem hlen $$ [Hbytes] with Hbytes
  · iframe Hbytes
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbytes Hpost]
  · iapply (ownSlice_slice (v.pos' + W64 1) v.buf'.len v.buf' dq xs
      ⟨hslice.1, hslice.2.1, Int.le_refl _⟩).2
    rw [hbytes]
    iframe Hpre Hbytes Hpost
  have hadd : v.pos' + W64 1 + W64 8 = v.pos' + W64 9 := by word
  simp only [hadd]
  wp_end

/-- The definite CBOR head argument encodings from RFC 8949 section 3.1.
The major type is orthogonal. Wider-than-preferred representations are allowed.
This relation does not assert validity of the item's payload or tags. -/
inductive HeadArgument : w8 → List w8 → w64 → Prop where
  | inline (head : w8) (h : uint.Z (head &&& W8 31) < 24) :
      HeadArgument head [] (W64 (uint.Z (head &&& W8 31)))
  | byte (head b : w8) (h : head &&& W8 31 = W8 24) :
      HeadArgument head [b] (W64 (uint.Z b))
  | u16 (head a b : w8) (h : head &&& W8 31 = W8 25) :
      HeadArgument head [a, b] (W64 (uint.Z (W16 (uint.Z b) ||| (W16 (uint.Z a) <<< W64 8))))
  | u32 (head : w8) (bs : List w8) (hlen : bs.length = 4) (h : head &&& W8 31 = W8 26) :
      HeadArgument head bs (W64 (uint.Z (leToU32 bs.reverse)))
  | u64 (head : w8) (bs : List w8) (hlen : bs.length = 8) (h : head &&& W8 31 = W8 27) :
      HeadArgument head bs (leToU64 bs.reverse)

/-- Inline heads for every major type and arbitrary cursor state. -/
theorem wp_headSlow_inline_at (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : uint.Z (head &&& W8 31) < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #(W64 (uint.Z (head &&& W8 31)))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  wp_auto
  simp only [hlt, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, by omega⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  have hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24) := hinfo
  simp only [hsmall, _root_.decide_true]
  wp_auto
  wp_end

/-- The slow head parser implements every definite head argument encoding.
The bounds are in mathematical integers, so the position update cannot overflow. -/
theorem wp_headSlow_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #argument) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  cases enc with
  | inline h =>
    simpa using wp_headSlow_inline_at c v xs head dq hp (by simpa using hbound) hi h
  | byte b h =>
    iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by
      simp only [List.length_singleton] at hbound
      word
    have hb : xs[(sint.Z v.pos' + 1).toNat]? = some b := by
      have he := congrArg (fun ys : List w8 => ys[0]?) hbytes
      simp only [subslice, List.getElem?_take, List.getElem?_drop] at he
      simp only [sint.nat, hs, List.singleton_append, List.getElem?_cons_zero] at he
      simp only [sint.nat, sint.Z] at Hlen
      split at he <;> simp_all
    wp_apply wp_headSlow_byte c v xs head b dq hp (by simp only [List.length_cons, List.length_nil] at hbound; omega) hi hb h
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end
  | u16 a b h =>
    simpa using wp_headSlow_u16 c v xs rem a b head dq hp (by simp only [List.length_cons, List.length_nil] at hbound; omega) hi hbytes h
  | u32 bs hlen h =>
    simpa [hlen] using wp_headSlow_u32 c v xs payload rem head dq hp
      (by rw [hlen] at hbound; omega) hi hbytes hlen h
  | u64 bs hlen h =>
    simpa [hlen] using wp_headSlow_u64 c v xs payload rem head dq hp
      (by rw [hlen] at hbound; omega) hi hbytes hlen h

theorem headArgument_inline_inv (head : w8) (payload : List w8) (argument : w64)
    (enc : HeadArgument head payload argument) (hsmall : uint.Z (head &&& W8 31) < 24) :
    payload = [] ∧ argument = W64 (uint.Z (head &&& W8 31)) := by
  cases enc with
  | inline h => exact ⟨rfl, rfl⟩
  | byte b h => rw [h] at hsmall; contradiction
  | u16 a b h => rw [h] at hsmall; contradiction
  | u32 bs hlen h => rw [h] at hsmall; contradiction
  | u64 bs hlen h => rw [h] at hsmall; contradiction

/-- General inline `expect`, for any matching major type. -/
theorem wp_expect_inline_major (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : uint.Z (head &&& W8 31) < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(head &&& W8 224)))
    {{ RET (PairV #(W64 (uint.Z (head &&& W8 31))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hsub : head - (head &&& W8 224) = head &&& W8 31 := byte_major_remainder head
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  have hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24) := hinfo
  simp only [hsub, hsmall, _root_.decide_true]
  wp_auto
  wp_end

/-- The slow expectation wrapper preserves the exact argument and consumption. -/
theorem wp_expectSlow_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #(head &&& W8 224)))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_headSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- Both expectation paths implement the same definite-head relation. -/
theorem wp_expect_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(head &&& W8 224)))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  by_cases hsmall : uint.Z (head &&& W8 31) < 24
  · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
    simpa using wp_expect_inline_major c v xs head dq hp (by simpa using hbound) hi hsmall
  · wp_start as ⟨Hc, Hs⟩
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    have hsub : head - (head &&& W8 224) = head &&& W8 31 := byte_major_remainder head
    wp_auto
    simp only [hlt, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    have hlarge : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := hsmall
    simp only [hsub, hlarge, _root_.decide_false]
    wp_auto
    wp_apply wp_expectSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end

/-- Every unsigned CBOR width is supported by Uint64, including non-preferred encodings. -/
theorem wp_uint64_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hmajor : head &&& W8 224 = W8 0) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  rw [← hmajor]
  wp_apply wp_expect_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The typed scalar reader stores the value for every unsigned width. -/
theorem wp_readUint64_encoded (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 0)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ argument ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi (by intro h; subst head; contradiction)
    (by intro h; subst head; contradiction) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_encoded c v xs payload rem head argument dq enc hmajor hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- Truncation inside any extended head preserves the entire cursor and input. -/
theorem wp_headSlow_truncated (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (size : w64) (dq : DFrac) (kind : GoInterface)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (hsize : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (htrunc : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0))
        #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends inside the head") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hnotEnd : ¬ sint.Z v.buf'.len ≤ sint.Z v.pos' := by omega
  have hnotSmall : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := by change ¬ uint.Z (head &&& W8 31) < 24; omega
  have hnotReserved : ¬ uint.Z (W8 27) < uint.Z (head &&& W8 31) := by change ¬ 27 < uint.Z (head &&& W8 31); omega
  have hs : sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 := by word
  wp_auto
  simp only [hnotEnd, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hnotSmall, _root_.decide_false]
  wp_auto
  simp only [hnotReserved, _root_.decide_false]
  wp_auto
  have hleft : sint.Z (v.buf'.len - (v.pos' + W64 1)) < sint.Z size := by word
  simp only [hsize, hleft, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"input ends inside the head") slice.nil $$ [Hc]
    as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- The public-to-cursor head routine agrees with the same relation on both paths. -/
theorem wp_head_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #argument) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  by_cases hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24)
  · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
    simp only [hsmall, _root_.decide_true]
    wp_auto
    wp_end
  · simp only [hsmall, _root_.decide_false]
    wp_auto
    wp_apply wp_headSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end

/-- On the 64-bit target the mode reader accepts every uint64, not only named enum values. -/
theorem wp_readMode_encoded (c dst : Loc) (old : L1DAMode) (v : cursor.Cursor)
    (xs payload rem : List w8) (head : w8) (argument : w64) (dq : DFrac)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 0)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMode)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ argument ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi (by intro h; subst head; contradiction)
    (by intro h; subst head; contradiction) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_encoded c v xs payload rem head argument dq enc hmajor hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_byte

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_u32
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_u64

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_binaryBigEndian_u16

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_u16

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_inline_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_inline_inv

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_inline_major

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_truncated

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_head_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMode_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.byte_major_remainder
