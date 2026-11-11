import ProofCheck.ReceiptPointerComplete
import ProofCheck.ReceiptGuard

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def ReceiptPointerDestination (dst : Loc) (value : Option ReceiptValue) : IProp GF :=
  iprop(∃ ptr : Loc, dst ↦ ptr ∗ ReceiptPointerOwned ptr value)

theorem receiptPointerOwned_null_iff (ptr : Loc) (value : Option ReceiptValue) :
    ReceiptPointerOwned ptr value ⊢ (⌜ptr = null ↔ value = none⌝ : IProp GF) := by
  cases value with
  | none => isimp only [ReceiptPointerOwned]; iintro %h; ipureintro; simp [h]
  | some value =>
    isimp only [ReceiptPointerOwned, ReceiptOwned]
    iintro ⟨%h, Hdst, _⟩
    ihave %hn := typedPointsto_not_null ptr h (DFrac.own 1) $$ Hdst
    ipureintro; simp [hn]

theorem wp_clearReceiptPointer_owned (dst : Loc) (value : Option ReceiptValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       ReceiptPointerDestination dst value }}
      (App (Val (@! cborClearTransactionReceiptPointer)) (Val #dst))
    {{ RET #(); ReceiptPointerDestination dst none }} := by
  wp_start as Howned
  isimp only [ReceiptPointerDestination] at Howned
  icases Howned with ⟨%ptr, Hdst, _⟩
  wp_auto
  iapply HΦ
  unfold ReceiptPointerDestination
  iexists null
  iframe Hdst
  isimp only [ReceiptPointerOwned]
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptPointerOwned_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearReceiptPointer_owned
