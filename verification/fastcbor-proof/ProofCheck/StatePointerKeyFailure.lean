import ProofCheck.StateRecordKeyFailure
import ProofCheck.StateUpdateDiffFields

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

/-- The wrapper retains the fresh zero child privately after its first-key
    rejection, preserving the caller's prior pointer and exact error position. -/
theorem wp_readStateDiffPointer_first_key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst old : Loc) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count : Nat) (reason : KeyFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ wire.length) (failure : KeyRejected wire reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ (∃ temporary : Loc, StateDiffOwned temporary stateDiffZero) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateDiffFailureOwned err (v.pos' + W64 (1 + payload.length)) (.key reason) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = wire ∧
         StateDiffPointerRejected d ((head :: payload) ++ wire) stateDiffZero
           (1 + payload.length) (1 + payload.length) 1 (.key reason)⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  have hn : ((head :: payload) ++ wire).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have heq := Option.some.inj eq
    subst head
    contradiction
  have hu : ((head :: payload) ++ wire).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have heq := Option.some.inj eq
    subst head
    contradiction
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ wire) dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  ihave Howned := stateDiffPointerOwned_zero «$r0_ptr» $$ «$r0»
  isimp only [StateDiffPointerOwned] at Howned
  wp_apply wp_readStateDiff_first_key_rejected validator c «$r0_ptr» stateDiffZero v xs payload wire head dq d count reason
    enc major positive fit failure hd depth hp hbytes
    $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
  · iframe Howned Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs Herr
  isplitl [Howned]
  · iexists «$r0_ptr»
    iexact Howned
  ipureintro
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, .record hn hu Hnext.2.2.2⟩

/-- Failure inside the selected StateDiff field preserves the entire old update,
    while owning the failed private child through the field error contract. -/
theorem wp_stateUpdate_diff_first_key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs payload wire : List w8) (head : w8) (dq : DFrac) (d count : Nat) (reason : KeyFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (positive : 0 < count) (fit : 2 * count ≤ wire.length) (failure : KeyRejected wire reason)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c))
        (Val #(structFieldRef StateUpdate go!"StateDiff" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ StateUpdateFieldFailureOwned err (v.pos' + W64 (1 + payload.length))
         (.diff stateDiffZero (.key reason)) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = wire ∧
         StateUpdateFieldRejected d .diff ((head :: payload) ++ wire)
           (1 + payload.length) (1 + payload.length) 1 (.diff stateDiffZero (.key reason))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_diff_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStateDiffPointer_first_key_rejected validator c _ previous v xs payload wire head dq d count reason
    enc major positive fit failure hd depth hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Htemporary, Hc, Hs, Herr, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %previous %old.diff [Hfield Hprevious]
  · iframe Hfield Hprevious
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Htemporary Herr]
  · unfold StateUpdateFieldFailureOwned
    isplitl [Htemporary]
    · iexact Htemporary
    · iexact Herr
  ipureintro
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2.1, .diff Hnext.2.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readStateDiffPointer_first_key_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_diff_first_key_rejected
