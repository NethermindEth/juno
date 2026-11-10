import ProofCheck.InitializationState

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Repeated package initialization uses the actual package-state check and
    retrieves the recorded resources without executing the initializer body.
    This accepts any body; the proof relies on established state, not its assumed
    correctness or a model that replaces its instructions. -/
theorem wp_package_init_already (pkg : GoString) (f x : Binder) (body : Expr) (get_is_pkg_init : GoString → IProp GF) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg }}
      (App (Val (package.init pkg)) (Val (RecV f x body)))
    {{ RET #(); ownInitializing get_is_pkg_init ∗ get_is_pkg_init pkg }} := by
  rw [package.init_unseal]
  wp_start as ⟨Hown, #Hready⟩
  isimp only [PackageInitialized] at Hready
  icases Hready with ⟨%σ0, #Hlb, %Hknown⟩
  isimp only [ownInitializing_unseal, ownInitializingDef] at Hown
  icases Hown with ⟨%σ, Hg, #Hinit⟩
  ihave %Hle := ownGoStateLb_le σ0 σ $$ Hlb Hg
  have Hlookup := Hle pkg Hknown
  wp_apply_core wp_PackageInitCheck pkg σ $$ Hg
  iintro Hg
  simp only [Hlookup, Option.getD_some]
  wp_auto
  ihave Hx := BigSepM.bigSepM_lookup (Φ := fun k (b : Bool) =>
    if b then get_is_pkg_init k else iprop(True)) Hlookup $$ Hinit
  iapply HΦ
  isplitl [Hg]
  · isimp only [ownInitializing_unseal, ownInitializingDef]
    iexists σ
    iframe Hg Hinit
  · isimp only [ite_true] at Hx
    iexact Hx

/-- The runtime has already initialized internal/cpu. Its package-state entry
    ensures this call preserves its feature flags instead of allocating/resetting
    them. Hardware probing and the runtime's creation of this initial state are
    explicitly outside the decoder proof. -/
theorem wp_cpu_initialize_ready (g : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.internal.cpu g) :
    {{ ownInitializing g ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cpu.initialize') (Val #()))
    {{ RET #(); ownInitializing g ∗ isPkgInit (PROP := IProp GF) pkg_id.internal.cpu }} := by
  wp_start as ⟨Hown, #Hready⟩
  wp_apply wp_package_init_already _ _ _ _ g $$ [Hown] as ⟨Hown, Hcpu⟩
  · iframe Hown Hready
  isimp only [Hinit.1] at Hcpu
  iapply HΦ
  iframe Hown Hcpu

/-- Both packages' generated representations identify the same Go struct fields.
    The runtime resource describes the actual CPU globals read by math/big. -/
theorem cpu_feature_fields_match :
    (globalAddr internal.cpu.X86).[math.big.«struct#08e4e4b9», go!"HasADX"] =
      (globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», go!"HasADX"] ∧
    (globalAddr internal.cpu.X86).[math.big.«struct#08e4e4b9», go!"HasBMI2"] =
      (globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», go!"HasBMI2"] := by
  have Htype : math.big.«struct#08e4e4b9».underlying = internal.cpu.«struct#08e4e4b9».underlying := by
    unfold math.big.«struct#08e4e4b9».underlying internal.cpu.«struct#08e4e4b9».underlying
      math.big.«struct#08e4e4b9».fields internal.cpu.«struct#08e4e4b9».fields
    rfl
  have hfield (name : GoString) :
      (globalAddr internal.cpu.X86).[math.big.«struct#08e4e4b9», name] =
        (globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», name] := by
    have hbig := go.structFieldRef_step math.big.«struct#08e4e4b9».underlying name
      (globalAddr internal.cpu.X86) (V := math.big.«struct#08e4e4b9»)
    have hcpu := go.structFieldRef_step internal.cpu.«struct#08e4e4b9».underlying name
      (globalAddr internal.cpu.X86) (V := internal.cpu.«struct#08e4e4b9»)
    have hb := hbig.isGoStep_det_internal.isGoStep_pure_det
    have hc := hcpu.isGoStep_det_internal.isGoStep_pure_det
    rw [← Htype] at hc
    have heq := congrFun (hb.symm.trans hc)
      (Val #((globalAddr internal.cpu.X86).[internal.cpu.«struct#08e4e4b9», name]))
    have hv := heq.mpr rfl
    exact go.intoVal_inj (Expr.Val.inj hv)
  exact ⟨hfield _, hfield _⟩

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_package_init_already
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_cpu_initialize_ready
#print axioms Perennial.github_com.NethermindEth.juno.core.cpu_feature_fields_match
