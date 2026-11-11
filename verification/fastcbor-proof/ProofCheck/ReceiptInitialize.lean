import ProofCheck.ReceiptFields
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
theorem receiptTable_publish (schema : ReceiptSchema) (fields : Loc) :
    (globalAddr (receiptGlobal schema) ↦ fields ∗
     StaticFieldsShared fields (receiptKeys schema) (receiptNames schema) (receiptMask schema) : IProp GF) ⊢
      |==> ReceiptTable (GF := GF) schema := by
  iintro ⟨Hglobal, #Hfields⟩
  ipersist Hglobal
  imodintro
  isimp only [ReceiptTable]
  iexists fields
  iframe Hglobal Hfields

/-- Complete actual receipt-package startup, including all eight receipt
    dependency tables and the checked cursor dependency. Runtime CPU startup
    has the same explicit boundary as the Header initialization proof. -/
theorem wp_receipt_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cborreceipts.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborTransactionReceiptFields
    (go.PointerType cursor.Fields.ty) as HtransactionReceiptGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborL2ToL1MessageFields
    (go.PointerType cursor.Fields.ty) as Hl2ToL1MessageGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborL1ToL2MessageFields
    (go.PointerType cursor.Fields.ty) as Hl1ToL2MessageGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborEventFields
    (go.PointerType cursor.Fields.ty) as HeventGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborExecutionResourcesFields
    (go.PointerType cursor.Fields.ty) as HexecutionResourcesGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborGasConsumedFields
    (go.PointerType cursor.Fields.ty) as HgasConsumedGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborDataAvailabilityFields
    (go.PointerType cursor.Fields.ty) as HdataAvailabilityGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cborreceipts.cborBuiltinInstanceCounterFields
    (go.PointerType cursor.Fields.ty) as HbuiltinInstanceCounterGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  iapply wp_fupd
  wp_apply wp_makeBuiltinInstanceCounterFields_shared as %builtinInstanceCounterFields #HbuiltinInstanceCounterFields
  wp_apply wp_makeDataAvailabilityFields_shared as %dataAvailabilityFields #HdataAvailabilityFields
  wp_apply wp_makeGasConsumedFields_shared as %gasConsumedFields #HgasConsumedFields
  wp_apply wp_makeExecutionResourcesFields_shared as %executionResourcesFields #HexecutionResourcesFields
  wp_apply wp_makeEventFields_shared as %eventFields #HeventFields
  wp_apply wp_makeL1ToL2MessageFields_shared as %l1ToL2MessageFields #Hl1ToL2MessageFields
  wp_apply wp_makeL2ToL1MessageFields_shared as %l2ToL1MessageFields #Hl2ToL1MessageFields
  wp_apply wp_makeTransactionReceiptFields_shared as %transactionReceiptFields #HtransactionReceiptFields
  imod receiptTable_publish .builtinInstanceCounter builtinInstanceCounterFields $$ [HbuiltinInstanceCounterGlobal] with #HbuiltinInstanceCounterTable
  · isimp only [receiptGlobal]
    iframe HbuiltinInstanceCounterGlobal HbuiltinInstanceCounterFields
  imod receiptTable_publish .dataAvailability dataAvailabilityFields $$ [HdataAvailabilityGlobal] with #HdataAvailabilityTable
  · isimp only [receiptGlobal]
    iframe HdataAvailabilityGlobal HdataAvailabilityFields
  imod receiptTable_publish .gasConsumed gasConsumedFields $$ [HgasConsumedGlobal] with #HgasConsumedTable
  · isimp only [receiptGlobal]
    iframe HgasConsumedGlobal HgasConsumedFields
  imod receiptTable_publish .executionResources executionResourcesFields $$ [HexecutionResourcesGlobal] with #HexecutionResourcesTable
  · isimp only [receiptGlobal]
    iframe HexecutionResourcesGlobal HexecutionResourcesFields
  imod receiptTable_publish .event eventFields $$ [HeventGlobal] with #HeventTable
  · isimp only [receiptGlobal]
    iframe HeventGlobal HeventFields
  imod receiptTable_publish .l1ToL2Message l1ToL2MessageFields $$ [Hl1ToL2MessageGlobal] with #Hl1ToL2MessageTable
  · isimp only [receiptGlobal]
    iframe Hl1ToL2MessageGlobal Hl1ToL2MessageFields
  imod receiptTable_publish .l2ToL1Message l2ToL1MessageFields $$ [Hl2ToL1MessageGlobal] with #Hl2ToL1MessageTable
  · isimp only [receiptGlobal]
    iframe Hl2ToL1MessageGlobal Hl2ToL1MessageFields
  imod receiptTable_publish .transactionReceipt transactionReceiptFields $$ [HtransactionReceiptGlobal] with #HtransactionReceiptTable
  · isimp only [receiptGlobal]
    iframe HtransactionReceiptGlobal HtransactionReceiptFields
  ihave #Hmetadata : (ReceiptMetadata (GF := GF)) $$ []
  · isimp only [ReceiptMetadata]
    iframe #
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed initialization yields the exact published receipt metadata. -/
theorem receipt_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ⊢
      ReceiptMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Any receipt reader can borrow its own table from the complete initialized
    package without assuming that allocation factories have already run. -/
theorem receipt_initialized_table (schema : ReceiptSchema) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ⊢
      ReceiptTable (GF := GF) schema := by
  iintro #Hpkg
  ihave Hmetadata := receipt_initialized_metadata $$ Hpkg
  isimp only [ReceiptMetadata] at Hmetadata
  icases Hmetadata with ⟨#HbuiltinInstanceCounter, #HdataAvailability, #HgasConsumed, #HexecutionResources, #Hevent, #Hl1ToL2Message, #Hl2ToL1Message, #HtransactionReceipt⟩
  cases schema <;> iframe #

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptTable_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receipt_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_initialized_metadata
#print axioms Perennial.github_com.NethermindEth.juno.core.receipt_initialized_table
