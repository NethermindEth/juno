import ProofCheck.StateFields
import ProofCheck.CursorInitialize

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Publish an actually stored table and retain both the global and backing
    storage as immutable resources. -/
theorem stateTable_publish (schema : StateSchema) (fields : Loc) :
    (globalAddr (stateGlobal schema) ↦ fields ∗
     StaticFieldsShared fields (stateKeys schema) (stateNames schema) (stateMask schema) : IProp GF) ⊢
      |==> StateTable (GF := GF) schema := by
  iintro ⟨Hglobal, #Hfields⟩
  ipersist Hglobal
  imodintro
  isimp only [StateTable]
  iexists fields
  iframe Hglobal Hfields

/-- Actual state-package startup publishes all 2 tables after the
    checked cursor dependency. Modeled runtime CPU startup remains a precondition. -/
theorem wp_state_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cborstate get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cborstate.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborstate.cborStateUpdateFields
    (go.PointerType cursor.Fields.ty) as HstateUpdateGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborstate.cborStateDiffFields
    (go.PointerType cursor.Fields.ty) as HstateDiffGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  iapply wp_fupd
  wp_apply wp_makeStateDiffFields_shared as %stateDiffFields #HstateDiffFields
  wp_apply wp_makeStateUpdateFields_shared as %stateUpdateFields #HstateUpdateFields
  imod stateTable_publish .stateDiff stateDiffFields $$ [HstateDiffGlobal] with #HstateDiffTable
  · isimp only [stateGlobal]
    iframe HstateDiffGlobal HstateDiffFields
  imod stateTable_publish .stateUpdate stateUpdateFields $$ [HstateUpdateGlobal] with #HstateUpdateTable
  · isimp only [stateGlobal]
    iframe HstateUpdateGlobal HstateUpdateFields
  ihave #Hmetadata : (StateMetadata (GF := GF)) $$ []
  · isimp only [StateMetadata]
    iframe #
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed initialization yields the exact published state metadata. -/
theorem state_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ⊢
      StateMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Any state reader can borrow its own table from the complete initialized
    package without assuming that allocation factories have already run. -/
theorem state_initialized_table (schema : StateSchema) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborstate ⊢
      StateTable (GF := GF) schema := by
  iintro #Hpkg
  ihave Hmetadata := state_initialized_metadata $$ Hpkg
  isimp only [StateMetadata] at Hmetadata
  icases Hmetadata with ⟨#HstateDiff, #HstateUpdate⟩
  cases schema <;> iframe #

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateTable_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_state_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.state_initialized_metadata
#print axioms Perennial.github_com.NethermindEth.juno.core.state_initialized_table
