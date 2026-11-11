import ProofCheck.DeprecatedEntryPointValues
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

def DeprecatedEntryPointOwned (dst : Loc) (value : DeprecatedEntryPointValue) : IProp GF := iprop(
  ∃ selector offset : Loc, dst ↦ DeprecatedEntryPoint.mk selector offset ∗
    FeltResult selector value.selector ∗ FeltResult offset value.offset)

/-- Borrow the selector pointer and its child, retaining the other field. -/
theorem deprecatedEntryPoint_selector_acc (dst : Loc) (old : DeprecatedEntryPointValue) :
    DeprecatedEntryPointOwned dst old ⊢ (iprop(∃ ptr : Loc,
      structFieldRef DeprecatedEntryPoint go!"Selector" dst ↦ ptr ∗ FeltResult ptr old.selector ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef DeprecatedEntryPoint go!"Selector" dst ↦ out ∗ FeltResult out value -∗
        DeprecatedEntryPointOwned dst {old with selector := value})) : IProp GF) := by
  unfold DeprecatedEntryPointOwned
  iintro ⟨%selector, %offset, Hh, Hselector, Hoffset⟩
  icases typedPointsto_not_null_dup dst (DeprecatedEntryPoint.mk selector offset) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedEntryPoint.mk selector offset) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HselectorPtr, HoffsetPtr, -⟩
  iexists selector
  iframe HselectorPtr Hselector
  iintro %out %value ⟨HselectorPtr, Hselector⟩
  iexists out, offset
  iframe Hselector Hoffset
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HselectorPtr HoffsetPtr

/-- Borrow the offset pointer and its child, retaining the other field. -/
theorem deprecatedEntryPoint_offset_acc (dst : Loc) (old : DeprecatedEntryPointValue) :
    DeprecatedEntryPointOwned dst old ⊢ (iprop(∃ ptr : Loc,
      structFieldRef DeprecatedEntryPoint go!"Offset" dst ↦ ptr ∗ FeltResult ptr old.offset ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)),
        structFieldRef DeprecatedEntryPoint go!"Offset" dst ↦ out ∗ FeltResult out value -∗
        DeprecatedEntryPointOwned dst {old with offset := value})) : IProp GF) := by
  unfold DeprecatedEntryPointOwned
  iintro ⟨%selector, %offset, Hh, Hselector, Hoffset⟩
  icases typedPointsto_not_null_dup dst (DeprecatedEntryPoint.mk selector offset) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (DeprecatedEntryPoint.mk selector offset) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨HselectorPtr, HoffsetPtr, -⟩
  iexists offset
  iframe HoffsetPtr Hoffset
  iintro %out %value ⟨HoffsetPtr, Hoffset⟩
  iexists selector, out
  iframe Hselector Hoffset
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe HselectorPtr HoffsetPtr

theorem deprecatedEntryPointOwned_zero (dst : Loc) :
    dst ↦ zero_val DeprecatedEntryPoint ⊢ (DeprecatedEntryPointOwned dst deprecatedEntryPointZero : IProp GF) := by
  iintro Hdst
  unfold DeprecatedEntryPointOwned
  iexists null, null
  have hz : zero_val DeprecatedEntryPoint = DeprecatedEntryPoint.mk null null := rfl
  isimp only [hz] at Hdst
  isimp only [deprecatedEntryPointZero, FeltResult]
  iframe Hdst
  isplit <;> ipureintro <;> trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPoint_selector_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPoint_offset_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.deprecatedEntryPointOwned_zero
