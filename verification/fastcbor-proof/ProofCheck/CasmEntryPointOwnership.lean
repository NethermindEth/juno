import ProofCheck.CasmEntryPointValues
import ProofCheck.StringsComplete
import ProofCheck.ScalarComplete
import ProofCheck.FeltValid
import ProofCheck.FeltFailureSound

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

def CasmEntryPointOwned (dst : Loc) (value : CasmEntryPointValue) : IProp GF := iprop(
  ∃ (builtins : GoSlice) (selector : Loc),
    dst ↦ CasmEntryPoint.mk value.offset builtins selector ∗
    StringsResult builtins value.builtins ∗ FeltResult selector value.selector)

/-- Borrow the offset field while retaining all unrelated record ownership. -/
theorem casmEntryPoint_offset_acc (dst : Loc) (old : CasmEntryPointValue) :
    CasmEntryPointOwned dst old ⊢ (iprop(
      structFieldRef CasmEntryPoint go!"Offset" dst ↦ old.offset ∗ 
      (∀ value : w64, structFieldRef CasmEntryPoint go!"Offset" dst ↦ value -∗
        CasmEntryPointOwned dst {old with offset := value})) : IProp GF) := by
  unfold CasmEntryPointOwned
  iintro ⟨%builtins, %selector, Hh, Hbuiltins, Hselector⟩
  icases typedPointsto_not_null_dup dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hoffset, Hslice, Hptr, -⟩
  iframe Hoffset
  iintro %value Hoffset
  iexists builtins, selector
  iframe Hbuiltins Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hoffset Hslice Hptr

/-- Borrow the builtins field while retaining all unrelated record ownership. -/
theorem casmEntryPoint_builtins_acc (dst : Loc) (old : CasmEntryPointValue) :
    CasmEntryPointOwned dst old ⊢ (iprop(∃ builtins : GoSlice, 
      structFieldRef CasmEntryPoint go!"Builtins" dst ↦ builtins ∗ StringsResult builtins old.builtins ∗
      (∀ (out : GoSlice) (value : Option (List GoString)), structFieldRef CasmEntryPoint go!"Builtins" dst ↦ out ∗ StringsResult out value -∗
        CasmEntryPointOwned dst {old with builtins := value})) : IProp GF) := by
  unfold CasmEntryPointOwned
  iintro ⟨%builtins, %selector, Hh, Hbuiltins, Hselector⟩
  icases typedPointsto_not_null_dup dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hoffset, Hslice, Hptr, -⟩
  iexists builtins
  iframe Hslice Hbuiltins
  iintro %out %value ⟨Hslice, Hbuiltins⟩
  iexists out, selector
  iframe Hbuiltins Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hoffset Hslice Hptr

/-- Borrow the selector field while retaining all unrelated record ownership. -/
theorem casmEntryPoint_selector_acc (dst : Loc) (old : CasmEntryPointValue) :
    CasmEntryPointOwned dst old ⊢ (iprop(∃ selector : Loc, 
      structFieldRef CasmEntryPoint go!"Selector" dst ↦ selector ∗ FeltResult selector old.selector ∗
      (∀ (out : Loc) (value : Option (GoArray w64 4)), structFieldRef CasmEntryPoint go!"Selector" dst ↦ out ∗ FeltResult out value -∗
        CasmEntryPointOwned dst {old with selector := value})) : IProp GF) := by
  unfold CasmEntryPointOwned
  iintro ⟨%builtins, %selector, Hh, Hbuiltins, Hselector⟩
  icases typedPointsto_not_null_dup dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (CasmEntryPoint.mk old.offset builtins selector) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hoffset, Hslice, Hptr, -⟩
  iexists selector
  iframe Hptr Hselector
  iintro %out %value ⟨Hptr, Hselector⟩
  iexists builtins, out
  iframe Hbuiltins Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hoffset Hslice Hptr

theorem casmEntryPointOwned_zero (dst : Loc) :
    dst ↦ zero_val CasmEntryPoint ⊢ (CasmEntryPointOwned dst casmEntryPointZero : IProp GF) := by
  iintro Hdst
  unfold CasmEntryPointOwned
  iexists slice.nil, null
  have hz : zero_val CasmEntryPoint = CasmEntryPoint.mk (W64 0) slice.nil null := rfl
  isimp only [hz] at Hdst
  isimp only [casmEntryPointZero, StringsResult, FeltResult]
  iframe Hdst
  ipureintro; trivial

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPoint_offset_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPoint_builtins_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPoint_selector_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointOwned_zero
