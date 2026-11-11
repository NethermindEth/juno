import ProofCheck.ReceiptEventsAPIZero

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

/-- Complete caller-facing contract for the registered receipt events
    projection First entry: every input and every owned initial destination,
    exact suffix aliasing, owned output/errors, unchanged input and actual
    cleanup. The shared UTF-8 validator premise remains explicit. -/
theorem wp_decodeReceiptEventsFirst_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : ReceiptEventsValue)
    (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       ReceiptEventsOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! decodeReceiptEventsProjectionFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptEventsFirstResult,
       ⌜ReceiptEventsFirstSpec old wire outcome⌝ ∗
       ReceiptEventsFirstResultOwned dst input remaining wire result outcome }} := by
  wp_start_folded as ⟨Howned, Hs⟩
  by_cases zero : old = receiptEventsZero
  · subst old
    wp_apply wp_decodeReceiptEventsFirst_zero validator dst input wire dq
      $$ [Howned Hs] as %remaining %result ⟨Hs, Houtcome⟩
    · iframe Howned Hs
    wp_end
  · ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
    · solve_pkg_init
    isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
    icases Hobjects with ⟨⟨%unsupported, #Hcategory, _⟩, _⟩
    wp_apply wp_decodeReceiptEventsFirst_nonzero dst old input _ zero
      $$ [Howned] as %err ⟨Howned, Herr⟩
    · iframe Hcategory Howned
    iapply HΦ
    iframe Hs
    iexists ReceiptEventsFirstResult.failure old .nonzero
    isplitl []
    · ipureintro; exact .nonzero zero
    isimp only [ReceiptEventsFirstResultOwned, ReceiptEventsAPIErrorOwned]
    iframe Howned
    isplitl [Herr]
    · iexists _, err
      iframe Hcategory Herr
      ipureintro; trivial
    ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptEventsFirst_complete
