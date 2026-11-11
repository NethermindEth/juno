import ProofCheck.ReceiptStatusAPIGuard

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

/-- The actual receipt status First function on initially zero destinations and every
    input. Success aliases the original input suffix; every parser failure,
    including the bare tag sentinel, runs the real whole-destination cleanup. -/
theorem wp_decodeReceiptStatusFirst_zero (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptStatusOwned dst receiptStatusZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! decodeReceiptExecutionStatusProjectionFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptStatusFirstResult,
       ⌜ReceiptStatusFirstSpec receiptStatusZero wire outcome⌝ ∗
       ReceiptStatusFirstResultOwned dst input remaining wire result outcome }} := by
  all_goals
    wp_start as ⟨Howned, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
    have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
    have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
      simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
    have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
    wp_auto
    wp_apply wp_zeroReceiptStatus_owned (GF := GF) dst receiptStatusZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    · iframe Howned
    have hg : guard = true := Hzero.mpr rfl
    subst guard
    wp_auto
    wp_apply wp_newCursor input
    wp_apply wp_readReceiptStatus_record_complete (GF := GF) validator 0 c_ptr dst receiptStatusZero
      {(zero_val cursor.Cursor) with buf' := input} wire wire dq rfl (by decide) hp hb
      $$ [c Howned Hs] as %result ⟨Hs, %kinds, %outcome, #Hglobals, %Hspec, Houtcome⟩
    · iframe c Howned Hs
    cases outcome with
    | success final rest =>
      isimp only [ReceiptStatusRecordOutcome] at Houtcome
      icases Houtcome with ⟨%consumed, Howned, Hc, %Hnext⟩
      simp only [hz,
        show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hnext
      obtain ⟨hr, hprogress, hbound, hsuffix, hlength⟩ := Hnext
      subst result
      isimp only [hz] at Hc
      wp_auto
      wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 consumed}
        ⟨by change 0 ≤ sint.Z (W64 consumed); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
      · iframe Hc
      iapply HΦ
      iframe Hs
      iexists ReceiptStatusFirstResult.success final rest
      isplitl []
      · ipureintro; exact .success Hspec
      isimp only [ReceiptStatusFirstResultOwned]
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
      isimp only [ReceiptStatusRecordOutcome] at Houtcome
      icases Houtcome with ⟨Howned, Hc, Herr, %Hnext⟩
      isimp only [hz] at Herr
      ihave %HnonNil := receiptStatusRecordError_nonNil result (W64 offset) reason kinds $$ Herr
      cases returnShape : result with
      | nil => exact False.elim (HnonNil returnShape)
      | ok info =>
        wp_auto
        wp_apply wp_clearReceiptStatus_owned (GF := GF) dst retained $$ [Howned] with Howned
        · iframe Howned
        iapply HΦ
        iframe Hs
        iexists ReceiptStatusFirstResult.failure receiptStatusZero (.decode offset reason)
        isplitl []
        · ipureintro; exact .rejected Hspec
        isimp only [ReceiptStatusFirstResultOwned, ReceiptStatusAPIErrorOwned]
        iframe Howned
        isplitl [Herr]
        · iexists kinds
          iframe Hglobals Herr
          ipureintro
          simpa only [hz,
            show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] using Hnext.2.2.1
        ipureintro
        trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptStatusFirst_zero
