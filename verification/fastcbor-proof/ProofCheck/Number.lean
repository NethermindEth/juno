import ProofCheck.BigInt
import Perennial.Golang.Theory.String

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

private abbrev hashKey : cursor.StaticField :=
  ⟨array.mk 3 [W64 0x6873614864, W64 0, W64 0],
   array.mk 3 [W64 0xffffffffff, W64 0, W64 0], go!"dHash", W64 0, W64 1, true⟩

private abbrev numberKey : cursor.StaticField :=
  ⟨array.mk 3 [W64 0x7265626d754e66, W64 0, W64 0],
   array.mk 3 [W64 0xffffffffffffff, W64 0, W64 0], go!"fNumber", W64 2, W64 4, true⟩

/-- The short-key search finds Number in the actual generated Header table. -/
theorem wp_findHeaderNumber (st fields : Loc) (v : cursor.Struct)
    (order names buf : GoSlice) (b : w8) (dq : DFrac)
    (hfields : v.fields' = fields) (hnext : v.next' = W64 0)
    (horder : order.len = W64 15) (hbuf : buf.len = W64 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ v ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ buf ↦*{dq} (go!"fNumber" ++ [b]) }}
      (App (Val (st @!! go.PointerType cursor.Struct.ty @!! go!"find")) (Val #buf))
    {{ RET #(W64 1); st ↦ v ∗ fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ buf ↦*{dq} (go!"fNumber" ++ [b]) }} := by
  wp_start as ⟨Hst, Hfields, Horder, Hbuf⟩
  wp_auto
  simp only [hfields]
  wp_auto
  simp only [hbuf]
  wp_auto
  isimp only [hnext] at i
  ihave HI : (∃ (i : w64), "i" ∷ i_ptr ↦ i ∗
      "%Hi" ∷ ⌜i = W64 0 ∨ i = W64 1⌝ : IProp GF) $$ [i]
  · iexists (W64 0); iframe; ipureintro; exact Or.inl rfl
  have hbpos : decide (sint.Z (W64 0) < sint.Z buf.len) = true := by rw [hbuf]; decide
  wp_for HI
  rcases Hi with rfl | rfl
  · simp only [horder]
    wp_auto
    simp only [hbpos, _root_.decide_true, ite_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [horder]; decide))]
    icases ownSlice_elem_acc 0 (hashKey) order (DFrac.own 1) headerFieldKeys
      (by decide) (by rfl) $$ Horder with ⟨Hkey, Hclose⟩
    isimp only [hashKey] at Hkey
    simp only [show sint.Z (W64 0) = 0 from rfl]
    wp_auto
    ihave Horder := Hclose $$ Hkey
    ieval (change (order ↦* headerFieldKeys : IProp GF) ⊢ _) at Horder
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hbuf]; decide))]
    wp_apply wp_load_slice_index (t := go.byte) buf (sint.Z (W64 0)) (go!"fNumber" ++ [b]) dq (W8 102) (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hbuf]; decide))]
    wp_apply wp_load_slice_index (t := go.byte) buf (sint.Z (W64 0)) (go!"fNumber" ++ [b]) dq (W8 102) (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    wp_for_post
    iframe
    iexists (W64 1); iframe; ipureintro; exact Or.inr rfl
  · simp only [horder]
    wp_auto
    simp only [hbpos, _root_.decide_true, ite_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [horder]; decide))]
    icases ownSlice_elem_acc 1 (numberKey) order (DFrac.own 1) headerFieldKeys
      (by decide) (by rfl) $$ Horder with ⟨Hkey, Hclose⟩
    isimp only [numberKey] at Hkey
    simp only [show sint.Z (W64 1) = 1 from rfl]
    wp_auto
    ihave Horder := Hclose $$ Hkey
    ieval (change (order ↦* headerFieldKeys : IProp GF) ⊢ _) at Horder
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hbuf]; decide))]
    wp_apply wp_load_slice_index (t := go.byte) buf (sint.Z (W64 0)) (go!"fNumber" ++ [b]) dq (W8 102) (by decide) $$ [Hbuf] with Hbuf
    · iframe Hbuf; ipureintro; rfl
    wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
    simp only [hbuf]
    wp_auto
    wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
    ihave %Hwf := ownSlice_wf _ _ _ $$ Hbuf
    have hb : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z (W64 7) ∧
        sint.Z (W64 7) ≤ sint.Z buf.len := by rw [hbuf]; decide
    have hcap : sint.Z (W64 7) ≤ sint.Z buf.cap := by
      rw [hbuf] at Hwf
      exact Int.le_trans (by decide) Hwf.2
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hb.1, hb.2.1, hcap⟩)]
    icases (ownSlice_slice (W64 0) (W64 7) buf dq (go!"fNumber" ++ [b]) hb).1 $$ Hbuf
      with ⟨Hpre, Hbytes, Hpost⟩
    have hs : subslice (sint.nat (W64 0)) (sint.nat (W64 7)) (go!"fNumber" ++ [b]) = go!"fNumber" := rfl
    isimp only [hs] at Hbytes
    wp_apply wp_bytes_to_string (slice.slice buf w8 (W64 0) (W64 7)) (go!"fNumber") dq $$ [Hbytes] with Hbytes
    · iexact Hbytes
    ihave Hbuf : (buf ↦*{dq} (go!"fNumber" ++ [b]) : IProp GF) $$ [Hpre Hbytes Hpost]
    · iapply (ownSlice_slice (W64 0) (W64 7) buf dq (go!"fNumber" ++ [b]) hb).2
      rw [hs]
      iframe Hpre Hbytes Hpost
    wp_for_post
    wp_end

