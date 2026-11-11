import ProofCheck.StateUpdateOwnership
import ProofCheck.HeaderOwnedZero

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Nil pointer ownership is exact, even when a nonnil child has zero fields. -/
theorem stateDiffPointerOwned_null_iff (ptr : Loc) (value : Option StateDiffValue) :
    StateDiffPointerOwned ptr value ⊢ (⌜ptr = null ↔ value = none⌝ : IProp GF) := by
  cases value with
  | none => isimp only [StateDiffPointerOwned]; iintro %h; ipureintro; simp [h]
  | some value =>
    isimp only [StateDiffPointerOwned, StateDiffOwned]
    iintro ⟨%storage, %nonces, %deployed, %declaredV0, %declaredV1, %replaced, %migrated, Hdst, _⟩
    ihave %hn := typedPointsto_not_null ptr
      (StateDiff.mk storage nonces deployed declaredV0 declaredV1 replaced migrated) (DFrac.own 1) $$ Hdst
    ipureintro; simp [hn]

theorem stateUpdateValue_zero_iff (value : StateUpdateValue) :
    value = stateUpdateZero ↔ value.blockHash = none ∧ value.newRoot = none ∧
      value.oldRoot = none ∧ value.diff = none := by
  cases value
  simp [stateUpdateZero]

/-- All four actual pointer fields participate in the zero-destination guard. -/
theorem wp_zeroStateUpdate (dst : Loc) (h : StateUpdate) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗ dst ↦ h }}
      (App (Val (@! cborZeroStateUpdate)) (Val #dst))
    {{ (result : Bool), RET #result; dst ↦ h ∗ ⌜result = true ↔ h = zero_val StateUpdate⌝ }} := by
  wp_start as Hdst
  wp_auto
  by_cases h0 : h.BlockHash' ≠ null
  · have hn : h ≠ zero_val StateUpdate := by
      intro heq
      subst h
      exact h0 rfl
    simp only [h0, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h0' : h.BlockHash' = null := Classical.not_not.mp h0
  simp only [h0', _root_.decide_true]
  wp_auto
  by_cases h1 : h.NewRoot' ≠ null
  · have hn : h ≠ zero_val StateUpdate := by
      intro heq
      subst h
      exact h1 rfl
    simp only [h1, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h1' : h.NewRoot' = null := Classical.not_not.mp h1
  simp only [h1', _root_.decide_true]
  wp_auto
  by_cases h2 : h.OldRoot' ≠ null
  · have hn : h ≠ zero_val StateUpdate := by
      intro heq
      subst h
      exact h2 rfl
    simp only [h2, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro; simp [hn]
  have h2' : h.OldRoot' = null := Classical.not_not.mp h2
  simp only [h2', _root_.decide_true]
  wp_auto
  iapply HΦ
  iframe Hdst
  ipureintro
  have hz : zero_val StateUpdate = StateUpdate.mk null null null null := rfl
  rw [hz]
  cases h
  simp_all

theorem wp_zeroStateUpdate_owned (dst : Loc) (value : StateUpdateValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst value }}
      (App (Val (@! cborZeroStateUpdate)) (Val #dst))
    {{ (result : Bool), RET #result; StateUpdateOwned dst value ∗ ⌜result = true ↔ value = stateUpdateZero⌝ }} := by
  wp_start_folded as Howned
  isimp only [StateUpdateOwned] at Howned
  icases Howned with ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, HblockHash, HnewRoot, HoldRoot, Hdiff⟩
  ihave %HblockHashNil := feltResult_null_iff blockHash value.blockHash $$ HblockHash
  ihave %HnewRootNil := feltResult_null_iff newRoot value.newRoot $$ HnewRoot
  ihave %HoldRootNil := feltResult_null_iff oldRoot value.oldRoot $$ HoldRoot
  ihave %HdiffNil := stateDiffPointerOwned_null_iff diff value.diff $$ Hdiff
  wp_apply wp_zeroStateUpdate dst (StateUpdate.mk blockHash newRoot oldRoot diff) $$ [Hh] as %result ⟨Hh, %Hresult⟩
  · iframe Hh
  iapply HΦ
  isplitl [Hh HblockHash HnewRoot HoldRoot Hdiff]
  · unfold StateUpdateOwned
    iexists blockHash, newRoot, oldRoot, diff
    iframe
  · ipureintro
    rw [Hresult, stateUpdateValue_zero_iff]
    have hz : zero_val StateUpdate = StateUpdate.mk null null null null := rfl
    rw [hz, StateUpdate.mk.injEq]
    simp only [HblockHashNil, HnewRootNil, HoldRootNil, HdiffNil]

/-- Whole-record cleanup clears any partially decoded state update; former
    children become unreachable under the garbage-collected ownership model. -/
theorem wp_clearStateUpdate_owned (dst : Loc) (value : StateUpdateValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ∗
       StateUpdateOwned dst value }}
      (App (Val (@! cborClearStateUpdate)) (Val #dst))
    {{ RET #(); StateUpdateOwned dst stateUpdateZero }} := by
  wp_start as Howned
  isimp only [StateUpdateOwned] at Howned
  icases Howned with ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, _⟩
  wp_auto
  iapply HΦ
  iapply stateUpdateOwned_zero $$ Hh

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffPointerOwned_null_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateValue_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroStateUpdate
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroStateUpdate_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearStateUpdate_owned
