import ProofCheck.ReceiptPointerAPIGuard

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
theorem wp_decodeReceiptPointerFirst_zero (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptPointerDestination dst none ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeTransactionReceiptPointerFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : ReceiptPointerFirstResult,
       ⌜ReceiptPointerFirstSpec none wire outcome⌝ ∗
       ReceiptPointerFirstResultOwned dst input remaining wire result outcome }} := by
  all_goals
    wp_start as ⟨Howned, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
    have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
    have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
      simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
    have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
    isimp only [ReceiptPointerDestination, ReceiptPointerOwned] at Howned
    icases Howned with ⟨%ptr, Hdst, %Hnull⟩
    subst ptr
    wp_auto
    wp_apply wp_newCursor input
    rcases receiptPointer_classify 0 wire (by decide) with
      ⟨item, rest, final, split, valid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
    · wp_apply wp_readReceiptPointer_valid validator c_ptr dst null
        {(zero_val cursor.Cursor) with buf' := input} wire item rest final dq 0 valid rfl hp (hb.trans split)
        $$ [c Hdst Hs] as %out ⟨Hdst, Hchild, Hc, Hs, %Hnext⟩
      · iframe c Hdst Hs
      simp only [hz,
        show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hnext
      obtain ⟨hprogress, hbound, hsuffix⟩ := Hnext
      isimp only [hz] at Hc
      wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 item.length}
        ⟨by change 0 ≤ sint.Z (W64 item.length); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
      · iframe Hc
      iapply HΦ
      iframe Hs
      iexists ReceiptPointerFirstResult.success final rest
      isplitl []
      · ipureintro; exact .success ⟨item, split, valid⟩
      isimp only [ReceiptPointerFirstResultOwned]
      isplitl [Hdst Hchild]
      · unfold ReceiptPointerDestination
        iexists out
        iframe Hdst Hchild
      iexists item.length
      ipureintro
      refine ⟨trivial, rfl, hprogress, by rw [split, List.length_append], ?_, hsuffix⟩
      simp only [slice.slice]
      have hlen : (wire.length : Int) = sint.Z input.len := by
        simp only [sint.nat, sint.Z] at Hlen ⊢
        omega
      have hlength : item.length + rest.length = wire.length := by rw [split, List.length_append]
      word
    · wp_apply wp_readReceiptPointer_rejected validator c_ptr dst null
        {(zero_val cursor.Cursor) with buf' := input} wire wire retained dq 0 consumed offset extraDepth reason failure rfl hp hb
        $$ [c Hdst Hs] as %result ⟨Hdst, _, Hc, Hs, Herr, %Hnext⟩
      · iframe c Hdst Hs
      isimp only [hz] at Herr
      ihave %HnonNil := receiptFailureOwned_nonNil result (W64 offset) reason $$ Herr
      cases returnShape : result with
      | nil => exact False.elim (HnonNil returnShape)
      | ok info =>
        wp_auto
        ihave Howned : (ReceiptPointerDestination dst none : IProp GF) $$ [Hdst]
        · unfold ReceiptPointerDestination
          iexists null
          iframe Hdst
          isimp only [ReceiptPointerOwned]
          ipureintro; trivial
        wp_apply wp_clearReceiptPointer_owned (GF := GF) dst none $$ [Howned] with Howned
        · iframe Howned
        iapply HΦ
        iframe Hs
        iexists ReceiptPointerFirstResult.failure none (.decode offset reason)
        isplitl []
        · ipureintro; exact .rejected failure
        isimp only [ReceiptPointerFirstResultOwned, ReceiptPointerAPIErrorOwned]
        iframe Howned Herr
        ipureintro
        simpa only [hz,
          show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] using Hnext.2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeReceiptPointerFirst_zero
