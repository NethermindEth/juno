import ProofCheck.ReceiptState
import ProofCheck.FieldsShared

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

attribute [-instance] go.SliceSemantics.composite_literal_slice

/-- The actual generated BuiltinInstanceCounter factory allocates the specified table. -/
theorem wp_makeBuiltinInstanceCounterFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeBuiltinInstanceCounterFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .builtinInstanceCounter) ∗
       names ↦* receiptNames .builtinInstanceCounter ∗ order ↦* receiptKeys .builtinInstanceCounter }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .builtinInstanceCounter)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .builtinInstanceCounter)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (12 : Int) 0) 0 = 12 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 12) (W64 12)) (slice.mk order (W64 12) (W64 12))
    (by change 0 ≤ sint.Z (W64 12); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeBuiltinInstanceCounterFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeBuiltinInstanceCounterFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .builtinInstanceCounter) (receiptNames .builtinInstanceCounter) (receiptMask .builtinInstanceCounter) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeBuiltinInstanceCounterFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .builtinInstanceCounter))
    (receiptKeys .builtinInstanceCounter) (receiptNames .builtinInstanceCounter) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated DataAvailability factory allocates the specified table. -/
theorem wp_makeDataAvailabilityFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeDataAvailabilityFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .dataAvailability) ∗
       names ↦* receiptNames .dataAvailability ∗ order ↦* receiptKeys .dataAvailability }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .dataAvailability)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .dataAvailability)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (2 : Int) 0) 0 = 2 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 2) (W64 2)) (slice.mk order (W64 2) (W64 2))
    (by change 0 ≤ sint.Z (W64 2); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDataAvailabilityFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeDataAvailabilityFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .dataAvailability) (receiptNames .dataAvailability) (receiptMask .dataAvailability) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDataAvailabilityFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .dataAvailability))
    (receiptKeys .dataAvailability) (receiptNames .dataAvailability) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated GasConsumed factory allocates the specified table. -/
theorem wp_makeGasConsumedFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeGasConsumedFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .gasConsumed) ∗
       names ↦* receiptNames .gasConsumed ∗ order ↦* receiptKeys .gasConsumed }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .gasConsumed)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .gasConsumed)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (3 : Int) 0) 0 = 3 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 3) (W64 3)) (slice.mk order (W64 3) (W64 3))
    (by change 0 ≤ sint.Z (W64 3); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeGasConsumedFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeGasConsumedFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .gasConsumed) (receiptNames .gasConsumed) (receiptMask .gasConsumed) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeGasConsumedFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .gasConsumed))
    (receiptKeys .gasConsumed) (receiptNames .gasConsumed) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated ExecutionResources factory allocates the specified table. -/
theorem wp_makeExecutionResourcesFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeExecutionResourcesFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .executionResources) ∗
       names ↦* receiptNames .executionResources ∗ order ↦* receiptKeys .executionResources }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .executionResources)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .executionResources)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (5 : Int) 0) 0 = 5 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 5) (W64 5)) (slice.mk order (W64 5) (W64 5))
    (by change 0 ≤ sint.Z (W64 5); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeExecutionResourcesFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeExecutionResourcesFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .executionResources) (receiptNames .executionResources) (receiptMask .executionResources) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeExecutionResourcesFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .executionResources))
    (receiptKeys .executionResources) (receiptNames .executionResources) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated Event factory allocates the specified table. -/
theorem wp_makeEventFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeEventFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .event) ∗
       names ↦* receiptNames .event ∗ order ↦* receiptKeys .event }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .event)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .event)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (3 : Int) 0) 0 = 3 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 3) (W64 3)) (slice.mk order (W64 3) (W64 3))
    (by change 0 ≤ sint.Z (W64 3); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeEventFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeEventFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .event) (receiptNames .event) (receiptMask .event) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeEventFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .event))
    (receiptKeys .event) (receiptNames .event) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated L1ToL2Message factory allocates the specified table. -/
theorem wp_makeL1ToL2MessageFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeL1ToL2MessageFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .l1ToL2Message) ∗
       names ↦* receiptNames .l1ToL2Message ∗ order ↦* receiptKeys .l1ToL2Message }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .l1ToL2Message)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .l1ToL2Message)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (5 : Int) 0) 0 = 5 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 5) (W64 5)) (slice.mk order (W64 5) (W64 5))
    (by change 0 ≤ sint.Z (W64 5); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeL1ToL2MessageFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeL1ToL2MessageFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .l1ToL2Message) (receiptNames .l1ToL2Message) (receiptMask .l1ToL2Message) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeL1ToL2MessageFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .l1ToL2Message))
    (receiptKeys .l1ToL2Message) (receiptNames .l1ToL2Message) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated L2ToL1Message factory allocates the specified table. -/
theorem wp_makeL2ToL1MessageFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeL2ToL1MessageFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .l2ToL1Message) ∗
       names ↦* receiptNames .l2ToL1Message ∗ order ↦* receiptKeys .l2ToL1Message }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .l2ToL1Message)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .l2ToL1Message)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (3 : Int) 0) 0 = 3 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 3) (W64 3)) (slice.mk order (W64 3) (W64 3))
    (by change 0 ≤ sint.Z (W64 3); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeL2ToL1MessageFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeL2ToL1MessageFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .l2ToL1Message) (receiptNames .l2ToL1Message) (receiptMask .l2ToL1Message) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeL2ToL1MessageFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .l2ToL1Message))
    (receiptKeys .l2ToL1Message) (receiptNames .l2ToL1Message) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated TransactionReceipt factory allocates the specified table. -/
theorem wp_makeTransactionReceiptFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeTransactionReceiptFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (receiptMask .transactionReceipt) ∗
       names ↦* receiptNames .transactionReceipt ∗ order ↦* receiptKeys .transactionReceipt }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (receiptNames .transactionReceipt)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (receiptKeys .transactionReceipt)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (9 : Int) 0) 0 = 9 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 9) (W64 9)) (slice.mk order (W64 9) (W64 9))
    (by change 0 ≤ sint.Z (W64 9); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [receiptMask, receiptNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeTransactionReceiptFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborreceipts.cborMakeTransactionReceiptFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (receiptKeys .transactionReceipt) (receiptNames .transactionReceipt) (receiptMask .transactionReceipt) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeTransactionReceiptFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (receiptMask .transactionReceipt))
    (receiptKeys .transactionReceipt) (receiptNames .transactionReceipt) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeBuiltinInstanceCounterFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeBuiltinInstanceCounterFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDataAvailabilityFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDataAvailabilityFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeGasConsumedFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeGasConsumedFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeExecutionResourcesFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeExecutionResourcesFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeEventFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeEventFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL1ToL2MessageFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL1ToL2MessageFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL2ToL1MessageFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL2ToL1MessageFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeTransactionReceiptFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeTransactionReceiptFields_shared
