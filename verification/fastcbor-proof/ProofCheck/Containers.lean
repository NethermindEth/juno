import ProofCheck.Head

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

theorem headArgument_width (head : w8) (payload : List w8) (argument : w64)
    (enc : HeadArgument head payload argument) : payload.length ≤ 8 := by
  cases enc <;> simp_all

/-- Opening one container increments depth and changes no other cursor state. -/
theorem wp_enter (c : Loc) (v : cursor.Cursor) (start : w64)
    (hdepth : sint.Z v.depth' < 32) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ c ↦ v }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"enter")) (Val #start))
    {{ RET #interface.nil; c ↦ {v with depth' := v.depth' + W64 1} }} := by
  wp_start as Hc
  wp_auto
  have hd : ¬ sint.Z (W64 32) ≤ sint.Z v.depth' := by change ¬ 32 ≤ sint.Z v.depth'; omega
  simp only [hd, _root_.decide_false]
  wp_auto
  wp_end

/-- Slow container opening for either arrays or maps, at any width and offset.
The mathematical size bound ensures the count is backed by remaining bytes. -/
theorem wp_containerLen_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument perItem : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hper : perItem = W64 1 ∨ perItem = W64 2)
    (hcount : uint.Z argument * uint.Z perItem ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"containerLen"))
        (Val #(head &&& W8 224))) (Val #perItem))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
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
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' hdepth
    $$ [Hc] with Hc
  · iframe Hc
  wp_end

/-- The inline ArrayLen shortcut, including its byte-count and nesting checks. -/
theorem wp_ArrayLen_inline (c : Loc) (v : cursor.Cursor) (xs : List w8) (head : w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hmajor : head &&& W8 224 = W8 128) (hinfo : uint.Z (head &&& W8 31) < 24)
    (hbound : sint.Z v.pos' + 1 + 1 * uint.Z (head &&& W8 31) ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"ArrayLen")) (Val #()))
    {{ RET (PairV #(W64 (uint.Z (head &&& W8 31))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by have := (head &&& W8 31).isLt; word
  have hd : sint.Z v.depth' < sint.Z (W64 32) := hdepth
  have hsub : head - W8 128 = head &&& W8 31 := by
    simpa only [hmajor] using byte_major_remainder head
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  simp only [hd, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  have hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24) := hinfo
  simp only [hsub, hsmall, _root_.decide_true]
  wp_auto
  have hcount : sint.Z (W64 (uint.Z (head &&& W8 31))) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hcount, _root_.decide_true]
  wp_auto
  wp_end

/-- The inline MapLen shortcut, including its byte-count and nesting checks. -/
theorem wp_MapLen_inline (c : Loc) (v : cursor.Cursor) (xs : List w8) (head : w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' < 32)
    (hmajor : head &&& W8 224 = W8 160) (hinfo : uint.Z (head &&& W8 31) < 24)
    (hbound : sint.Z v.pos' + 1 + 2 * uint.Z (head &&& W8 31) ≤ sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ RET (PairV #(W64 (uint.Z (head &&& W8 31))) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1, depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by have := (head &&& W8 31).isLt; word
  have hd : sint.Z v.depth' < sint.Z (W64 32) := hdepth
  have hsub : head - W8 160 = head &&& W8 31 := by
    simpa only [hmajor] using byte_major_remainder head
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  simp only [hd, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  have hsmall : uint.Z (head &&& W8 31) < uint.Z (W8 24) := hinfo
  simp only [hsub, hsmall, _root_.decide_true]
  wp_auto
  have hcount : sint.Z (W64 2 * W64 (uint.Z (head &&& W8 31))) < sint.Z (v.buf'.len - v.pos') := by word
  simp only [hcount, _root_.decide_true]
  wp_auto
  wp_end

/-- The public ArrayLen method, composing both paths at every supported width. -/
theorem wp_ArrayLen_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hmajor : head &&& W8 224 = W8 128) (hp : 0 ≤ sint.Z v.pos')
    (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 1 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"ArrayLen")) (Val #()))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs }} := by
  by_cases hsmall : uint.Z (head &&& W8 31) < 24
  · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
    have hc : sint.Z v.pos' + 1 + 1 * uint.Z (head &&& W8 31) ≤ sint.Z v.buf'.len := by
      simp only [List.length_nil] at hcount
      word
    simpa using wp_ArrayLen_inline c v xs head dq hp hdepth hmajor hsmall hc hi
  · wp_start as ⟨Hc, Hs⟩
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    have hd : sint.Z v.depth' < sint.Z (W64 32) := hdepth
    have hsub : head - W8 128 = head &&& W8 31 := by
      simpa only [hmajor] using byte_major_remainder head
    wp_auto
    simp only [hlt, _root_.decide_true]
    wp_auto
    simp only [hd, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    have hlarge : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := hsmall
    simp only [hsub, hlarge, _root_.decide_false]
    wp_auto
    rw [← hmajor]
    wp_apply wp_containerLen_encoded c v xs payload rem head argument (W64 1) dq enc hp hdepth hbound
      (by left; rfl) (by change uint.Z argument * 1 ≤ _; omega) hi hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end

/-- The public MapLen method, composing both paths at every supported width. -/
theorem wp_MapLen_encoded (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hmajor : head &&& W8 224 = W8 160) (hp : 0 ≤ sint.Z v.pos')
    (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 2 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ RET (PairV #argument #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs }} := by
  by_cases hsmall : uint.Z (head &&& W8 31) < 24
  · obtain ⟨rfl, rfl⟩ := headArgument_inline_inv head payload argument enc hsmall
    have hc : sint.Z v.pos' + 1 + 2 * uint.Z (head &&& W8 31) ≤ sint.Z v.buf'.len := by
      simp only [List.length_nil] at hcount
      word
    simpa using wp_MapLen_inline c v xs head dq hp hdepth hmajor hsmall hc hi
  · wp_start as ⟨Hc, Hs⟩
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by omega
    have hd : sint.Z v.depth' < sint.Z (W64 32) := hdepth
    have hsub : head - W8 160 = head &&& W8 31 := by
      simpa only [hmajor] using byte_major_remainder head
    wp_auto
    simp only [hlt, _root_.decide_true]
    wp_auto
    simp only [hd, _root_.decide_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    have hlarge : ¬ uint.Z (head &&& W8 31) < uint.Z (W8 24) := hsmall
    simp only [hsub, hlarge, _root_.decide_false]
    wp_auto
    rw [← hmajor]
    wp_apply wp_containerLen_encoded c v xs payload rem head argument (W64 2) dq enc hp hdepth hbound
      (by right; rfl) (by change uint.Z argument * 2 ≤ _; omega) hi hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_end

/-- A definite struct map opens with its full declared pair count and no seen fields. -/
theorem wp_openStruct_encoded (c fields : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hmajor : head &&& W8 224 = W8 160) (hp : 0 ≤ sint.Z v.pos')
    (hdepth : sint.Z v.depth' < 32)
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 2 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #(cursor.Struct.mk fields argument (W64 0) (W64 0) true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi (by intro h; subst head; contradiction)
    (by intro h; subst head; contradiction) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload rem head argument dq enc hmajor hp hdepth hbound hcount hi hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  simp only [show (0 : w64) = W64 0 from rfl]
  wp_end

/-- Closing a container restores its caller's depth without touching input or position. -/
theorem wp_leave (c : Loc) (v : cursor.Cursor) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ {v with depth' := v.depth' + W64 1} }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Leave")) (Val #()))
    {{ RET #(); c ↦ v }} := by
  wp_start as Hc
  wp_auto
  have hd : v.depth' + W64 1 - W64 1 = v.depth' := by word
  isimp only [hd] at Hc
  wp_end

/-- Finishing an open struct restores the enclosing depth and preserves field bookkeeping. -/
theorem wp_nextField_close (c st fields : Loc) (v : cursor.Cursor) (next seen : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ cursor.Struct.mk fields (W64 0) next seen true ∗
       c ↦ {v with depth' := v.depth' + W64 1} }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 (-1)) #interface.nil);
       st ↦ cursor.Struct.mk fields (W64 0) next seen false ∗ c ↦ v }} := by
  wp_start as ⟨Hst, Hc⟩
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_apply wp_leave c v $$ [Hc] with Hc
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headArgument_width
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_enter
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_containerLen_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ArrayLen_inline

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_MapLen_inline

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ArrayLen_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_MapLen_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_encoded

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_leave

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_close
