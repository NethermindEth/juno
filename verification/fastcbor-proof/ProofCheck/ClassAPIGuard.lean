import ProofCheck.ClassAPIOutcome

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

/-- The guard rejects every nonnil interface, including a typed nil pointer,
    without dereferencing its payload or accessing input bytes. -/
theorem wp_decodeClassFirst_nonzero (dst : Loc) (old : GoInterface)
    (input : GoSlice) (tags : CBORClassTags) (category : GoInterface) (nonzero : old ≠ interface.nil) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ dst ↦ old }}
      (App (App (App (Val (@! DecodeClassDefinitionFirst)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       dst ↦ old ∗ err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  wp_start as ⟨#Hcategory, Hdst⟩
  cases old with
  | nil => exact False.elim (nonzero rfl)
  | ok info =>
    wp_auto
    wp_alloc err as Herr
    wp_auto
    iapply HΦ
    iframe Hdst Herr

/-- Actual interface cleanup only clears the destination cell. Existing payload
    ownership can be retained by the caller or forgotten by the GC model. -/
theorem wp_clearClass (dst : Loc) (old : GoInterface) :
    {{ (dst ↦ old : IProp GF) }}
      (App (Val (@! cborClearClassDefinition)) (Val #dst))
    {{ RET #(); dst ↦ interface.nil }} := by
  wp_start as Hdst
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeClassFirst_nonzero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearClass
