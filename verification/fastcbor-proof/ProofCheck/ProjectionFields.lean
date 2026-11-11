import ProofCheck.ProjectionState
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

/-- The actual generated HeaderHash factory allocates the specified table. -/
theorem wp_makeHeaderHashProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderHashProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .headerHash) ∗
       names ↦* projectionNames .headerHash ∗ order ↦* projectionKeys .headerHash }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .headerHash)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .headerHash)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (1 : Int) 0) 0 = 1 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 1) (W64 1)) (slice.mk order (W64 1) (W64 1))
    (by change 0 ≤ sint.Z (W64 1); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeHeaderHashProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderHashProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .headerHash) (projectionNames .headerHash) (projectionMask .headerHash) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderHashProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .headerHash))
    (projectionKeys .headerHash) (projectionNames .headerHash) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated HeaderGlobalStateRoot factory allocates the specified table. -/
theorem wp_makeHeaderGlobalStateRootProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderGlobalStateRootProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .headerGlobalStateRoot) ∗
       names ↦* projectionNames .headerGlobalStateRoot ∗ order ↦* projectionKeys .headerGlobalStateRoot }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .headerGlobalStateRoot)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .headerGlobalStateRoot)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (1 : Int) 0) 0 = 1 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 1) (W64 1)) (slice.mk order (W64 1) (W64 1))
    (by change 0 ≤ sint.Z (W64 1); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeHeaderGlobalStateRootProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderGlobalStateRootProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .headerGlobalStateRoot) (projectionNames .headerGlobalStateRoot) (projectionMask .headerGlobalStateRoot) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderGlobalStateRootProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .headerGlobalStateRoot))
    (projectionKeys .headerGlobalStateRoot) (projectionNames .headerGlobalStateRoot) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated HeaderTransactionCount factory allocates the specified table. -/
theorem wp_makeHeaderTransactionCountProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderTransactionCountProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .headerTransactionCount) ∗
       names ↦* projectionNames .headerTransactionCount ∗ order ↦* projectionKeys .headerTransactionCount }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .headerTransactionCount)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .headerTransactionCount)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (1 : Int) 0) 0 = 1 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 1) (W64 1)) (slice.mk order (W64 1) (W64 1))
    (by change 0 ≤ sint.Z (W64 1); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeHeaderTransactionCountProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderTransactionCountProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .headerTransactionCount) (projectionNames .headerTransactionCount) (projectionMask .headerTransactionCount) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderTransactionCountProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .headerTransactionCount))
    (projectionKeys .headerTransactionCount) (projectionNames .headerTransactionCount) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated HeaderTimestamp factory allocates the specified table. -/
theorem wp_makeHeaderTimestampProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderTimestampProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .headerTimestamp) ∗
       names ↦* projectionNames .headerTimestamp ∗ order ↦* projectionKeys .headerTimestamp }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .headerTimestamp)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .headerTimestamp)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (1 : Int) 0) 0 = 1 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 1) (W64 1)) (slice.mk order (W64 1) (W64 1))
    (by change 0 ≤ sint.Z (W64 1); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeHeaderTimestampProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderTimestampProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .headerTimestamp) (projectionNames .headerTimestamp) (projectionMask .headerTimestamp) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderTimestampProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .headerTimestamp))
    (projectionKeys .headerTimestamp) (projectionNames .headerTimestamp) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated HeaderHashAndStateRoot factory allocates the specified table. -/
theorem wp_makeHeaderHashAndStateRootProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderHashAndStateRootProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .headerHashAndStateRoot) ∗
       names ↦* projectionNames .headerHashAndStateRoot ∗ order ↦* projectionKeys .headerHashAndStateRoot }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .headerHashAndStateRoot)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .headerHashAndStateRoot)
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
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeHeaderHashAndStateRootProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeHeaderHashAndStateRootProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .headerHashAndStateRoot) (projectionNames .headerHashAndStateRoot) (projectionMask .headerHashAndStateRoot) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderHashAndStateRootProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .headerHashAndStateRoot))
    (projectionKeys .headerHashAndStateRoot) (projectionNames .headerHashAndStateRoot) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated ReceiptExecutionStatus factory allocates the specified table. -/
theorem wp_makeReceiptExecutionStatusProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeReceiptExecutionStatusProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .receiptExecutionStatus) ∗
       names ↦* projectionNames .receiptExecutionStatus ∗ order ↦* projectionKeys .receiptExecutionStatus }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .receiptExecutionStatus)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .receiptExecutionStatus)
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
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeReceiptExecutionStatusProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeReceiptExecutionStatusProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .receiptExecutionStatus) (projectionNames .receiptExecutionStatus) (projectionMask .receiptExecutionStatus) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeReceiptExecutionStatusProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .receiptExecutionStatus))
    (projectionKeys .receiptExecutionStatus) (projectionNames .receiptExecutionStatus) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated ReceiptEvents factory allocates the specified table. -/
theorem wp_makeReceiptEventsProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeReceiptEventsProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (projectionMask .receiptEvents) ∗
       names ↦* projectionNames .receiptEvents ∗ order ↦* projectionKeys .receiptEvents }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (projectionNames .receiptEvents)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (projectionKeys .receiptEvents)
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
  isimp only [projectionMask, projectionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeReceiptEventsProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborprojections.cborMakeReceiptEventsProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (projectionKeys .receiptEvents) (projectionNames .receiptEvents) (projectionMask .receiptEvents) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeReceiptEventsProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (projectionMask .receiptEvents))
    (projectionKeys .receiptEvents) (projectionNames .receiptEvents) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared


end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderHashProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderHashProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderGlobalStateRootProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderGlobalStateRootProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderTransactionCountProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderTransactionCountProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderTimestampProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderTimestampProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderHashAndStateRootProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderHashAndStateRootProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeReceiptExecutionStatusProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeReceiptExecutionStatusProjectionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeReceiptEventsProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeReceiptEventsProjectionFields_shared
