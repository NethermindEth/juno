import ProofCheck.TransactionAPIComplete
import ProofCheck.TransactionAPIFull
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
theorem wp_decodeTransactionFirst_verified
    (dst : Loc) (old : GoInterface)
    (input : GoSlice) (wire : List w8) (dq : DFrac)
    (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ old ∗ input ↦*{dq} wire }}
      (App (App (App (Val (@! DecodeTransactionFirst)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : TransactionFirstResult,
       ⌜TransactionFirstSpec tags old wire outcome⌝ ∗
       TransactionFirstResultOwned dst input remaining wire result outcome }} :=
  wp_decodeTransactionFirst_complete utf8ValidatorContract_proved dst old input wire dq tags distinct

/-- Complete production API contract with the UTF-8 dependency discharged.
    Partial correctness under the pinned 64-bit model and the stated ownership,
    initialization and schema preconditions; no helper correctness premise. -/
theorem wp_decodeTransaction_verified
    (dst : Loc) (old : GoInterface) (input : GoSlice) (wire : List w8) (dq : DFrac)
    (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ old ∗ input ↦*{dq} wire }}
      (App (App (App (Val (@! DecodeTransaction)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : TransactionDecodeResult,
       ⌜TransactionFullSpec tags old wire outcome⌝ ∗ TransactionFullResultOwned dst result outcome }} :=
  wp_decodeTransaction_complete utf8ValidatorContract_proved dst old input wire dq tags distinct

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeTransactionFirst_verified
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeTransaction_verified