/-- Reading the Number key advances exactly seven bytes and marks the field seen. -/
theorem wp_nextHeaderNumber_count (c st fields : Loc) (order names data : GoSlice)
    (b head : w8) (left : w64) (dq : DFrac) (depth : w64)
    (hleft : 0 < sint.Z left)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields left (W64 0) (W64 0) true ∗
       c ↦ cursor.Cursor.mk data (W64 1) depth (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (head :: (go!"fNumber" ++ [b])) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 2) #interface.nil);
       st ↦ cursor.Struct.mk fields (left - W64 1) (W64 2) (W64 4) true ∗
       c ↦ cursor.Cursor.mk data (W64 8) depth (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (head :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨Hst, Hc, Hfields, Horder, Hdata⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hdata
  wp_auto
  simp only [show sint.Z (W64 0) = 0 from rfl, hleft, _root_.decide_true]
  wp_auto
  simp only [horder]
  wp_auto
  simp only [hdata]
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hn : left ≠ W64 0 := by intro h; rw [h] at hleft; contradiction
  simp only [hn, _root_.decide_false]
  wp_auto
  have hb : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.cap := by rw [hdata] at *; exact ⟨by decide, by decide, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
  icases (ownSlice_slice (W64 1) data.len data dq (head :: (go!"fNumber" ++ [b]))
    ⟨hb.1, hb.2.1, Int.le_refl _⟩).1 $$ Hdata with ⟨Hpre, Hbytes, Hpost⟩
  have hs : subslice (sint.nat (W64 1)) (sint.nat data.len)
      (head :: (go!"fNumber" ++ [b])) = go!"fNumber" ++ [b] := by rw [hdata]; rfl
  isimp only [hs] at Hbytes
  wp_auto
  ieval (change (st ↦ cursor.Struct.mk fields (left - W64 1) (W64 0) (W64 0) true : IProp GF) ⊢ _) at Hst
  wp_apply wp_findHeaderNumber st fields (cursor.Struct.mk fields (left - W64 1) (W64 0) (W64 0) true)
    order names (slice.slice data w8 (W64 1) data.len) b dq rfl rfl horder
    (by simp only [slice.slice, hdata]; rfl)
    $$ [Hst Hfields Horder Hbytes] with ⟨Hst, Hfields, Horder, Hbytes⟩
  · iframe Hst Hfields Horder Hbytes
  ihave Hdata : (data ↦*{dq} (head :: (go!"fNumber" ++ [b])) : IProp GF) $$ [Hpre Hbytes Hpost]
  · iapply (ownSlice_slice (W64 1) data.len data dq (head :: (go!"fNumber" ++ [b]))
      ⟨hb.1, hb.2.1, Int.le_refl _⟩).2
    rw [hs]
    iframe Hpre Hbytes Hpost
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [horder]; decide))]
  icases ownSlice_elem_acc (sint.Z (W64 1)) numberKey order (DFrac.own 1) headerFieldKeys
    (by decide) (by rfl) $$ Horder with ⟨Hkey, Hclose⟩
  isimp only [numberKey] at Hkey
  wp_auto
  wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %-
  ihave Horder := Hclose $$ Hkey
  ieval (change (order ↦* headerFieldKeys : IProp GF) ⊢ _) at Horder
  wp_method_call
  wp_call
  wp_auto
  wp_apply wp_shiftCount_int
  · ipureintro; decide
  wp_end

