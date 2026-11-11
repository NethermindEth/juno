import ProofCheck.BytesIntoComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The full slice of an owned address has exactly the same twenty byte cells.
    This bridge neither allocates nor assumes validity of an arbitrary model array. -/
theorem address_slice_owned (dst : Loc) (old : GoArray w8 20) (dq : DFrac) :
    (dst ↦{dq} old : IProp GF) ⊣⊢ (slice.mk dst (W64 20) (W64 20)) ↦*{dq} old.arr := by
  cases old with
  | mk bytes =>
    rw [ownSlice_unseal]
    unfold ownSliceDef
    change (dst ↦{dq} (array.mk 20 bytes) : IProp GF) ⊣⊢
      iprop(⌜slice.mk dst (W64 20) (W64 20) = slice.nil ∧ bytes = []⌝ ∨
        (dst ↦{dq} (array.mk 20 bytes) ∗ ⌜(20 : Int) ≤ 20⌝))
    constructor
    · iintro Hdst
      iright
      iframe Hdst
      ipureintro
      decide
    · iintro (%Hnil | ⟨Hdst, %_⟩)
      · have impossible := congrArg GoSlice.len Hnil.1
        have h : (20 : Int) = 0 := congrArg sint.Z impossible
        omega
      · iexact Hdst

/-- Named-array slicing follows from the existing array rule. The tagged
    wrapper only makes that checked rule available to underlying-type dispatch. -/
theorem address_slice_step (dst : Loc) (low high : w64) :
    ⟦Slice l1.eth.Address.ty, (#dst, #low, #high)⟧ ⤳
      (if 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ 20 then
        #(slice.mk (arrayIndexRef w8 (sint.Z low) dst) (high - low) (W64 20 - low))
      else Panic "slice bounds out of range") := by
  letI : ⟦Slice (go.ArrayType 20 go.byte), (#dst, #low, #high)⟧ ⤳[under]
      (if 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ 20 then
        #(slice.mk (arrayIndexRef w8 (sint.Z low) dst) (high - low) (W64 20 - low))
      else Panic "slice bounds out of range") := ⟨go.slice_array_step 20 go.byte dst low high⟩
  exact go.slice_underlying _ _

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.address_slice_owned

#print axioms Perennial.github_com.NethermindEth.juno.core.address_slice_step
