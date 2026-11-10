import ProofCheck.MapExpectedFailure

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A rejected map expectation propagates without allocating or changing
    the destination. Null/undefined are handled separately by the wrapper. -/
theorem wp_readGasPrice_outer_failure (c dst fields : Loc) (old : GasPrice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 160) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 160) reason kind }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_expected_failure c fields v xs wire dq reason kind notNull notUndefined failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for
  wp_end

/-- An impossible pair count is rejected before map entry and depth entry. -/
theorem wp_readGasPrice_count_failure (c dst fields : Loc) (old : GasPrice) (v : cursor.Cursor)
    (xs payload tail : List w8) (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
    (count : tail.length < 2 * uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"count exceeds the input") slice.nil }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hlen
  have htail : (tail.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
  have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_count c fields v xs payload tail head argument dq kind enc major hp.1 hbound
    (by rw [← htail]; omega) hi hafter
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for
  wp_end

/-- Exhausted nesting depth is rejected after a valid count, before map entry. -/
theorem wp_readGasPrice_depth_failure (c dst fields : Loc) (old : GasPrice) (v : cursor.Cursor)
    (xs payload tail : List w8) (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
    (count : 2 * uint.Z argument ≤ tail.length) (depth : 32 ≤ sint.Z v.depth')
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗
       globalAddr cursor.ErrDepth ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"nested more than %d levels") args ∗
       args ↦* [interface.mkOk go.int #(W64 32)] }} := by
  wp_start as ⟨#Hfields, #Hkind, Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hlen := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hlen
  have htail : (tail.length : Int) = sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [sint.nat, sint.Z] at Hlen hp hlen ⊢; omega
  have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_depth c fields v xs payload tail head argument dq kind enc major hp.1 depth hbound
    (by rw [← htail]; omega) hi hafter
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_for
  wp_end

/-- Null/undefined preserve the existing GasPrice amounts, consume one byte,
    and leave the caller's depth unchanged. They never access field metadata. -/
theorem wp_readGasPrice_nullish (c dst fields : Loc) (old : GasPrice) (v : cursor.Cursor)
    (xs : List w8) (head : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨#Hfields, Hdst, Hc, Hs⟩
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_nullish c fields v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_for
  wp_apply wp_nextField_done c _ fields $$ [s] with Hst
  · iframe
  wp_for_post
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_outer_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_count_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_depth_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_nullish
