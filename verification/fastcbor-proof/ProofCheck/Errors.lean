import ProofCheck.Null

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

/-- Depth errors carry the caller's starting offset and an owned diagnostic argument. -/
theorem wp_errDepth (start : w64) (kind : GoInterface) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind }}
      (App (Val (@! cursor.errDepth)) (Val #start))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind start (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as #Hkind
  wp_auto
  wp_apply wp_slice_literal (V := GoInterface) [interface.mkOk go.int #(W64 32)]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_alloc err as Herr
  wp_auto
  wp_end

/-- Exhausted depth does not alter the cursor, and the supplied start identifies the error. -/
theorem wp_enter_depth (c : Loc) (v : cursor.Cursor) (start : w64) (kind : GoInterface)
    (hdepth : 32 ≤ sint.Z v.depth') :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"enter")) (Val #start))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ v ∗ ∃ args : GoSlice, err ↦ cursor.DecodeError.mk kind start (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc⟩
  wp_auto
  have hd : sint.Z (W64 32) ≤ sint.Z v.depth' := hdepth
  simp only [hd, _root_.decide_true]
  wp_auto
  wp_apply wp_errDepth start kind as %err Herr
  · iframe Hkind
  wp_end

/-- A count larger than the remaining bytes allows is rejected before entering the container.
This also establishes that count truncation takes precedence over a depth failure. -/
theorem wp_containerLen_count (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument perItem : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hper : perItem = W64 1 ∨ perItem = W64 2)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < uint.Z argument * uint.Z perItem)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"containerLen"))
        (Val #(head &&& W8 224))) (Val #perItem))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hwidth := headArgument_width head payload argument enc
  wp_auto
  wp_apply wp_expectSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hnz : uint.Z perItem ≠ 0 := by rcases hper with rfl | rfl <;> decide
  simp only [hnz, ite_false]
  wp_auto
  have hcountbad : uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perItem) < uint.Z argument := by
    rcases hper with rfl | rfl <;> word
  simp only [hcountbad, _root_.decide_true]
  wp_auto
  wp_apply wp_fail c v kind (go!"count exceeds the input") slice.nil $$ [Hc]
    as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- With a valid count, exhausted depth rewinds the parsed head and preserves the input. -/
theorem wp_containerLen_depth (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument perItem : w64) (dq : DFrac) (kind : GoInterface) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hper : perItem = W64 1 ∨ perItem = W64 2)
    (hcount : uint.Z argument * uint.Z perItem ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"containerLen"))
        (Val #(head &&& W8 224))) (Val #perItem))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hwidth := headArgument_width head payload argument enc
  wp_auto
  wp_apply wp_expectSlow_encoded c v xs payload rem head argument dq enc hp hbound hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hnz : uint.Z perItem ≠ 0 := by rcases hper with rfl | rfl <;> decide
  simp only [hnz, ite_false]
  wp_auto
  have hcountok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / perItem) < uint.Z argument := by
    rcases hper with rfl | rfl <;> word
  simp only [hcountok, _root_.decide_false]
  wp_auto
  wp_apply wp_enter_depth c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' kind hdepth
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- ArrayLen propagates depth rejection through its fast-path guard without consuming input. -/
theorem wp_ArrayLen_depth (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 1 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"ArrayLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hd : ¬ sint.Z v.depth' < sint.Z (W64 32) := by change ¬ sint.Z v.depth' < 32; omega
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  simp only [hd, _root_.decide_false]
  wp_auto
  rw [← hmajor]
  wp_apply wp_containerLen_depth c v xs payload rem head argument (W64 1) dq kind enc hp hdepth hbound
    (by left; rfl) (by change uint.Z argument * 1 ≤ _; omega) hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- ArrayLen propagates count rejection through its fast-path guard without consuming input. -/
theorem wp_ArrayLen_count (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') 
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < 1 * uint.Z argument)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"ArrayLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  have hslow := wp_containerLen_count (GF := GF) (hG := hG) c v xs payload rem head argument (W64 1) dq kind enc hp hbound
    (by left; rfl) (by change _ < uint.Z argument * 1; omega) hi hbytes
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hsub : head - W8 128 = head &&& W8 31 := by
    simpa only [hmajor] using byte_major_remainder head
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  by_cases hd : sint.Z v.depth' < sint.Z (W64 32)
  · simp only [hd, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    by_cases hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24)
    · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
      simp only [hsub, hsmall, _root_.decide_true]
      wp_auto
      have hbad : ¬ sint.Z (W64 (uint.Z (head &&& W8 31))) < sint.Z (v.buf'.len - v.pos') := by
        simp only [List.length_nil] at hcount
        word
      simp only [hbad, _root_.decide_false]
      wp_auto
      rw [← hmajor]
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    · simp only [hsub, hsmall, _root_.decide_false]
      wp_auto
      rw [← hmajor]
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
  · simp only [hd, _root_.decide_false]
    wp_auto
    rw [← hmajor]
    wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- MapLen propagates depth rejection through its fast-path guard without consuming input. -/
theorem wp_MapLen_depth (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 2 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hd : ¬ sint.Z v.depth' < sint.Z (W64 32) := by change ¬ sint.Z v.depth' < 32; omega
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  simp only [hd, _root_.decide_false]
  wp_auto
  rw [← hmajor]
  wp_apply wp_containerLen_depth c v xs payload rem head argument (W64 2) dq kind enc hp hdepth hbound
    (by right; rfl) (by change uint.Z argument * 2 ≤ _; omega) hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- MapLen propagates count rejection through its fast-path guard without consuming input. -/
theorem wp_MapLen_count (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos') 
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < 2 * uint.Z argument)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  have hslow := wp_containerLen_count (GF := GF) (hG := hG) c v xs payload rem head argument (W64 2) dq kind enc hp hbound
    (by right; rfl) (by change _ < uint.Z argument * 2; omega) hi hbytes
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
  have hsub : head - W8 160 = head &&& W8 31 := by
    simpa only [hmajor] using byte_major_remainder head
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  by_cases hd : sint.Z v.depth' < sint.Z (W64 32)
  · simp only [hd, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    by_cases hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24)
    · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
      simp only [hsub, hsmall, _root_.decide_true]
      wp_auto
      have hbad : ¬ sint.Z (W64 2 * W64 (uint.Z (head &&& W8 31))) < sint.Z (v.buf'.len - v.pos') := by
        simp only [List.length_nil] at hcount
        word
      simp only [hbad, _root_.decide_false]
      wp_auto
      rw [← hmajor]
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    · simp only [hsub, hsmall, _root_.decide_false]
      wp_auto
      rw [← hmajor]
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
  · simp only [hd, _root_.decide_false]
    wp_auto
    rw [← hmajor]
    wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- Struct opening preserves count failure, including the untouched cursor and empty field state. -/
theorem wp_openStruct_count (c fields : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos') 
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < 2 * uint.Z argument)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (err : Loc), RET (PairV #({(zero_val cursor.Struct) with fields' := fields}) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi (by intro h; subst head; contradiction)
    (by intro h; subst head; contradiction) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_count c v xs payload rem head argument dq kind enc hmajor hp hbound hcount hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Struct opening preserves depth failure, including the untouched cursor and empty field state. -/
theorem wp_openStruct_depth (c fields : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (hmajor : head &&& W8 224 = W8 160)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 2 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (err : Loc), RET (PairV #({(zero_val cursor.Struct) with fields' := fields}) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi (by intro h; subst head; contradiction)
    (by intro h; subst head; contradiction) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_depth c v xs payload rem head argument dq kind enc hmajor hp hdepth hbound hcount hi hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_errDepth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_enter_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_containerLen_count

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_containerLen_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ArrayLen_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ArrayLen_count

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_MapLen_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_MapLen_count

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_count

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_depth