/-- Reading the Number key advances exactly seven bytes and marks the field seen. -/
theorem wp_nextHeaderNumber (c st fields : Loc) (order names data : GoSlice)
    (b : w8) (dq : DFrac) (depth : w64)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields (W64 1) (W64 0) (W64 0) true ∗
       c ↦ cursor.Cursor.mk data (W64 1) depth (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 2) #interface.nil);
       st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true ∗
       c ↦ cursor.Cursor.mk data (W64 8) depth (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }} := by
  iintro %Φ ⟨#Hpkg, Hst, Hc, Hfields, Horder, Hdata⟩ HΦ
  wp_apply wp_nextHeaderNumber_count c st fields order names data b (W8 161) (W64 1)
    dq depth (by decide) horder hdata $$ [Hst Hc Hfields Horder Hdata]
    with ⟨Hst, Hc, Hfields, Horder, Hdata⟩
  · iframe Hst Hc Hfields Horder Hdata
  ieval (change (st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true : IProp GF) ⊢ _) at Hst
  wp_end

/-- Open the one-pair map containing Number, consuming only its map head. -/
theorem wp_openHeaderNumber (c fields : Loc) (data : GoSlice) (b : w8) (dq : DFrac)
    (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗
       data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #(cursor.Struct.mk fields (W64 1) (W64 0) (W64 0) true) #interface.nil);
       c ↦ cursor.Cursor.mk data (W64 1) (W64 1) (zero_val (GoArray w64 4)) ∗
       data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨Hc, Hdata⟩
  wp_auto
  wp_apply wp_null_other c (cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4))) (W8 161 :: (go!"fNumber" ++ [b])) (W8 161) dq
    (by change 0 ≤ sint.Z (W64 0); decide) rfl (by decide) (by decide) $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  wp_method_call
  wp_call
  wp_auto
  simp only [hdata]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [hdata]; decide))]
  wp_apply wp_load_slice_index (t := go.byte) data (sint.Z (W64 0))
    (W8 161 :: (go!"fNumber" ++ [b])) dq (W8 161) (by decide) $$ [Hdata] with Hdata
  · iframe Hdata; ipureintro; rfl
  simp only [hdata]
  wp_auto
  simp only [show W64 (uint.Z (W8 161 - W8 160)) = W64 1 from rfl, show (0 : w64) = W64 0 from rfl]
  wp_end

/-- Finishing the one-pair map restores the cursor's original nesting depth. -/
theorem wp_doneHeaderNumber (c st fields : Loc) (data : GoSlice) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) false ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 0) (zero_val (GoArray w64 4)) }} := by
  wp_start as ⟨Hst, Hc⟩
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_end

