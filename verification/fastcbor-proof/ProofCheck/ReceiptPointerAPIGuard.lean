import ProofCheck.ReceiptPointerAPIOutcome

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
theorem wp_decodeReceiptPointerFirst_nonzero (dst : Loc)
    (old : Option ReceiptValue) (input : GoSlice) (category : GoInterface)
    (nonzero : old ≠ none) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       globalAddr cborerr.ErrUnsupported ↦□ category ∗ ReceiptPointerDestination dst old }}
      (App (App (Val (@! DecodeTransactionReceiptPointerFirst)) (Val #input)) (Val #dst))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err));
       ReceiptPointerDestination dst old ∗
       err ↦ {(zero_val cborerr.DecodeError) with Kind' := category, Detail' := go!"the destination is not the zero value"} }} := by
  all_goals
    wp_start as ⟨#Hcategory, Howned⟩
    isimp only [ReceiptPointerDestination] at Howned
    icases Howned with ⟨%ptr, Hdst, Hchild⟩
    ihave %Hnull := receiptPointerOwned_null_iff ptr old $$ Hchild
    have hn : ptr ≠ null := fun eq => nonzero (Hnull.mp eq)
    wp_auto
    simp only [hn, _root_.decide_true]
    wp_auto
    wp_alloc err as Herr
    wp_auto
    iapply HΦ
    iframe Herr
    unfold ReceiptPointerDestination
    iexists ptr
    iframe Hdst Hchild

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptPointerFirst_nonzero
