import ProofCheck.StateUpdateAPIGuardSpec

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

/-- Every independently specified opening failure runs the actual First
    wrapper, including cursor construction and cleanup. Input is unchanged,
    the returned suffix is nil and the diagnostic has exact offset zero.
    This does not assume correctness of a reader or its helpers. -/
theorem wp_decodeStateUpdateFirst_open_rejected_spec (dst : Loc) (input : GoSlice)
    (wire : List w8) (dq : DFrac) (reason : FeltFailureReason)
    (failure : StructOpenFailure 0 wire reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗
       ⌜StateUpdateFirstSpec stateUpdateZero wire (.failure stateUpdateZero (.decode 0 (.opening reason)))⌝ ∗
       StateUpdateFirstResultOwned dst input remaining wire result (.failure stateUpdateZero (.decode 0 (.opening reason))) }} := by
  wp_start as ⟨Howned, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  wp_auto
  wp_apply wp_zeroStateUpdate_owned (GF := GF) dst stateUpdateZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
  · iframe Howned
  have hg : guard = true := Hzero.mpr rfl
  subst guard
  wp_auto
  wp_apply wp_newCursor input
  wp_apply wp_readStateUpdate_open_rejected_owned c_ptr dst stateUpdateZero
    {(zero_val cursor.Cursor) with buf' := input} wire wire dq 0 reason failure rfl hp hb
    $$ [c Howned Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
  · iframe c Howned Hs
  isimp only [show (zero_val cursor.Cursor).pos' = W64 0 by rfl] at Herr
  wp_apply wp_clearStateUpdate_owned (GF := GF) dst stateUpdateZero $$ [Howned] with Howned
  · iframe Howned
  iapply HΦ
  iframe Hs
  isplitl []
  · ipureintro; exact .rejected (.opening failure)
  isimp only [StateUpdateFirstResultOwned, StateUpdateAPIErrorOwned]
  iframe Howned
  isplitl [Herr]
  · iexists err
    isplitl [Herr]
    · iexact Herr
    ipureintro; exact ⟨rfl, rfl⟩
  ipureintro; trivial

/-- Full decoding forwards every opening diagnostic from First and retains
    its cleared output and unchanged input, with the independent full spec. -/
theorem wp_decodeStateUpdate_open_rejected_spec (dst : Loc) (input : GoSlice)
    (wire : List w8) (dq : DFrac) (reason : FeltFailureReason)
    (failure : StructOpenFailure 0 wire reason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdate)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗
       ⌜StateUpdateFullSpec stateUpdateZero wire (.failure stateUpdateZero (.decode 0 (.opening reason)))⌝ ∗
       StateUpdateFullResultOwned dst result (.failure stateUpdateZero (.decode 0 (.opening reason))) }} := by
  wp_start as ⟨Howned, Hs⟩
  wp_auto
  wp_apply wp_decodeStateUpdateFirst_open_rejected_spec dst input wire dq reason failure
    $$ [Howned Hs] as %remaining %result ⟨Hs, %Hspec, Houtcome⟩
  · iframe Howned Hs
  isimp only [StateUpdateFirstResultOwned] at Houtcome
  icases Houtcome with ⟨Howned, Herr, %Hremaining⟩
  ihave %HnonNil := stateUpdateAPIErrorOwned_nonNil result (.decode 0 (.opening reason)) $$ Herr
  cases returnShape : result with
  | nil => exact False.elim (HnonNil returnShape)
  | ok info =>
    wp_auto
    iapply HΦ
    iframe Hs
    isplitl []
    · ipureintro; exact .firstError Hspec
    isimp only [StateUpdateFullResultOwned]
    iframe Howned Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_open_rejected_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdate_open_rejected_spec
