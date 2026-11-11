import ProofCheck.HeaderProjectionAPIGuard

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 12800000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- All five actual First functions on initially zero destinations and every
    input. Success aliases the original input suffix; every parser failure,
    including the bare tag sentinel, runs the real whole-destination cleanup. -/
theorem wp_decodeHeaderProjectionFirst_zero (validator : Utf8ValidatorContract (GF := GF))
    (kind : HeaderProjectionKind) (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       HeaderProjectionOwned kind dst headerProjectionZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! (headerProjectionFirstName kind))) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : HeaderProjectionFirstResult,
       ⌜HeaderProjectionFirstSpec kind headerProjectionZero wire outcome⌝ ∗
       HeaderProjectionFirstResultOwned kind dst input remaining wire result outcome }} := by
  cases kind <;> simp only [headerProjectionFirstName]
  all_goals
    wp_start as ⟨Howned, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
    have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
    have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
      simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
    have hz (x : w64) : (0 : w64) + x = x := by word
    wp_auto
    first
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .hash dst headerProjectionZero
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .root dst headerProjectionZero
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .count dst headerProjectionZero
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .timestamp dst headerProjectionZero
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    | have hguard := wp_zeroHeaderProjection_owned (GF := GF) .hashAndRoot dst headerProjectionZero
      simp only [headerProjectionZeroName] at hguard
      wp_apply hguard $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    all_goals try (iframe Howned)
    have hg : guard = true := Hzero.mpr trivial
    subst guard
    wp_auto
    wp_apply wp_newCursor input
    first
    | have hread := wp_readHeaderProjection_record_complete (GF := GF) validator 0 .hash c_ptr dst headerProjectionZero
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      simp only [headerProjectionReadName] at hread
      wp_apply hread $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    | have hread := wp_readHeaderProjection_record_complete (GF := GF) validator 0 .root c_ptr dst headerProjectionZero
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      simp only [headerProjectionReadName] at hread
      wp_apply hread $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    | have hread := wp_readHeaderProjection_record_complete (GF := GF) validator 0 .count c_ptr dst headerProjectionZero
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      simp only [headerProjectionReadName] at hread
      wp_apply hread $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    | have hread := wp_readHeaderProjection_record_complete (GF := GF) validator 0 .timestamp c_ptr dst headerProjectionZero
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      simp only [headerProjectionReadName] at hread
      wp_apply hread $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    | have hread := wp_readHeaderProjection_record_complete (GF := GF) validator 0 .hashAndRoot c_ptr dst headerProjectionZero
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      simp only [headerProjectionReadName] at hread
      wp_apply hread $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    all_goals try (iframe c Howned Hs)
    cases outcome with
    | success final rest =>
      isimp only [HeaderProjectionRecordOutcome] at Houtcome
      icases Houtcome with ⟨%consumed, Howned, Hc, %Hnext⟩
      simp only [hz,
        show sint.Z (0 : w64) = 0 by rfl, Int.zero_add] at Hnext
      obtain ⟨hr, hprogress, hbound, hsuffix, hlength⟩ := Hnext
      subst result
      isimp only [hz] at Hc
      wp_auto
      wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 consumed}
        ⟨by change 0 ≤ sint.Z (W64 consumed); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
      · iframe Hc
      iapply HΦ
      iframe Hs
      iexists HeaderProjectionFirstResult.success final rest
      isplitl []
      · ipureintro; exact .success Hspec
      isimp only [HeaderProjectionFirstResultOwned]
      iframe Howned
      iexists consumed
      ipureintro
      refine ⟨trivial, rfl, hprogress, hlength, ?_, hsuffix⟩
      simp only [slice.slice]
      have hlen : (wire.length : Int) = sint.Z input.len := by
        simp only [sint.nat, sint.Z] at Hlen ⊢
        omega
      word
    | failure retained consumed offset entered reason =>
      isimp only [HeaderProjectionRecordOutcome] at Houtcome
      icases Houtcome with ⟨Howned, Hc, Herr, %Hnext⟩
      isimp only [hz] at Herr
      ihave %HnonNil := headerProjectionRecordError_nonNil result (W64 offset) reason kinds $$ Herr
      cases returnShape : result with
      | nil => exact False.elim (HnonNil returnShape)
      | ok info =>
        wp_auto
        first
        | have hclear := wp_clearHeaderProjection_owned (GF := GF) .hash dst retained
          simp only [headerProjectionClearName] at hclear
          wp_apply hclear $$ [Howned] with Howned
        | have hclear := wp_clearHeaderProjection_owned (GF := GF) .root dst retained
          simp only [headerProjectionClearName] at hclear
          wp_apply hclear $$ [Howned] with Howned
        | have hclear := wp_clearHeaderProjection_owned (GF := GF) .count dst retained
          simp only [headerProjectionClearName] at hclear
          wp_apply hclear $$ [Howned] with Howned
        | have hclear := wp_clearHeaderProjection_owned (GF := GF) .timestamp dst retained
          simp only [headerProjectionClearName] at hclear
          wp_apply hclear $$ [Howned] with Howned
        | have hclear := wp_clearHeaderProjection_owned (GF := GF) .hashAndRoot dst retained
          simp only [headerProjectionClearName] at hclear
          wp_apply hclear $$ [Howned] with Howned
        all_goals try (iframe Howned)
        iapply HΦ
        iframe Hs
        iexists HeaderProjectionFirstResult.failure headerProjectionZero (.decode offset reason)
        isplitl []
        · ipureintro; exact .rejected Hspec
        isimp only [HeaderProjectionFirstResultOwned, HeaderProjectionAPIErrorOwned]
        iframe Howned
        isplitl [Herr]
        · iexists kinds
          iframe Hglobals Herr
          ipureintro
          simpa only [show (zero_val cursor.Cursor).pos' = (0 : w64) by rfl, hz,
            show sint.Z (0 : w64) = 0 by rfl, Int.zero_add] using Hnext.2.2.1
        ipureintro
        trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderProjectionFirst_zero
