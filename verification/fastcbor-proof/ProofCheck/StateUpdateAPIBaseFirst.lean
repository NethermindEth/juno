import ProofCheck.StateUpdateAPIOpenFailure
import ProofCheck.StateUpdateEmpty

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

/-- Actual First decoding of null and undefined, with an arbitrary suffix.
    The result satisfies the independent API spec and aliases the exact unread
    input slice, preserving its storage and all zero destination fields. -/
theorem wp_decodeStateUpdateFirst_nullish_spec (dst : Loc) (input : GoSlice) (head : w8)
    (rest : List w8) (dq : DFrac) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} (head :: rest) }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} (head :: rest) ∗
       ⌜StateUpdateFirstSpec stateUpdateZero (head :: rest) (.success stateUpdateZero rest)⌝ ∗
       StateUpdateFirstResultOwned dst input remaining (head :: rest) result (.success stateUpdateZero rest) }} := by
  wp_start as ⟨Howned, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) (head :: rest) = (head :: rest) := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
  wp_auto
  wp_apply wp_zeroStateUpdate_owned (GF := GF) dst stateUpdateZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
  · iframe Howned
  have hg : guard = true := Hzero.mpr rfl
  subst guard
  wp_auto
  wp_apply wp_newCursor input
  wp_apply wp_readStateUpdate_nullish c_ptr dst stateUpdateZero
    {(zero_val cursor.Cursor) with buf' := input} (head :: rest) rest head dq hn hp hb
    $$ [c Howned Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
  · iframe c Howned Hs
  simp only [hz, show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hnext
  obtain ⟨hprogress, hbound, hsuffix⟩ := Hnext
  isimp only [hz] at Hc
  wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 1}
    ⟨by change 0 ≤ sint.Z (W64 1); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ
  iframe Hs
  isplitl []
  · ipureintro
    exact .success ⟨[head], rfl, .nullish rest hn⟩
  isimp only [StateUpdateFirstResultOwned]
  iframe Howned
  iexists 1
  ipureintro
  refine ⟨trivial, rfl, ?_, ?_, ?_, hsuffix⟩
  · simpa only [Int.natCast_one] using hprogress
  · simp only [List.length_cons]; omega
  · simp only [slice.slice]
    have hlen : ((head :: rest).length : Int) = sint.Z input.len := by
      simp only [sint.nat, sint.Z] at Hlen ⊢
      omega
    simp only [List.length_cons] at hlen
    word

/-- Actual First decoding of all zero-count map widths, with an arbitrary suffix.
    The result satisfies the independent API spec and aliases the exact unread
    input slice, preserving its storage and all zero destination fields. -/
theorem wp_decodeStateUpdateFirst_empty_spec (dst : Loc) (input : GoSlice) (head : w8)
    (payload rest : List w8) (dq : DFrac) (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       StateUpdateOwned dst stateUpdateZero ∗ input ↦*{dq} ((head :: payload) ++ rest) }}
      (App (App (Val (@! DecodeStateUpdateFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} ((head :: payload) ++ rest) ∗
       ⌜StateUpdateFirstSpec stateUpdateZero ((head :: payload) ++ rest) (.success stateUpdateZero rest)⌝ ∗
       StateUpdateFirstResultOwned dst input remaining ((head :: payload) ++ rest) result (.success stateUpdateZero rest) }} := by
  wp_start as ⟨Howned, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hcap := ownSlice_wf _ _ _ $$ Hs
  have hp : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z input.len := ⟨by decide, Hlen.2⟩
  have hb : subslice (sint.nat (W64 0)) (sint.nat input.len) ((head :: payload) ++ rest) = ((head :: payload) ++ rest) := by
    simp only [show sint.nat (W64 0) = 0 by rfl, subslice, List.drop_zero, ← Hlen.1, List.take_length]
  have hz (x : w64) : (zero_val cursor.Cursor).pos' + x = x := by change (0 : w64) + x = x; word
  wp_auto
  wp_apply wp_zeroStateUpdate_owned (GF := GF) dst stateUpdateZero $$ [Howned] as %guard ⟨Howned, %Hzero⟩
  · iframe Howned
  have hg : guard = true := Hzero.mpr rfl
  subst guard
  wp_auto
  wp_apply wp_newCursor input
  wp_apply wp_readStateUpdate_empty c_ptr dst stateUpdateZero
    {(zero_val cursor.Cursor) with buf' := input} ((head :: payload) ++ rest) payload rest head dq 0 enc major rfl (by decide) hp hb
    $$ [c Howned Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
  · iframe c Howned Hs
  simp only [hz, show sint.Z (zero_val cursor.Cursor).pos' = 0 by rfl, Int.zero_add] at Hnext
  obtain ⟨hprogress, hbound, hsuffix⟩ := Hnext
  isimp only [hz] at Hc
  wp_apply wp_remaining c_ptr {(zero_val cursor.Cursor) with buf' := input, pos' := W64 (1 + payload.length)}
    ⟨by change 0 ≤ sint.Z (W64 (1 + payload.length)); rw [hprogress]; omega, hbound⟩ Hcap.2 $$ [Hc] with Hc
  · iframe Hc
  iapply HΦ
  iframe Hs
  isplitl []
  · ipureintro
    have valid : StateUpdateRecordValid 0 stateUpdateZero (head :: payload) rest stateUpdateZero := by
      simpa only [List.append_nil] using
        (StateUpdateRecordValid.map (d := 0) (old := stateUpdateZero) enc major
          (count := 0) (by rfl) (by decide) (.nil [] stateUpdateZero rest))
    exact .success ⟨head :: payload, rfl, valid⟩
  isimp only [StateUpdateFirstResultOwned]
  iframe Howned
  iexists (1 + payload.length)
  ipureintro
  refine ⟨trivial, rfl, ?_, ?_, ?_, hsuffix⟩
  · simpa only [Int.natCast_add, Int.natCast_one] using hprogress
  · simp only [List.length_append, List.length_cons]; omega
  · simp only [slice.slice]
    have hlen : (((head :: payload) ++ rest).length : Int) = sint.Z input.len := by
      simp only [sint.nat, sint.Z] at Hlen ⊢
      omega
    simp only [List.length_append, List.length_cons] at hlen
    word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_nullish_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeStateUpdateFirst_empty_spec
