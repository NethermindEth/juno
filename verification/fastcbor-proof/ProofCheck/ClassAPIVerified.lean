import ProofCheck.ClassAPIComplete
import ProofCheck.ClassAPIFull
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
theorem wp_decodeClassFirst_verified
    (dst : Loc) (old : GoInterface)
    (input : GoSlice) (wire : List w8) (dq : DFrac)
    (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ old ∗ input ↦*{dq} wire }}
      (App (App (App (Val (@! DecodeClassDefinitionFirst)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ClassFirstResult,
       ⌜ClassFirstSpec tags old wire outcome⌝ ∗
       ClassFirstResultOwned dst input remaining wire result outcome }} :=
  wp_decodeClassFirst_complete utf8ValidatorContract_proved dst old input wire dq tags distinct

/-- Complete production API contract with the UTF-8 dependency discharged.
    Partial correctness under the pinned 64-bit model and the stated ownership,
    initialization and schema preconditions; no helper correctness premise. -/
theorem wp_decodeClass_verified
    (dst : Loc) (old : GoInterface) (input : GoSlice) (wire : List w8) (dq : DFrac)
    (tags : CBORClassTags) (distinct : ClassTagsDistinct tags) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       dst ↦ old ∗ input ↦*{dq} wire }}
      (App (App (App (Val (@! DecodeClassDefinition)) (Val #input)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : ClassDecodeResult,
       ⌜ClassFullSpec tags old wire outcome⌝ ∗ ClassFullResultOwned dst result outcome }} :=
  wp_decodeClass_complete utf8ValidatorContract_proved dst old input wire dq tags distinct

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeClassFirst_verified
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeClass_verified
