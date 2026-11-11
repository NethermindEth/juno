import ProofCheck.SliceAllocation

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Keep the zero contents of the entire new allocation, including capacity
    beyond length. The standard make contract intentionally forgets those values. -/
theorem wp_slice_make3_full_zero {V : Type} [ZeroVal V] [TypedPointsto (GF := GF) V]
    {st t : go.GoType} [st ↓u go.SliceType t] [IntoValTyped (GF := GF) V t]
    (len cap : w64) (hle : 0 < sint.Z len ∧ sint.Z len ≤ sint.Z cap) :
    {{ (True : IProp GF) }}
      (App (App (Val #(functions go.make3 [st])) (Val #len)) (Val #cap))
    {{ (sl : GoSlice), RET #sl;
        {sl with len := sl.cap} ↦* (List.replicate (sint.nat cap) (zero_val V)) ∗
        ⌜sl.len = len ∧ sl.cap = cap⌝ }} := by
  wp_start
  wp_if_destruct
  · exfalso; omega
  wp_if_destruct
  · exfalso; word
  wp_if_destruct
  · exfalso; word
  · have hcap : W64 (sint.Z cap) = cap := by word
    ihave Hfull := slice_array p_ptr (sint.Z cap) (zero_val (GoArray V (sint.Z cap)))
      (by change (List.replicate (sint.Z cap).toNat (zero_val V)).length < 2 ^ 63; simp only [List.length_replicate]; word) $$ p
    isimp only [hcap] at Hfull
    iapply HΦ
    iframe Hfull
    ipureintro; exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_slice_make3_full_zero
