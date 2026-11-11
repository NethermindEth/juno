import ProofCheck.TransactionFields
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
theorem transactionTable_publish (schema : TransactionSchema) (fields : Loc) :
    (globalAddr (transactionGlobal schema) ↦ fields ∗
     StaticFieldsShared fields (transactionKeys schema) (transactionNames schema) (transactionMask schema) : IProp GF) ⊢
      |==> TransactionTable (GF := GF) schema := by
  iintro ⟨Hglobal, #Hfields⟩
  ipersist Hglobal
  imodintro
  isimp only [TransactionTable]
  iexists fields
  iframe Hglobal Hfields

/-- Actual transaction-package startup publishes all seven tables after the
    checked cursor dependency. Modeled runtime CPU startup remains a precondition. -/
theorem wp_transaction_initialize (get_is_pkg_init : GoString → IProp GF)
    (Hinit : GetIsPkgInitProp pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions get_is_pkg_init) :
    {{ ownInitializing get_is_pkg_init ∗ PackageInitialized pkg_id.internal.cpu }}
      (App (Val internal.cbortransactions.initialize') (Val #()))
    {{ RET #(); ownInitializing get_is_pkg_init ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions }} := by
  wp_start as ⟨Hown, #Hready⟩
  iapply wp_package_init (heq := Hinit.1) $$ [Hown] HΦ
  iframe Hown
  iintro Hown
  wp_auto
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborTransactionHashProjectionFields
    (go.PointerType cursor.Fields.ty) as HtransactionHashProjectionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborDeployAccountTransactionFields
    (go.PointerType cursor.Fields.ty) as HdeployAccountTransactionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborL1HandlerTransactionFields
    (go.PointerType cursor.Fields.ty) as Hl1HandlerTransactionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborInvokeTransactionFields
    (go.PointerType cursor.Fields.ty) as HinvokeTransactionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborDeployTransactionFields
    (go.PointerType cursor.Fields.ty) as HdeployTransactionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborDeclareTransactionFields
    (go.PointerType cursor.Fields.ty) as HdeclareTransactionGlobal
  wp_apply wp_GlobalAlloc (V := Loc) internal.cbortransactions.cborResourceBoundsFields
    (go.PointerType cursor.Fields.ty) as HresourceBoundsGlobal
  wp_apply wp_cursor_initialize _ Hinit.2.1 $$ [Hown] as ⟨Hown, #Hcursor⟩
  · iframe Hown Hready
  iapply wp_fupd
  wp_apply wp_makeResourceBoundsFields_shared as %resourceBoundsFields #HresourceBoundsFields
  wp_apply wp_makeDeclareTransactionFields_shared as %declareTransactionFields #HdeclareTransactionFields
  wp_apply wp_makeDeployTransactionFields_shared as %deployTransactionFields #HdeployTransactionFields
  wp_apply wp_makeInvokeTransactionFields_shared as %invokeTransactionFields #HinvokeTransactionFields
  wp_apply wp_makeL1HandlerTransactionFields_shared as %l1HandlerTransactionFields #Hl1HandlerTransactionFields
  wp_apply wp_makeDeployAccountTransactionFields_shared as %deployAccountTransactionFields #HdeployAccountTransactionFields
  wp_apply wp_makeTransactionHashProjectionFields_shared as %transactionHashProjectionFields #HtransactionHashProjectionFields
  imod transactionTable_publish .resourceBounds resourceBoundsFields $$ [HresourceBoundsGlobal] with #HresourceBoundsTable
  · isimp only [transactionGlobal]
    iframe HresourceBoundsGlobal HresourceBoundsFields
  imod transactionTable_publish .declareTransaction declareTransactionFields $$ [HdeclareTransactionGlobal] with #HdeclareTransactionTable
  · isimp only [transactionGlobal]
    iframe HdeclareTransactionGlobal HdeclareTransactionFields
  imod transactionTable_publish .deployTransaction deployTransactionFields $$ [HdeployTransactionGlobal] with #HdeployTransactionTable
  · isimp only [transactionGlobal]
    iframe HdeployTransactionGlobal HdeployTransactionFields
  imod transactionTable_publish .invokeTransaction invokeTransactionFields $$ [HinvokeTransactionGlobal] with #HinvokeTransactionTable
  · isimp only [transactionGlobal]
    iframe HinvokeTransactionGlobal HinvokeTransactionFields
  imod transactionTable_publish .l1HandlerTransaction l1HandlerTransactionFields $$ [Hl1HandlerTransactionGlobal] with #Hl1HandlerTransactionTable
  · isimp only [transactionGlobal]
    iframe Hl1HandlerTransactionGlobal Hl1HandlerTransactionFields
  imod transactionTable_publish .deployAccountTransaction deployAccountTransactionFields $$ [HdeployAccountTransactionGlobal] with #HdeployAccountTransactionTable
  · isimp only [transactionGlobal]
    iframe HdeployAccountTransactionGlobal HdeployAccountTransactionFields
  imod transactionTable_publish .transactionHashProjection transactionHashProjectionFields $$ [HtransactionHashProjectionGlobal] with #HtransactionHashProjectionTable
  · isimp only [transactionGlobal]
    iframe HtransactionHashProjectionGlobal HtransactionHashProjectionFields
  ihave #Hmetadata : (TransactionMetadata (GF := GF)) $$ []
  · isimp only [TransactionMetadata]
    iframe #
  imodintro
  iframe Hown
  is_pkg_init_finish

/-- Completed initialization yields the exact published transaction metadata. -/
theorem transaction_initialized_metadata :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ⊢
      TransactionMetadata (GF := GF) := by
  exact isPkgInit_access _

/-- Any transaction reader can borrow its own table from the complete initialized
    package without assuming that allocation factories have already run. -/
theorem transaction_initialized_table (schema : TransactionSchema) :
    isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ⊢
      TransactionTable (GF := GF) schema := by
  iintro #Hpkg
  ihave Hmetadata := transaction_initialized_metadata $$ Hpkg
  isimp only [TransactionMetadata] at Hmetadata
  icases Hmetadata with ⟨#HresourceBounds, #HdeclareTransaction, #HdeployTransaction, #HinvokeTransaction, #Hl1HandlerTransaction, #HdeployAccountTransaction, #HtransactionHashProjection⟩
  cases schema <;> iframe #

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionTable_publish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transaction_initialize
#print axioms Perennial.github_com.NethermindEth.juno.core.transaction_initialized_metadata
#print axioms Perennial.github_com.NethermindEth.juno.core.transaction_initialized_table
