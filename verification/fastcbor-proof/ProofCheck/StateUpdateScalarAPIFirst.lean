import ProofCheck.StateUpdateScalarRecord
import ProofCheck.StateUpdateAPIOutcome
import ProofCheck.StateUpdateGuard

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

/-- Actual First decoding of every independently valid record that does not
    select StateDiff. Arbitrary suffixes, felt contents, supported unknown values
    and key order are covered; UTF-8 correctness remains explicit. -/
theorem wp_decodeStateUpdateFirst_scalar_valid (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (input : GoSlice) (wire item rest : List w8) (final : StateUpdateValue) (dq : DFrac)
    (split : wire = item ++ rest) (valid : StateUpdateScalarRecordValid 0 stateUpdateZero item rest final) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗
       ⌜StateUpdateFirstSpec stateUpdateZero wire (.success final rest)⌝ ∗
       StateUpdateFirstResultOwned dst input remaining wire result (.success final rest) }} := by
  all_goals
    wp_start as ⟨Howned, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
    have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
    have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) wire = wire := by
      simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
    have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
    wp_auto
    wp_apply wp_zeroStateUpdate_owned (GF := GF) dst stateUpdateZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
    · iframe Howned
    have hg : guard = true := Hzero.mpr rfl
    subst guard
    wp_auto
    wp_apply wp_newCursor input
    wp_apply wp_readStateUpdate_scalar_record_valid validator c_ptr dst stateUpdateZero final
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
    isplitl []
    · ipureintro; exact .success ⟨item, split, stateUpdateScalarRecord_valid _ _ _ _ _ valid⟩
    isimp only [StateUpdateFirstResultOwned]
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_scalar_valid
