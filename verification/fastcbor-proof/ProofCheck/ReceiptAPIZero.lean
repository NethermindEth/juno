import ProofCheck.ReceiptAPIGuard

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

/-- The actual receipt First function on initially zero destinations and every
    input. Success aliases the original input suffix; every parser failure,
    including the bare FeeUnit overflow sentinel, runs the real whole-destination cleanup. -/
theorem wp_decodeReceiptFirst_zero (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptOwned dst receiptZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeTransactionReceiptFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptFirstResult,
       ⌜ReceiptFirstSpec receiptZero wire outcome⌝ ∗
       ReceiptFirstResultOwned dst input remaining wire result outcome }} := by
  all_goals
    wp_start as ⟨Howned, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
    have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
    have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
      simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
    have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
    wp_auto
    wp_apply wp_zeroReceipt_owned (GF := GF) dst receiptZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    · iframe Howned
    have hg : guard = true := Hzero.mpr rfl
    subst guard
    wp_auto
    wp_apply wp_newCursor input
    rcases receiptRecord_classify 0 receiptZero wire (by decide) with
      ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · wp_apply wp_readReceipt_record_valid validator c_ptr dst receiptZero final
        {(zero_val cursor.Cursor) with buf' := input} wire item rest dq 0 valid rfl hp (hb.trans split)
        $$ [c Howned Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
      · iframe c Howned Hs
      simp only [hz,
        show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hnext
      obtain ⟨hprogress, hbound, hsuffix⟩ := Hnext
      isimp only [hz] at Hc
      wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 item.length}
        ⟨by change 0 ≤ sint.Z (W64 item.length); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
      · iframe Hc
      iapply HΦ
      iframe Hs
      iexists ReceiptFirstResult.success final rest
      isplitl []
      · ipureintro; exact .success ⟨item, split, valid⟩
      isimp only [ReceiptFirstResultOwned]
      iframe Howned
      iexists item.length
      ipureintro
      refine ⟨trivial, rfl, hprogress, by rw [split, List.length_append], ?_, hsuffix⟩
      simp only [slice.slice]
      have hlen : (wire.length : Int) = sint.Z input.len := by
        simp only [sint.nat, sint.Z] at Hlen ⊢
        omega
      have hlength : item.length + rest.length = wire.length := by rw [split, List.length_append]
      word
    · wp_apply wp_readReceipt_record_rejected validator c_ptr dst receiptZero retained
        {(zero_val cursor.Cursor) with buf' := input} wire wire dq 0 consumed offset extraDepth reason failure rfl hp hb
        $$ [c Howned Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
      · iframe c Howned Hs
      isimp only [hz] at Herr
      ihave %HnonNil := receiptFailureOwned_nonNil result (W64 offset) reason $$ Herr
      cases returnShape : result with
      | nil => exact False.elim (HnonNil returnShape)
      | ok info =>
        wp_auto
        wp_apply wp_clearReceipt_owned (GF := GF) dst retained $$ [Howned] with Howned
        · iframe Howned
        iapply HΦ
        iframe Hs
        iexists ReceiptFirstResult.failure receiptZero (.decode offset reason)
        isplitl []
        · ipureintro; exact .rejected failure
        isimp only [ReceiptFirstResultOwned, ReceiptAPIErrorOwned]
        iframe Howned Herr
        ipureintro
        simpa only [hz,
          show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] using Hnext.2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptFirst_zero
