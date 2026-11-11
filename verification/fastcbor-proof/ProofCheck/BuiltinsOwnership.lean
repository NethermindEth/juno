import ProofCheck.BuiltinsValues
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

def builtinsConcrete (value : BuiltinsValue) : BuiltinInstanceCounter := ⟨value.pedersen, value.rangeCheck, value.bitwise, value.output, value.ecsda, value.ecOp, value.keccak, value.poseidon, value.segmentArena, value.addMod, value.mulMod, value.rangeCheck96⟩
def BuiltinsOwned (dst : Loc) (value : BuiltinsValue) : IProp GF := iprop(dst ↦ builtinsConcrete value)

/-- Each scalar call borrows its actual field and retains every other counter. -/
theorem builtins_field_acc (dst : Loc) (old : BuiltinsValue) (field : BuiltinsField) :
    BuiltinsOwned dst old ⊢ (iprop(
      structFieldRef BuiltinInstanceCounter (builtinsFieldName field) dst ↦ builtinsGet field old ∗
      (∀ value : w64, structFieldRef BuiltinInstanceCounter (builtinsFieldName field) dst ↦ value -∗
        BuiltinsOwned dst (builtinsSet field old value))) : IProp GF) := by
  unfold BuiltinsOwned
  iintro Hh
  icases typedPointsto_not_null_dup dst (builtinsConcrete old) (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst (builtinsConcrete old) (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete] at Hh
  icases Hh with ⟨Hpedersen, HrangeCheck, Hbitwise, Houtput, Hecsda, HecOp, Hkeccak, Hposeidon, HsegmentArena, HaddMod, HmulMod, HrangeCheck96, -⟩
  cases field <;> isimp only [builtinsFieldName, builtinsGet, builtinsSet]
  · iframe Hpedersen
    iintro %value Hpedersen
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HrangeCheck
    iintro %value HrangeCheck
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe Hbitwise
    iintro %value Hbitwise
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe Houtput
    iintro %value Houtput
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe Hecsda
    iintro %value Hecsda
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HecOp
    iintro %value HecOp
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe Hkeccak
    iintro %value Hkeccak
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe Hposeidon
    iintro %value Hposeidon
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HsegmentArena
    iintro %value HsegmentArena
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HaddMod
    iintro %value HaddMod
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HmulMod
    iintro %value HmulMod
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96
  · iframe HrangeCheck96
    iintro %value HrangeCheck96
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef, builtinsConcrete]
    iframe Hpedersen HrangeCheck Hbitwise Houtput Hecsda HecOp Hkeccak Hposeidon HsegmentArena HaddMod HmulMod HrangeCheck96

theorem builtinsOwned_zero (dst : Loc) :
    dst ↦ zero_val BuiltinInstanceCounter ⊢ (BuiltinsOwned dst builtinsZero : IProp GF) := by
  unfold BuiltinsOwned
  have hzero : builtinsConcrete builtinsZero = zero_val BuiltinInstanceCounter := rfl
  rw [hzero]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.builtins_field_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.builtinsOwned_zero
