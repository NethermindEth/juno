import ProofCheck.StateUpdateErrors

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

/-- Null/undefined clears the StateDiff pointer and preserves all three scalar
    felt fields; the old pointed-to record may be dropped under the GC model. -/
theorem wp_stateUpdate_diff_nullish (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs rest : List w8) (head : w8) (dq : DFrac) (d : Nat) (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #(structFieldRef StateUpdate go!"StateDiff" dst)))
    {{ RET #interface.nil; StateUpdateOwned dst {old with diff := none} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧
         sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest ∧
         StateUpdateFieldValid d .diff old [head] rest {old with diff := none}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_diff_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStateDiffPointer_nullish c _ previous v xs rest head dq hn hp hbytes
    $$ [Hfield Hc Hs] with ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %null %none [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .diff (.nullish rest hn)⟩

/-- Every empty-map encoding publishes a fresh nonnil zero StateDiff, preserving
    the sibling felts, with exact suffix and restored enclosing depth. -/
theorem wp_stateUpdate_diff_empty (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (dq : DFrac) (d : Nat)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #(structFieldRef StateUpdate go!"StateDiff" dst)))
    {{ RET #interface.nil; StateUpdateOwned dst {old with diff := some stateDiffZero} ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest ∧
         StateUpdateFieldValid d .diff old (head :: payload) rest {old with diff := some stateDiffZero}⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_diff_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStateDiffPointer_empty c _ previous v xs payload rest head dq d enc major hd depth hp hbytes
    $$ [Hfield Hc Hs] as %out ⟨Hfield, Hnew, Hc, Hs, %Hnext⟩
  · iframe Hfield Hc Hs
  ihave Howned := Hclose $$ %out %(some stateDiffZero) [Hfield Hnew]
  · iframe Hfield Hnew
  iapply HΦ
  iframe Howned Hc Hs
  ipureintro
  have hn : ((head :: payload) ++ rest).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have eqHead := Option.some.inj eq
    subst head
    contradiction
  have hu : ((head :: payload) ++ rest).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons]
    intro eq
    have eqHead := Option.some.inj eq
    subst head
    contradiction
  have record : StateDiffRecordValid d stateDiffZero (head :: payload) rest stateDiffZero := by
    simpa only [List.append_nil] using
      (StateDiffRecordValid.map (count := 0) enc major (by decide) depth
        (StateDiffPairsValid.nil (d := d + 1) [] stateDiffZero rest))
  exact ⟨Hnext.1, Hnext.2.1, Hnext.2.2, .diff (.record hn hu record)⟩

/-- Opening errors preserve the complete old update, including its old StateDiff.
    The failed fresh zero record stays private and owned in the field diagnostic. -/
theorem wp_stateUpdate_diff_open_rejected (c dst : Loc) (old : StateUpdateValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (reason : FeltFailureReason)
    (failure : StructOpenFailure d wire reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadStateDiffPointer)) (Val #c)) (Val #(structFieldRef StateUpdate go!"StateDiff" dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       StateUpdateOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       StateUpdateFieldFailureOwned err v.pos' (.diff stateDiffZero (.opening reason)) ∗
       ⌜StateUpdateFieldRejected d .diff wire 0 0 0 (.diff stateDiffZero (.opening reason))⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  icases stateUpdate_diff_acc dst old $$ Howned with ⟨%previous, Hfield, Hprevious, Hclose⟩
  wp_apply wp_readStateDiffPointer_open_rejected_owned c _ previous v xs wire dq d reason failure hd hp hbytes
    $$ [Hfield Hc Hs] as %err ⟨Hfield, Htemporary, Hc, Hs, Herr⟩
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
  have excluded := structOpenFailure_not_nullish d wire reason failure
  exact .diff (.record excluded.1 excluded.2 (.opening failure))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_diff_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_diff_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_stateUpdate_diff_open_rejected
