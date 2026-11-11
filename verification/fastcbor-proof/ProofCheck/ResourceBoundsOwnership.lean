import ProofCheck.ResourceBoundsValues
import ProofCheck.InitializationErrors

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

def ResourceBoundsOwned (dst : Loc) (value : ResourceBoundsValue) : IProp GF := iprop(
  ∃ ptr : Loc, dst ↦ ResourceBounds.mk value.amount ptr ∗ FeltResult ptr value.price)

/-- Borrow only the amount cell, retaining ownership of the nullable price. -/
theorem resourceBounds_amount_acc (dst : Loc) (old : ResourceBoundsValue) :
    ResourceBoundsOwned dst old ⊢ (iprop(
      structFieldRef ResourceBounds go!"MaxAmount" dst ↦ old.amount ∗
      (∀ value : w64, structFieldRef ResourceBounds go!"MaxAmount" dst ↦ value -∗
        ResourceBoundsOwned dst {old with amount := value})) : IProp GF) := by
  unfold ResourceBoundsOwned
  iintro ⟨%ptr, Hh, Hprice⟩
  icases typedPointsto_not_null_dup dst (ResourceBounds.mk old.amount ptr) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (ResourceBounds.mk old.amount ptr) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hamount, Hptr, -⟩
  iframe Hamount
  iintro %value Hamount
  iexists ptr
  iframe Hprice
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hamount Hptr

/-- Borrow the price pointer and child separately, so publication can replace
    the child while a rejected call reconstructs the exact previous record. -/
theorem resourceBounds_price_acc (dst : Loc) (old : ResourceBoundsValue) :
    ResourceBoundsOwned dst old ⊢ (iprop(∃ ptr : Loc,
      structFieldRef ResourceBounds go!"MaxPricePerUnit" dst ↦ ptr ∗ FeltResult ptr old.price ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef ResourceBounds go!"MaxPricePerUnit" dst ↦ out ∗ FeltResult out value -∗
        ResourceBoundsOwned dst {old with price := value})) : IProp GF) := by
  unfold ResourceBoundsOwned
  iintro ⟨%ptr, Hh, Hprice⟩
  icases typedPointsto_not_null_dup dst (ResourceBounds.mk old.amount ptr) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (ResourceBounds.mk old.amount ptr) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hamount, Hptr, -⟩
  iexists ptr
  iframe Hptr Hprice
  iintro %out %value ⟨Hptr, Hprice⟩
  iexists out
  iframe Hprice
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hamount Hptr

theorem resourceBoundsOwned_zero (dst : Loc) :
    dst ↦ zero_val ResourceBounds ⊢ (ResourceBoundsOwned dst resourceBoundsZero : IProp GF) := by
  iintro Hdst
  unfold ResourceBoundsOwned
  iexists null
  have hz : zero_val ResourceBounds = ResourceBounds.mk (W64 0) null := rfl
  isimp only [hz] at Hdst
  isimp only [resourceBoundsZero, FeltResult]
  iframe Hdst

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBounds_amount_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBounds_price_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceBoundsOwned_zero
