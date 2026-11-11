import ProofCheck.StateUpdateAPIOutcome
import ProofCheck.StateUpdateAPIGuard

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

/-- The actual public guard satisfies the independent API contract for every
    nonzero owned destination and input; initialized sentinels are derived from
    package initialization. This covers the guard, not successful decoding. -/
theorem wp_decodeStateUpdateFirst_nonzero_spec (dst : Loc) (old : StateUpdateValue)
    (input : GoSlice) (wire : List w8) (dq : DFrac) (nonzero : old ≠ stateUpdateZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : StateUpdateFirstResult,
       ⌜StateUpdateFirstSpec old wire outcome⌝ ∗ StateUpdateFirstResultOwned dst input remaining wire result outcome }} := by
  wp_start_folded as ⟨Howned, Hs⟩
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨⟨%category, #Hcategory, _⟩, _⟩
  wp_apply wp_decodeStateUpdateFirst_nonzero dst old input _ nonzero
    $$ [Howned] as %err ⟨Howned, Herr⟩
  · iframe Hcategory Howned
  iapply HΦ
  iframe Hs
  iexists StateUpdateFirstResult.failure old .nonzero
  isplitl []
  · ipureintro; exact .nonzero nonzero
  isimp only [StateUpdateFirstResultOwned, StateUpdateAPIErrorOwned]
  iframe Howned
  isplitl [Herr]
  · iexists _, err
    iframe Hcategory Herr
    ipureintro; trivial
  ipureintro; trivial

/-- The actual public guard satisfies the independent API contract for every
    nonzero owned destination and input; initialized sentinels are derived from
    package initialization. This covers the guard, not successful decoding. -/
theorem wp_decodeStateUpdate_nonzero_spec (dst : Loc) (old : StateUpdateValue)
    (input : GoSlice) (wire : List w8) (dq : DFrac) (nonzero : old ≠ stateUpdateZero) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : StateUpdateDecodeResult,
       ⌜StateUpdateFullSpec old wire outcome⌝ ∗ StateUpdateFullResultOwned dst result outcome }} := by
  wp_start_folded as ⟨Howned, Hs⟩
  ihave Hobjects := entryErrors_initialized_objects (GF := GF) $$ []
  · solve_pkg_init
  isimp only [EntryErrorsInitialized, errors.newVar] at Hobjects
  icases Hobjects with ⟨⟨%category, #Hcategory, _⟩, _⟩
  wp_apply wp_decodeStateUpdate_nonzero dst old input _ nonzero
    $$ [Howned] as %err ⟨Howned, Herr⟩
  · iframe Hcategory Howned
  iapply HΦ
  iframe Hs
  iexists StateUpdateDecodeResult.failure old .nonzero
  isplitl []
  · ipureintro; exact .firstError (.nonzero nonzero)
  isimp only [StateUpdateFullResultOwned, StateUpdateAPIErrorOwned]
  iframe Howned
  iexists _, err
  iframe Hcategory Herr
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_nonzero_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_nonzero_spec
