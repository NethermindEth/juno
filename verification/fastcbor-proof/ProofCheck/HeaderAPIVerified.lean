import ProofCheck.HeaderAPIComplete
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
    Partial correctness under the pinned 64-bit model and its ownership and
    package-initialization preconditions; no helper correctness premise. -/
theorem wp_decodeHeaderFirst_verified
    (dst : Loc) (old : HeaderValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       HeaderOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : HeaderFirstResult,
       ⌜HeaderFirstSpec old wire outcome⌝ ∗ HeaderFirstResultOwned dst input remaining wire result outcome }} :=
  wp_decodeHeaderFirst_complete utf8ValidatorContract_proved dst old input wire dq

/-- Complete production API contract with the UTF-8 dependency discharged.
    Partial correctness under the pinned 64-bit model and its ownership and
    package-initialization preconditions; no helper correctness premise. -/
theorem wp_decodeHeader_verified
    (dst : Loc) (old : HeaderValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       HeaderOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : HeaderDecodeResult,
       ⌜HeaderFullSpec old wire outcome⌝ ∗ HeaderFullResultOwned dst result outcome }} :=
  wp_decodeHeader_complete utf8ValidatorContract_proved dst old input wire dq

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_verified
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_verified
