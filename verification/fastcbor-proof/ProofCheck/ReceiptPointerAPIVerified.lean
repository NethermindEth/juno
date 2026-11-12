import ProofCheck.ReceiptPointerAPIComplete
import ProofCheck.ReceiptPointerAPIFull
import ProofCheck.Utf8FromWord

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

/-- Complete production API contract with the UTF-8 dependency discharged.
    Partial correctness under the pinned 64-bit model and the stated ownership,
    initialization and schema preconditions; no helper correctness premise. -/
theorem wp_decodeReceiptPointerFirst_verified
    (dst : Loc) (old : Option ReceiptValue)
    (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       ReceiptPointerDestination dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeTransactionReceiptPointerFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptPointerFirstResult,
       ⌜ReceiptPointerFirstSpec old wire outcome⌝ ∗
       ReceiptPointerFirstResultOwned dst input remaining wire result outcome }} :=
  wp_decodeReceiptPointerFirst_complete utf8ValidatorContract_proved dst old input wire dq

/-- Complete production API contract with the UTF-8 dependency discharged.
    Partial correctness under the pinned 64-bit model and the stated ownership,
    initialization and schema preconditions; no helper correctness premise. -/
theorem wp_decodeReceiptPointer_verified
    (dst : Loc) (old : Option ReceiptValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       ReceiptPointerDestination dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeTransactionReceiptPointer)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptPointerDecodeResult,
       ⌜ReceiptPointerFullSpec old wire outcome⌝ ∗ ReceiptPointerFullResultOwned dst result outcome }} :=
  wp_decodeReceiptPointer_complete utf8ValidatorContract_proved dst old input wire dq

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptPointerFirst_verified
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptPointer_verified
