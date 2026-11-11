import ProofCheck.ClassFields
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
theorem classTable_publish (schema : ClassSchema) (fields : Loc) :
    (globalAddr (classGlobal schema) ↦ fields ∗
     StaticFieldsShared fields (classKeys schema) (classNames schema) (classMask schema) : IProp GF) ⊢
      |==> ClassTable (GF := GF) schema := by
  iintro ⟨Hglobal, #Hfields⟩
  ipersist Hglobal
  imodintro
  isimp only [ClassTable]
  iexists fields
  iframe Hglobal Hfields

/-- Actual class-package startup publishes all 8 tables after the
    checked cursor dependency. Modeled runtime CPU startup remains a precondition. -/
theorem wp_class_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cborclasses.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborSierraClassFields
    (go.PointerType cursor.Fields.ty) as HsierraClassGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborCasmClassFields
    (go.PointerType cursor.Fields.ty) as HcasmClassGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborSegmentLengthsFields
    (go.PointerType cursor.Fields.ty) as HsegmentLengthsGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborCasmEntryPointFields
    (go.PointerType cursor.Fields.ty) as HcasmEntryPointGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborSierraEntryPointsByTypeFields
    (go.PointerType cursor.Fields.ty) as HsierraEntryPointsByTypeGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborSierraEntryPointFields
    (go.PointerType cursor.Fields.ty) as HsierraEntryPointGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborDeprecatedCairoClassFields
    (go.PointerType cursor.Fields.ty) as HdeprecatedCairoClassGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborclasses.cborDeprecatedEntryPointFields
    (go.PointerType cursor.Fields.ty) as HdeprecatedEntryPointGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  iapply wp_fupd
  wp_apply wp_makeDeprecatedEntryPointFields_shared as %deprecatedEntryPointFields #HdeprecatedEntryPointFields
  wp_apply wp_makeDeprecatedCairoClassFields_shared as %deprecatedCairoClassFields #HdeprecatedCairoClassFields
  wp_apply wp_makeSierraEntryPointFields_shared as %sierraEntryPointFields #HsierraEntryPointFields
  wp_apply wp_makeSierraEntryPointsByTypeFields_shared as %sierraEntryPointsByTypeFields #HsierraEntryPointsByTypeFields
  wp_apply wp_makeCasmEntryPointFields_shared as %casmEntryPointFields #HcasmEntryPointFields
  wp_apply wp_makeSegmentLengthsFields_shared as %segmentLengthsFields #HsegmentLengthsFields
  wp_apply wp_makeCasmClassFields_shared as %casmClassFields #HcasmClassFields
  wp_apply wp_makeSierraClassFields_shared as %sierraClassFields #HsierraClassFields
  imod classTable_publish .deprecatedEntryPoint deprecatedEntryPointFields $$ [HdeprecatedEntryPointGlobal] with #HdeprecatedEntryPointTable
  · isimp only [classGlobal]
    iframe HdeprecatedEntryPointGlobal HdeprecatedEntryPointFields
  imod classTable_publish .deprecatedCairoClass deprecatedCairoClassFields $$ [HdeprecatedCairoClassGlobal] with #HdeprecatedCairoClassTable
  · isimp only [classGlobal]
    iframe HdeprecatedCairoClassGlobal HdeprecatedCairoClassFields
  imod classTable_publish .sierraEntryPoint sierraEntryPointFields $$ [HsierraEntryPointGlobal] with #HsierraEntryPointTable
  · isimp only [classGlobal]
    iframe HsierraEntryPointGlobal HsierraEntryPointFields
  imod classTable_publish .sierraEntryPointsByType sierraEntryPointsByTypeFields $$ [HsierraEntryPointsByTypeGlobal] with #HsierraEntryPointsByTypeTable
  · isimp only [classGlobal]
    iframe HsierraEntryPointsByTypeGlobal HsierraEntryPointsByTypeFields
  imod classTable_publish .casmEntryPoint casmEntryPointFields $$ [HcasmEntryPointGlobal] with #HcasmEntryPointTable
  · isimp only [classGlobal]
    iframe HcasmEntryPointGlobal HcasmEntryPointFields
  imod classTable_publish .segmentLengths segmentLengthsFields $$ [HsegmentLengthsGlobal] with #HsegmentLengthsTable
  · isimp only [classGlobal]
    iframe HsegmentLengthsGlobal HsegmentLengthsFields
  imod classTable_publish .casmClass casmClassFields $$ [HcasmClassGlobal] with #HcasmClassTable
  · isimp only [classGlobal]
    iframe HcasmClassGlobal HcasmClassFields
  imod classTable_publish .sierraClass sierraClassFields $$ [HsierraClassGlobal] with #HsierraClassTable
  · isimp only [classGlobal]
    iframe HsierraClassGlobal HsierraClassFields
  ihave #Hmetadata : (ClassMetadata (GF := GF)) $$ []
  · isimp only [ClassMetadata]
    iframe #
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed initialization yields the exact published class metadata. -/
theorem class_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ⊢
      ClassMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Any class reader can borrow its own table from the complete initialized
    package without assuming that allocation factories have already run. -/
theorem class_initialized_table (schema : ClassSchema) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ⊢
      ClassTable (GF := GF) schema := by
  iintro #Hpkg
  ihave Hmetadata := class_initialized_metadata $$ Hpkg
  isimp only [ClassMetadata] at Hmetadata
  icases Hmetadata with ⟨#HdeprecatedEntryPoint, #HdeprecatedCairoClass, #HsierraEntryPoint, #HsierraEntryPointsByType, #HcasmEntryPoint, #HsegmentLengths, #HcasmClass, #HsierraClass⟩
  cases schema <;> iframe #

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classTable_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.class_initialized_metadata
#print axioms Perennial.github_com.NethermindEth.juno.core.class_initialized_table
