import ProofCheck.RuntimeInitialize
import ProofCheck.BinaryInitialize
import Perennial.Proof.fmt.initialize

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

/-- Execute the actual translated math/big initializer after runtime CPU startup.
    Both CPU flag branches, all literal allocations, interface conversions,
    errors.New calls and stores are proved. No initializer WP is a premise.
    This establishes package startup, not unrelated math/big operations. -/
theorem wp_big_initialize_ready (g : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.math.big g) :
    {{ ownInitializing g ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val math.big.initialize') (Val #()))
    {{ RET #(); ownInitializing g ∗ isPkgInit (PROP := IProp GF) pkg_id.math.big }} := by
  have herrors := errors.initialize'.wp_proof (GF := GF) errors.New.wp_proof
  have hsync := sync.initialize'.wp_proof (GF := GF)
    internal.synctest.initialize'.wp_proof internal.race.initialize'.wp_proof sync.atomic.initialize'.wp_proof
  have hfmt := fmt.initialize'.wp_proof hsync (io.initialize'.wp_proof hsync herrors) herrors errors.New.wp_proof
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  repeat (first
    | wp_apply wp_GlobalAlloc (V := GoArray w8 _) _ (go.ArrayType _ go.uint8) as _
    | wp_apply wp_GlobalAlloc (V := GoArray w64 _) _ (go.ArrayType _ go.uint64) as _
    | wp_apply wp_GlobalAlloc (V := Bool) _ go.bool as _
    | wp_apply wp_GlobalAlloc (V := math.big.Float) _ math.big.Float.ty as _
    | wp_apply wp_GlobalAlloc (V := math.big.Rat) _ math.big.Rat.ty as _
    | wp_apply wp_GlobalAlloc (V := Loc) _ (go.PointerType _) as _
    | wp_apply wp_GlobalAlloc (V := GoSlice) _ math.big.nat.ty as _
    | wp_apply wp_GlobalAlloc (V := GoInterface) _ go.error as _
    | wp_apply wp_GlobalAlloc (V := w64) _ go.int as _)
  wp_apply herrors _ Hinit.2.2.2.1 $$ Hown as ⟨Hown, #Herrors⟩
  wp_apply hfmt _ Hinit.2.2.1 $$ Hown as ⟨Hown, #Hfmt⟩
  wp_apply wp_cpu_initialize_ready _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcpu⟩
  · iframe Hown Hready
  ihave Hcontents := isPkgInit_access pkg_id.internal.cpu $$ Hcpu
  isimp only [isPkgInitDef, cpuInit, RuntimeCPUFlags] at Hcontents
  icases Hcontents with ⟨%adx, %bmi2, #Hadx, #Hbmi2⟩
  isimp only [← cpu_feature_fields_match.1] at Hadx
  isimp only [← cpu_feature_fields_match.2] at Hbmi2
  wp_auto
  wp_if_destruct
  all_goals
    repeat (first
      | wp_apply errors.wp_New_errorString as %_ _
      | (wp_apply wp_slice_literal (V := w64) [_]
         isplitr
         · ipureintro; rfl
         iintro %_ ⟨_, _⟩)
      | wp_alloc p as Hp
      | wp_auto)
    iframe Hown
    is_pkg_init_finish

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_big_initialize_ready
