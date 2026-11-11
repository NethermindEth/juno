import ProofCheck.ProjectionFields
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
theorem projectionTable_publish (schema : ProjectionSchema) (fields : Loc) :
    (globalAddr (projectionGlobal schema) ↦ fields ∗
     StaticFieldsShared fields (projectionKeys schema) (projectionNames schema) (projectionMask schema) : IProp GF) ⊢
      |==> ProjectionTable (GF := GF) schema := by
  iintro ⟨Hglobal, #Hfields⟩
  ipersist Hglobal
  imodintro
  isimp only [ProjectionTable]
  iexists fields
  iframe Hglobal Hfields

/-- Complete actual projection-package startup, including all five Header and
    both receipt tables and the checked cursor dependency. Runtime CPU startup
    has the same explicit boundary as the Header initialization proof. -/
theorem wp_projection_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cborprojections.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborReceiptEventsProjectionFields
    (go.PointerType cursor.Fields.ty) as HreceiptEventsGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborReceiptExecutionStatusProjectionFields
    (go.PointerType cursor.Fields.ty) as HreceiptExecutionStatusGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborHeaderHashAndStateRootProjectionFields
    (go.PointerType cursor.Fields.ty) as HheaderHashAndStateRootGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborHeaderTimestampProjectionFields
    (go.PointerType cursor.Fields.ty) as HheaderTimestampGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborHeaderTransactionCountProjectionFields
    (go.PointerType cursor.Fields.ty) as HheaderTransactionCountGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborHeaderGlobalStateRootProjectionFields
    (go.PointerType cursor.Fields.ty) as HheaderGlobalStateRootGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborprojections.cborHeaderHashProjectionFields
    (go.PointerType cursor.Fields.ty) as HheaderHashGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  iapply wp_fupd
  wp_apply wp_makeHeaderHashProjectionFields_shared as %headerHashFields #HheaderHashFields
  wp_apply wp_makeHeaderGlobalStateRootProjectionFields_shared as %headerGlobalStateRootFields #HheaderGlobalStateRootFields
  wp_apply wp_makeHeaderTransactionCountProjectionFields_shared as %headerTransactionCountFields #HheaderTransactionCountFields
  wp_apply wp_makeHeaderTimestampProjectionFields_shared as %headerTimestampFields #HheaderTimestampFields
  wp_apply wp_makeHeaderHashAndStateRootProjectionFields_shared as %headerHashAndStateRootFields #HheaderHashAndStateRootFields
  wp_apply wp_makeReceiptExecutionStatusProjectionFields_shared as %receiptExecutionStatusFields #HreceiptExecutionStatusFields
  wp_apply wp_makeReceiptEventsProjectionFields_shared as %receiptEventsFields #HreceiptEventsFields
  imod projectionTable_publish .headerHash headerHashFields $$ [HheaderHashGlobal] with #HheaderHashTable
  · isimp only [projectionGlobal]
    iframe HheaderHashGlobal HheaderHashFields
  imod projectionTable_publish .headerGlobalStateRoot headerGlobalStateRootFields $$ [HheaderGlobalStateRootGlobal] with #HheaderGlobalStateRootTable
  · isimp only [projectionGlobal]
    iframe HheaderGlobalStateRootGlobal HheaderGlobalStateRootFields
  imod projectionTable_publish .headerTransactionCount headerTransactionCountFields $$ [HheaderTransactionCountGlobal] with #HheaderTransactionCountTable
  · isimp only [projectionGlobal]
    iframe HheaderTransactionCountGlobal HheaderTransactionCountFields
  imod projectionTable_publish .headerTimestamp headerTimestampFields $$ [HheaderTimestampGlobal] with #HheaderTimestampTable
  · isimp only [projectionGlobal]
    iframe HheaderTimestampGlobal HheaderTimestampFields
  imod projectionTable_publish .headerHashAndStateRoot headerHashAndStateRootFields $$ [HheaderHashAndStateRootGlobal] with #HheaderHashAndStateRootTable
  · isimp only [projectionGlobal]
    iframe HheaderHashAndStateRootGlobal HheaderHashAndStateRootFields
  imod projectionTable_publish .receiptExecutionStatus receiptExecutionStatusFields $$ [HreceiptExecutionStatusGlobal] with #HreceiptExecutionStatusTable
  · isimp only [projectionGlobal]
    iframe HreceiptExecutionStatusGlobal HreceiptExecutionStatusFields
  imod projectionTable_publish .receiptEvents receiptEventsFields $$ [HreceiptEventsGlobal] with #HreceiptEventsTable
  · isimp only [projectionGlobal]
    iframe HreceiptEventsGlobal HreceiptEventsFields
  ihave #Hmetadata : (ProjectionMetadata (GF := GF)) $$ []
  · isimp only [ProjectionMetadata]
    iframe #
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed initialization yields the exact published projection metadata. -/
theorem projection_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ⊢
      ProjectionMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Any projection reader can borrow its own table from the complete initialized
    package without assuming that allocation factories have already run. -/
theorem projection_initialized_table (schema : ProjectionSchema) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ⊢
      ProjectionTable (GF := GF) schema := by
  iintro #Hpkg
  ihave Hmetadata := projection_initialized_metadata $$ Hpkg
  isimp only [ProjectionMetadata] at Hmetadata
  icases Hmetadata with ⟨#Hhash, #Hroot, #Hcount, #Htimestamp, #Hboth, #Hstatus, #Hevents⟩
  cases schema <;> iframe #

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionTable_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projection_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.projection_initialized_metadata
#print axioms Perennial.github_com.NethermindEth.juno.core.projection_initialized_table
