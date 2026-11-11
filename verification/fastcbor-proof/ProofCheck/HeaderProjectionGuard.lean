import ProofCheck.HeaderProjectionOwnership

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

def headerProjectionGuardCondition (kind : HeaderProjectionKind) : HeaderProjectionLayout kind → Prop :=
  match kind with
  | .hash => fun h => h.Hash' = null
  | .root => fun h => h.GlobalStateRoot' = null
  | .count => fun h => h.TransactionCount' = W64 0
  | .timestamp => fun h => h.Timestamp' = null
  | .hashAndRoot => fun h => h.Hash' = null ∧ h.GlobalStateRoot' = null

/-- The guards omit the embedded empty skeleton, which has only one value. -/
theorem headerProjectionLayout_zero_iff (kind : HeaderProjectionKind) (h : HeaderProjectionLayout kind) :
    h = headerProjectionLayoutZero kind ↔ headerProjectionGuardCondition kind h := by
  cases kind with
  | hash =>
    cases h with
    | mk skeleton first =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      change headerHashProjection.mk _ first = headerHashProjection.mk _ (null) ↔ first = null
      simp
  | root =>
    cases h with
    | mk skeleton first =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      change headerGlobalStateRootProjection.mk _ first = headerGlobalStateRootProjection.mk _ (null) ↔ first = null
      simp
  | count =>
    cases h with
    | mk skeleton first =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      change headerTransactionCountProjection.mk _ first = headerTransactionCountProjection.mk _ (W64 0) ↔ first = W64 0
      simp
  | timestamp =>
    cases h with
    | mk skeleton first =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      change headerTimestampProjection.mk _ first = headerTimestampProjection.mk _ (null) ↔ first = null
      simp
  | hashAndRoot =>
    cases h with
    | mk skeleton first second =>
      have hs := discardedHeaderSkeleton_eq_zero skeleton
      subst skeleton
      change headerHashAndStateRootProjection.mk _ first second = headerHashAndStateRootProjection.mk _ null null ↔ first = null ∧ second = null
      simp

def headerProjectionZeroName : HeaderProjectionKind → GoString
  | .hash => cborZeroHeaderHashProjection
  | .root => cborZeroHeaderGlobalStateRootProjection
  | .count => cborZeroHeaderTransactionCountProjection
  | .timestamp => cborZeroHeaderTimestampProjection
  | .hashAndRoot => cborZeroHeaderHashAndStateRootProjection

def headerProjectionClearName : HeaderProjectionKind → GoString
  | .hash => cborClearHeaderHashProjection
  | .root => cborClearHeaderGlobalStateRootProjection
  | .count => cborClearHeaderTransactionCountProjection
  | .timestamp => cborClearHeaderTimestampProjection
  | .hashAndRoot => cborClearHeaderHashAndStateRootProjection

/-- The actual generated guards inspect the selected fields and preserve all
    destination cells. No input or child-pointee ownership is consumed. -/
theorem wp_zeroHeaderProjection (kind : HeaderProjectionKind) (dst : Loc) (h : HeaderProjectionLayout kind) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       HeaderProjectionCells kind dst h }}
      (App (Val (@! (headerProjectionZeroName kind))) (Val #dst))
    {{ (result : Bool), RET #result; HeaderProjectionCells kind dst h ∗
       ⌜result = true ↔ headerProjectionGuardCondition kind h⌝ }} := by
  cases kind with
  | hash =>
    simp only [headerProjectionZeroName, HeaderProjectionCells, headerProjectionGuardCondition]
    wp_start as Hdst
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp
  | root =>
    simp only [headerProjectionZeroName, HeaderProjectionCells, headerProjectionGuardCondition]
    wp_start as Hdst
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp
  | count =>
    simp only [headerProjectionZeroName, HeaderProjectionCells, headerProjectionGuardCondition]
    wp_start as Hdst
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp
  | timestamp =>
    simp only [headerProjectionZeroName, HeaderProjectionCells, headerProjectionGuardCondition]
    wp_start as Hdst
    wp_auto
    iapply HΦ
    iframe Hdst
    ipureintro
    simp
  | hashAndRoot =>
    simp only [headerProjectionZeroName, HeaderProjectionCells, headerProjectionGuardCondition]
    wp_start as Hdst
    wp_auto
    by_cases hh : h.Hash' = null
    · simp only [hh, _root_.decide_true]
      wp_auto
      iapply HΦ
      iframe Hdst
      ipureintro
      simp [hh]
    · simp only [hh, _root_.decide_false]
      wp_auto
      iapply HΦ
      iframe Hdst
      ipureintro
      simp [hh]

/-- The actual clear functions overwrite the entire concrete struct, including
    its empty skeleton. This proves reset, not physical GC reclamation. -/
theorem wp_clearHeaderProjection (kind : HeaderProjectionKind) (dst : Loc) (h : HeaderProjectionLayout kind) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       HeaderProjectionCells kind dst h }}
      (App (Val (@! (headerProjectionClearName kind))) (Val #dst))
    {{ RET #(); HeaderProjectionCells kind dst (headerProjectionLayoutZero kind) }} := by
  cases kind <;> simp only [headerProjectionClearName, HeaderProjectionCells, headerProjectionLayoutZero]
  all_goals
    wp_start as Hdst
    wp_auto
    wp_end

/-- Caller-visible guard contract, preserving ownership even when a nonzero
    projection is rejected. Allocated zero children remain nonzero destinations. -/
theorem wp_zeroHeaderProjection_owned (kind : HeaderProjectionKind) (dst : Loc) (value : HeaderProjectionValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       HeaderProjectionOwned kind dst value }}
      (App (Val (@! (headerProjectionZeroName kind))) (Val #dst))
    {{ (result : Bool), RET #result; HeaderProjectionOwned kind dst value ∗ ⌜result = true ↔ value = headerProjectionZero⌝ }} := by
  wp_start_folded as Howned
  isimp only [HeaderProjectionOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hzero := headerProjectionChildren_zero_iff kind h value $$ Hchildren
  wp_apply wp_zeroHeaderProjection kind dst h $$ [Hdst] as %result ⟨Hdst, %Hguard⟩
  · iframe Hdst
  iapply HΦ
  isplitl [Hdst Hchildren]
  · unfold HeaderProjectionOwned
    iexists h
    iframe Hdst Hchildren
  ipureintro
  exact Hguard.trans ((headerProjectionLayout_zero_iff kind h).symm.trans Hzero)

/-- Cleanup discards any owned partial children and publishes an owned zero
    projection under the affine GC model. -/
theorem wp_clearHeaderProjection_owned (kind : HeaderProjectionKind) (dst : Loc) (value : HeaderProjectionValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       HeaderProjectionOwned kind dst value }}
      (App (Val (@! (headerProjectionClearName kind))) (Val #dst))
    {{ RET #(); HeaderProjectionOwned kind dst headerProjectionZero }} := by
  wp_start_folded as Howned
  isimp only [HeaderProjectionOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  wp_apply wp_clearHeaderProjection kind dst h $$ [Hdst] with Hdst
  · iframe Hdst
  iapply HΦ
  unfold HeaderProjectionOwned
  iexists headerProjectionLayoutZero kind
  iframe Hdst
  iapply headerProjectionChildren_zero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionLayout_zero_iff
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroHeaderProjection
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeaderProjection
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_zeroHeaderProjection_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearHeaderProjection_owned
