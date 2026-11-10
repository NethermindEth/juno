import ProofCheck.LimbsSlice

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

/-- Preserve the actual allocator's nonnil result even for a zero-length slice.
    The standard allocation contract drops this fact, but nullable CBOR output
    ownership needs to distinguish an allocated empty slice from nil. -/
theorem wp_slice_make3_nonnil {V : Type} [ZeroVal V] [TypedPointsto (GF := GF) V]
    {st t : go.GoType} [st ↓u go.SliceType t] [IntoValTyped (GF := GF) V t]
    (len cap : w64) (hle : 0 ≤ sint.Z len ∧ sint.Z len ≤ sint.Z cap) :
    {{ (True : IProp GF) }}
      (App (App (Val #(functions go.make3 [st])) (Val #len)) (Val #cap))
    {{ (sl : GoSlice), RET #sl;
        sl ↦* (List.replicate (sint.nat len) (zero_val V)) ∗
        ownSliceCap V sl (DFrac.own 1) ∗ ⌜sl.cap = cap ∧ sl.ptr ≠ null⌝ }} := by
  wp_start
  wp_if_destruct
  · exfalso; omega
  wp_if_destruct
  · exfalso; word
  wp_if_destruct
  · wp_apply wp_GoPrealloc as %p %hnn
    have hlen : len = W64 0 := by word
    subst hlen
    iapply HΦ
    isplitl []
    · have : sint.nat (W64 0) = 0 := rfl
      rw [this, List.replicate_zero]
      iapply ownSlice_empty _ _ (by rfl) (by word)
      iapply array_empty _ _ hnn
    isplitl []
    · iapply ownSliceCap_empty _ rfl (by word)
    · ipureintro; exact ⟨rfl, hnn⟩
  · ihave %Hnn := typedPointsto_not_null _ _ _ $$ p
    iapply HΦ
    have hz : (zero_val (GoArray V (sint.Z cap))).arr = List.replicate (sint.Z cap).toNat (zero_val V) := rfl
    have e := array_split (GF := GF) len p_ptr (DFrac.own 1) _ (zero_val (GoArray V (sint.Z cap))) ⟨by omega, by omega⟩
    rw [hz, List.take_replicate, List.drop_replicate,
      show min (sint.nat len) (sint.Z cap).toNat = sint.nat len by word] at e
    icases e.1 $$ p with ⟨Hsl, Hcap⟩
    rw [ownSlice_unseal, ownSliceCap_unseal]; unfold ownSliceDef ownSliceCapDef
    isplitl [Hsl]
    · iright; iframe Hsl; ipureintro; dsimp only; omega
    isplitl [Hcap]
    · iright
      isplit
      · ipureintro; dsimp only; omega
      iexists _
      simp only [sliceIndexRef]
      iexact Hcap
    · ipureintro; exact ⟨rfl, Hnn⟩

/-- The actual make([]T, n) produces owned zeroed storage and a nonnil slice,
    including n = 0, without changing trusted allocator semantics. -/
theorem wp_slice_make2_nonnil {V : Type} [ZeroVal V] [TypedPointsto (GF := GF) V]
    {st t : go.GoType} [st ↓u go.SliceType t] [IntoValTyped (GF := GF) V t]
    (len : w64) :
    {{ (⌜0 ≤ sint.Z len⌝ : IProp GF) }}
      (App (Val #(functions go.make2 [st])) (Val #len))
    {{ (sl : GoSlice), RET #sl;
        sl ↦* (List.replicate (sint.nat len) (zero_val V)) ∗
        ownSliceCap V sl (DFrac.own 1) ∗ ⌜sl.ptr ≠ null⌝ }} := by
  wp_start as %Hlen
  wp_apply wp_slice_make3_nonnil len len ⟨Hlen, Int.le_refl _⟩ as %sl ⟨Hs, Hcap, %Hout⟩
  iapply HΦ
  iframe Hs Hcap
  ipureintro; exact Hout.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_slice_make3_nonnil
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_slice_make2_nonnil
