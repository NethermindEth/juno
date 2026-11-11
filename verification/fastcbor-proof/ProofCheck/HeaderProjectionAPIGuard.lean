import ProofCheck.HeaderProjectionAPIOutcome

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

/-- Every actual First guard rejects a nonzero owned destination unchanged,
    returns nil remaining bytes and allocates the precise entry diagnostic.
    The input is never accessed and no UTF-8 premise is needed. -/
theorem wp_decodeHeaderProjectionFirst_nonzero (kind : HeaderProjectionKind) (dst : Loc)
    (old : HeaderProjectionValue) (input : GoSlice) (category : GoInterface)
    (nonzero : old ≠ headerProjectionZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ HeaderProjectionOwned kind dst old }}
      (App (App (Val (@! (headerProjectionFirstName kind))) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       HeaderProjectionOwned kind dst old ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  cases kind <;> simp only [headerProjectionFirstName]
  all_goals
    wp_start as ⟨#Hcategory, Howned⟩
    wp_auto
    first
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .hash dst old
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .root dst old
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .count dst old
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .timestamp dst old
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .hashAndRoot dst old
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    all_goals try (iframe Howned)
    have hfalse : guard = false := by
      cases guard
      · rfl
      · exact False.elim (nonzero (Hzero.mp rfl))
    subst guard
    wp_auto
    wp_alloc err as Herr
    wp_auto
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderProjectionFirst_nonzero