/-- A populated record through the actual generated reader, including field search and stores. -/
theorem wp_readHeaderNumber (c dst fields : Loc) (order names data : GoSlice)
    (h : Header) (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ dst ↦ h ∗
       c ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ {h with Number' := W64 (uint.Z b)} ∗
       c ↦ cursor.Cursor.mk data (W64 9) (W64 0) (zero_val (GoArray w64 4)) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨#Hglobal, Hdst, Hc, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_openHeaderNumber c fields data b dq hdata $$ [Hc Hdata] with ⟨Hc, Hdata⟩
  · iframe Hc Hdata
  ihave HI : (∃ (done : Bool),
      "Hs" ∷ s_ptr ↦ (if done then cursor.Struct.mk fields (W64 0) (W64 2) (W64 4) true
                                else cursor.Struct.mk fields (W64 1) (W64 0) (W64 0) true) ∗
      "Hc" ∷ c ↦ cursor.Cursor.mk data (if done then W64 9 else W64 1) (W64 1) (zero_val (GoArray w64 4)) ∗
      "Hdst" ∷ dst ↦ (if done then {h with Number' := W64 (uint.Z b)} else h) : IProp GF) $$ [s Hc Hdst]
  · iexists false; isimp only [Bool.false_eq_true, ite_false]; iframe
  wp_for HI
  cases done with
  | false =>
    isimp only [Bool.false_eq_true, ite_false] at Hs Hc Hdst
    wp_apply wp_nextHeaderNumber c s_ptr fields order names data b dq (W64 1) horder hdata
      $$ [Hs Hc Hfields Horder Hdata] with ⟨Hs, Hc, Hfields, Horder, Hdata⟩
    · iframe Hs Hc Hfields Horder Hdata
    wp_apply wp_headerNumber_inline_at c dst h
      (cursor.Cursor.mk data (W64 8) (W64 1) (zero_val (GoArray w64 4)))
      (W8 161 :: (go!"fNumber" ++ [b])) b dq (by change 0 ≤ sint.Z (W64 8); decide) rfl hb
      $$ [Hdst Hc Hdata] with ⟨Hdst, Hc, Hdata⟩
    · iframe Hdst Hc Hdata
    wp_for_post
    iframe
    iexists true
    isimp only [ite_true]
    ieval (change (c ↦ cursor.Cursor.mk data (W64 9) (W64 1) (zero_val (GoArray w64 4)) : IProp GF) ⊢ _) at Hc
    iframe Hs Hc Hdst
  | true =>
    isimp only [ite_true] at Hs Hc Hdst
    wp_apply wp_doneHeaderNumber c s_ptr fields data $$ [Hs Hc] with ⟨Hs, Hc⟩
    · iframe Hs Hc
    wp_for_post
    wp_end

/-- The public entry returns the populated Header and the exact empty suffix. -/
theorem wp_decodeHeaderFirst_Number (dst fields : Loc) (order names data : GoSlice)
    (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ dst ↦ (zero_val Header) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #data)) (Val #dst))
    {{ RET (PairV #(slice.slice data w8 (W64 9) data.len) #interface.nil);
       dst ↦ {(zero_val Header) with Number' := W64 (uint.Z b)} ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨#Hglobal, Hdst, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_zeroHeader dst $$ [Hdst] with Hdst
  · iframe Hdst
  wp_apply wp_newCursor data
  ieval (change (c_ptr ↦ cursor.Cursor.mk data (W64 0) (W64 0) (zero_val (GoArray w64 4)) : IProp GF) ⊢ _) at c
  wp_apply wp_readHeaderNumber _ dst fields order names data (zero_val Header) b dq hb horder hdata
    $$ [c Hdst Hfields Horder Hdata] with ⟨Hdst, Hc, Hfields, Horder, Hdata⟩
  · iframe Hglobal c Hdst Hfields Horder Hdata
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hdata
  wp_apply wp_remaining _ (cursor.Cursor.mk data (W64 9) (W64 0) (zero_val (GoArray w64 4)))
    (by change 0 ≤ sint.Z (W64 9) ∧ sint.Z (W64 9) ≤ sint.Z data.len; rw [hdata]; decide)
    Hwf.2 $$ [Hc] with Hc
  · iframe Hc
  wp_end

/-- The full-input API accepts the populated record and preserves its input bytes. -/
theorem wp_decodeHeader_Number (dst fields : Loc) (order names data : GoSlice)
    (b : w8) (dq : DFrac) (hb : uint.Z b < 24)
    (horder : order.len = W64 15) (hdata : data.len = W64 9) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ dst ↦ (zero_val Header) ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }}
      (App (App (Val (@! DecodeHeader)) (Val #data)) (Val #dst))
    {{ RET #interface.nil; dst ↦ {(zero_val Header) with Number' := W64 (uint.Z b)} ∗
       fields ↦ cursor.Fields.mk order names (W64 32767) ∗
       order ↦* headerFieldKeys ∗ data ↦*{dq} (W8 161 :: (go!"fNumber" ++ [b])) }} := by
  wp_start as ⟨#Hglobal, Hdst, Hfields, Horder, Hdata⟩
  wp_auto
  wp_apply wp_decodeHeaderFirst_Number dst fields order names data b dq hb horder hdata
    $$ [Hdst Hfields Horder Hdata] with ⟨Hdst, Hfields, Horder, Hdata⟩
  · iframe Hglobal Hdst Hfields Horder Hdata
  simp only [slice.slice, hdata]
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_findHeaderNumber

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextHeaderNumber

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openHeaderNumber
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_doneHeaderNumber

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderNumber

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_Number
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_Number

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextHeaderNumber_count
