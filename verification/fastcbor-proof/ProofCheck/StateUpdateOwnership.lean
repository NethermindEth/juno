import ProofCheck.StateUpdateValues
import ProofCheck.StateDiffPointerBoundary
import ProofCheck.FeltValid

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

def StateUpdateOwned (dst : Loc) (value : StateUpdateValue) : IProp GF := iprop(
  ∃ (blockHash : Loc) (newRoot : Loc) (oldRoot : Loc) (diff : Loc),
    dst ↦ StateUpdate.mk blockHash newRoot oldRoot diff ∗
    FeltResult blockHash value.blockHash ∗
    FeltResult newRoot value.newRoot ∗
    FeltResult oldRoot value.oldRoot ∗
    StateDiffPointerOwned diff value.diff)

/-- Borrow BlockHash and its owned children, retaining every other field. -/
theorem stateUpdate_blockHash_acc (dst : Loc) (old : StateUpdateValue) :
    StateUpdateOwned dst old ⊢ (iprop(∃ blockHash : Loc,
      structFieldRef StateUpdate go!"BlockHash" dst ↦ blockHash ∗ FeltResult blockHash old.blockHash ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef StateUpdate go!"BlockHash" dst ↦ out ∗ FeltResult out value -∗
        StateUpdateOwned dst {old with blockHash := value})) : IProp GF) := by
  unfold StateUpdateOwned
  iintro ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, HblockHash, HnewRoot, HoldRoot, Hdiff⟩
  icases typedPointsto_not_null_dup dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HblockHashCell, HnewRootCell, HoldRootCell, HdiffCell, -⟩
  iexists blockHash
  iframe HblockHashCell HblockHash
  iintro %out %value ⟨HblockHashCell, HblockHash⟩
  iexists out, newRoot, oldRoot, diff
  iframe HblockHash HnewRoot HoldRoot Hdiff
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HblockHashCell HnewRootCell HoldRootCell HdiffCell

/-- Borrow NewRoot and its owned children, retaining every other field. -/
theorem stateUpdate_newRoot_acc (dst : Loc) (old : StateUpdateValue) :
    StateUpdateOwned dst old ⊢ (iprop(∃ newRoot : Loc,
      structFieldRef StateUpdate go!"NewRoot" dst ↦ newRoot ∗ FeltResult newRoot old.newRoot ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef StateUpdate go!"NewRoot" dst ↦ out ∗ FeltResult out value -∗
        StateUpdateOwned dst {old with newRoot := value})) : IProp GF) := by
  unfold StateUpdateOwned
  iintro ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, HblockHash, HnewRoot, HoldRoot, Hdiff⟩
  icases typedPointsto_not_null_dup dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HblockHashCell, HnewRootCell, HoldRootCell, HdiffCell, -⟩
  iexists newRoot
  iframe HnewRootCell HnewRoot
  iintro %out %value ⟨HnewRootCell, HnewRoot⟩
  iexists blockHash, out, oldRoot, diff
  iframe HblockHash HnewRoot HoldRoot Hdiff
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HblockHashCell HnewRootCell HoldRootCell HdiffCell

/-- Borrow OldRoot and its owned children, retaining every other field. -/
theorem stateUpdate_oldRoot_acc (dst : Loc) (old : StateUpdateValue) :
    StateUpdateOwned dst old ⊢ (iprop(∃ oldRoot : Loc,
      structFieldRef StateUpdate go!"OldRoot" dst ↦ oldRoot ∗ FeltResult oldRoot old.oldRoot ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef StateUpdate go!"OldRoot" dst ↦ out ∗ FeltResult out value -∗
        StateUpdateOwned dst {old with oldRoot := value})) : IProp GF) := by
  unfold StateUpdateOwned
  iintro ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, HblockHash, HnewRoot, HoldRoot, Hdiff⟩
  icases typedPointsto_not_null_dup dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HblockHashCell, HnewRootCell, HoldRootCell, HdiffCell, -⟩
  iexists oldRoot
  iframe HoldRootCell HoldRoot
  iintro %out %value ⟨HoldRootCell, HoldRoot⟩
  iexists blockHash, newRoot, out, diff
  iframe HblockHash HnewRoot HoldRoot Hdiff
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HblockHashCell HnewRootCell HoldRootCell HdiffCell

/-- Borrow StateDiff and its owned children, retaining every other field. -/
theorem stateUpdate_diff_acc (dst : Loc) (old : StateUpdateValue) :
    StateUpdateOwned dst old ⊢ (iprop(∃ diff : Loc,
      structFieldRef StateUpdate go!"StateDiff" dst ↦ diff ∗ StateDiffPointerOwned diff old.diff ∗
      (∀ (out : Loc) (value : Option StateDiffValue),
        structFieldRef StateUpdate go!"StateDiff" dst ↦ out ∗ StateDiffPointerOwned out value -∗
        StateUpdateOwned dst {old with diff := value})) : IProp GF) := by
  unfold StateUpdateOwned
  iintro ⟨%blockHash, %newRoot, %oldRoot, %diff, Hh, HblockHash, HnewRoot, HoldRoot, Hdiff⟩
  icases typedPointsto_not_null_dup dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (StateUpdate.mk blockHash newRoot oldRoot diff) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HblockHashCell, HnewRootCell, HoldRootCell, HdiffCell, -⟩
  iexists diff
  iframe HdiffCell Hdiff
  iintro %out %value ⟨HdiffCell, Hdiff⟩
  iexists blockHash, newRoot, oldRoot, out
  iframe HblockHash HnewRoot HoldRoot Hdiff
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HblockHashCell HnewRootCell HoldRootCell HdiffCell

theorem stateUpdateOwned_zero (dst : Loc) :
    dst ↦ zero_val StateUpdate ⊢ (StateUpdateOwned dst stateUpdateZero : IProp GF) := by
  iintro Hdst
  unfold StateUpdateOwned
  iexists null, null, null, null
  have hz : zero_val StateUpdate = StateUpdate.mk null null null null := rfl
  isimp only [hz] at Hdst
  isimp only [stateUpdateZero, FeltResult, StateDiffPointerOwned]
  iframe Hdst
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdate_blockHash_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdate_newRoot_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdate_oldRoot_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdate_diff_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.stateUpdateOwned_zero
