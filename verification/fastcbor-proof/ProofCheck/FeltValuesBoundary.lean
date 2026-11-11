import ProofCheck.FeltValuesSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual generic slice decoder instantiated at felt.Felt accepts null and
    undefined without allocating or changing depth. -/
theorem wp_limbSlice_null (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ RET (PairV #slice.nil #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- Every rejected outer array head is propagated with the original cursor and
    actual owned diagnostic, before allocation and depth checks. -/
theorem wp_limbSlice_outer_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 128) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_expect_failure c v xs wire (W8 128) dq reason kind (by decide) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- The five-byte budget check runs before depth checks, rolls back the array
    head, and owns the exact count/remaining-byte diagnostic arguments. -/
theorem wp_limbSlice_count (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) < 5 * uint.Z argument)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos'
         (go!"count %d does not fit the %d bytes left at five bytes per felt") args ∗
       args ↦* [interface.mkOk go.uint64 #argument,
         interface.mkOk go.int #(v.buf'.len - (v.pos' + W64 (1 + payload.length)))] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hwidth := headArgument_width head payload argument enc
  have hn := array_head_not_nullish head major
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload rem head argument dq enc hp hbound hi hbytes
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hbad : uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z argument := by word
  simp only [hbad, _root_.decide_true]
  wp_auto
  wp_apply wp_slice_literal (V := GoInterface)
    [interface.mkOk go.uint64 #argument,
      interface.mkOk go.int #(v.buf'.len - (v.pos' + W64 (1 + payload.length)))]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (2 : Int) 0) 0 = 2 from by decide]
  wp_auto
  wp_apply wp_fail c v kind (go!"count %d does not fit the %d bytes left at five bytes per felt") (slice.mk args (W64 2) (W64 2))
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

/-- With a sufficient byte budget, exhausted outer depth rejects the slice and
    restores its cursor before any output allocation. -/
theorem wp_limbSlice_depth (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : 32 ≤ sint.Z v.depth')
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 5 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hwidth := headArgument_width head payload argument enc
  have hn := array_head_not_nullish head major
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload rem head argument dq enc hp hbound hi hbytes
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z argument := by word
  simp only [hok, _root_.decide_false]
  wp_auto
  wp_apply wp_enter_depth c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' kind hdepth
    $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- Nonempty slices at depth 31 reject their extra felt nesting level and
    restore both cursor and depth before allocating. -/
theorem wp_limbSlice_elementDepth (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
    (hp : 0 ≤ sint.Z v.pos') (hdepth : sint.Z v.depth' = 31) (nonempty : 0 < uint.Z argument)
    (hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len)
    (hcount : 5 * uint.Z argument ≤ sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length))
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hbytes : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  have hwidth := headArgument_width head payload argument enc
  have hn := array_head_not_nullish head major
  wp_auto
  wp_apply wp_null_other c v xs head dq hp hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have hexpect := wp_expect_encoded (GF := GF) c v xs payload rem head argument dq enc hp hbound hi hbytes
  rw [major] at hexpect
  wp_apply hexpect $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_method_call
  wp_call
  wp_auto
  have hok : ¬ uint.Z ((v.buf'.len - (v.pos' + W64 (1 + payload.length))) / W64 5) < uint.Z argument := by word
  simp only [hok, _root_.decide_false]
  wp_auto
  wp_apply wp_enter c {v with pos' := v.pos' + W64 (1 + payload.length)} v.pos' (by simp only; omega)
    $$ [Hc] with Hc
  · iframe Hc
  have hnonempty : uint.Z (W64 0) < uint.Z argument := nonempty
  have hdeep : sint.Z (W64 32) ≤ sint.Z (v.depth' + W64 1) := by word
  simp only [hnonempty, _root_.decide_true]
  wp_auto
  simp only [hdeep, _root_.decide_true]
  wp_auto
  wp_apply wp_errDepth v.pos' kind as %err Herr
  · iframe Hkind
  have hrestore : v.depth' + W64 1 - W64 1 = v.depth' := by word
  isimp only [hrestore] at Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_null
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_outer_failure

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_count
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_depth

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_elementDepth
