import ProofCheck.FieldsShared
import ProofCheck.CursorInitialize

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Complete startup of the decoder-owned schema package. Both actual factories
    execute, their results are stored in the actual globals, and the complete
    tables are retained immutably. All dependency initializers are proved, with
    the same explicit runtime CPU-startup precondition as cursor initialization. -/
theorem wp_schema_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cborschema get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cborschema.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborschema.cborHeaderFields
    (go.PointerType cursor.Fields.ty) as HheaderGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborschema.cborGasPriceFields
    (go.PointerType cursor.Fields.ty) as HgasGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  wp_apply wp_makeGasPriceFields_shared as %gasFields #HgasFields
  iapply wp_fupd
  wp_apply wp_makeHeaderFields_shared as %fields #Hfields
  imod headerMetadata_publish fields gasFields $$ [HheaderGlobal HgasGlobal] with #Hmetadata
  · iframe HheaderGlobal HgasGlobal Hfields HgasFields
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed schema startup yields the exact metadata needed by independent
    Header calls. This is derived from the proved initializer's postcondition. -/
theorem schema_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ⊢
      HeaderMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Header callers use the proved schema startup directly. Unrelated core and
    planner initialization are not preconditions of the decoder contracts. -/
theorem header_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ⊢
      HeaderMetadata (GF := GF) := by
  iintro #Hpkg
  iapply schema_initialized_metadata
  solve_pkg_init

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_schema_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.schema_initialized_metadata

#print axioms Perennial.github_com.NethermindEth.juno.core.header_initialized_metadata
