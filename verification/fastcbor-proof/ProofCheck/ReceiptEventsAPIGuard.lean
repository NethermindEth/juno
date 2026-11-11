import ProofCheck.ReceiptEventsAPIOutcome

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
theorem wp_decodeReceiptEventsFirst_nonzero (dst : Loc)
    (old : ReceiptEventsValue) (input : GoSlice) (category : GoInterface)
    (nonzero : old ≠ receiptEventsZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ ReceiptEventsOwned dst old }}
      (App (App (Val (@! decodeReceiptEventsProjectionFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       ReceiptEventsOwned dst old ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  all_goals
    wp_start as ⟨#Hcategory, Howned⟩
    wp_auto
    wp_apply wp_zeroReceiptEvents_owned (GF := GF) dst old $$ [Howned] as %guard ⟨Howned, %Hzero⟩
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptEventsFirst_nonzero
