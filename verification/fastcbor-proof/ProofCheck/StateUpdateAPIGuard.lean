import ProofCheck.StateUpdateGuard

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

/-- The actual First guard rejects a nonzero owned destination unchanged,
    returns nil remaining bytes and allocates the precise entry diagnostic.
    The input is never accessed and no UTF-8 premise is needed. -/
theorem wp_decodeStateUpdateFirst_nonzero (dst : Loc)
    (old : StateUpdateValue) (input : GoSlice) (category : GoInterface)
    (nonzero : old ≠ stateUpdateZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ StateUpdateOwned dst old }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       StateUpdateOwned dst old ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  all_goals
    wp_start as ⟨#Hcategory, Howned⟩
    wp_auto
    wp_apply wp_zeroStateUpdate_owned (GF := GF) dst old $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    · iframe Howned
    have hfalse : guard = false := by
      cases guard
      · rfl
      · exact False.elim (nonzero (Hzero.mp rfl))
    subst guard
    wp_auto
    wp_alloc err as Herr
    wp_auto
    wp_end

/-- The full API also preserves every owned nonzero destination and returns
    the exact guard diagnostic without reading the supplied input. -/
theorem wp_decodeStateUpdate_nonzero (dst : Loc)
    (old : StateUpdateValue) (input : GoSlice) (category : GoInterface)
    (nonzero : old ≠ stateUpdateZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ StateUpdateOwned dst old }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  wp_start as ⟨#Hcategory, Howned⟩
  wp_auto
  wp_apply wp_decodeStateUpdateFirst_nonzero dst old input category nonzero $$ [Howned] as %err ⟨Howned, Herr⟩
  · iframe Hcategory Howned
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_nonzero

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_nonzero
