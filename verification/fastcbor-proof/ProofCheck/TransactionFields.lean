import ProofCheck.TransactionState
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

/-- The actual generated ResourceBounds factory allocates the specified table. -/
theorem wp_makeResourceBoundsFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeResourceBoundsFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .resourceBounds) ∗
       names ↦* transactionNames .resourceBounds ∗ order ↦* transactionKeys .resourceBounds }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .resourceBounds)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .resourceBounds)
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
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeResourceBoundsFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeResourceBoundsFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .resourceBounds) (transactionNames .resourceBounds) (transactionMask .resourceBounds) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeResourceBoundsFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .resourceBounds))
    (transactionKeys .resourceBounds) (transactionNames .resourceBounds) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated DeclareTransaction factory allocates the specified table. -/
theorem wp_makeDeclareTransactionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeclareTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .declareTransaction) ∗
       names ↦* transactionNames .declareTransaction ∗ order ↦* transactionKeys .declareTransaction }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .declareTransaction)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .declareTransaction)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (14 : Int) 0) 0 = 14 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 14) (W64 14)) (slice.mk order (W64 14) (W64 14))
    (by change 0 ≤ sint.Z (W64 14); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDeclareTransactionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeclareTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .declareTransaction) (transactionNames .declareTransaction) (transactionMask .declareTransaction) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDeclareTransactionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .declareTransaction))
    (transactionKeys .declareTransaction) (transactionNames .declareTransaction) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated DeployTransaction factory allocates the specified table. -/
theorem wp_makeDeployTransactionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeployTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .deployTransaction) ∗
       names ↦* transactionNames .deployTransaction ∗ order ↦* transactionKeys .deployTransaction }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .deployTransaction)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .deployTransaction)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (6 : Int) 0) 0 = 6 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 6) (W64 6)) (slice.mk order (W64 6) (W64 6))
    (by change 0 ≤ sint.Z (W64 6); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDeployTransactionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeployTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .deployTransaction) (transactionNames .deployTransaction) (transactionMask .deployTransaction) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDeployTransactionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .deployTransaction))
    (transactionKeys .deployTransaction) (transactionNames .deployTransaction) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated InvokeTransaction factory allocates the specified table. -/
theorem wp_makeInvokeTransactionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeInvokeTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .invokeTransaction) ∗
       names ↦* transactionNames .invokeTransaction ∗ order ↦* transactionKeys .invokeTransaction }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .invokeTransaction)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .invokeTransaction)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (16 : Int) 0) 0 = 16 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 16) (W64 16)) (slice.mk order (W64 16) (W64 16))
    (by change 0 ≤ sint.Z (W64 16); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeInvokeTransactionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeInvokeTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .invokeTransaction) (transactionNames .invokeTransaction) (transactionMask .invokeTransaction) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeInvokeTransactionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .invokeTransaction))
    (transactionKeys .invokeTransaction) (transactionNames .invokeTransaction) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated L1HandlerTransaction factory allocates the specified table. -/
theorem wp_makeL1HandlerTransactionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeL1HandlerTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .l1HandlerTransaction) ∗
       names ↦* transactionNames .l1HandlerTransaction ∗ order ↦* transactionKeys .l1HandlerTransaction }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .l1HandlerTransaction)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .l1HandlerTransaction)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (6 : Int) 0) 0 = 6 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 6) (W64 6)) (slice.mk order (W64 6) (W64 6))
    (by change 0 ≤ sint.Z (W64 6); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeL1HandlerTransactionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeL1HandlerTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .l1HandlerTransaction) (transactionNames .l1HandlerTransaction) (transactionMask .l1HandlerTransaction) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeL1HandlerTransactionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .l1HandlerTransaction))
    (transactionKeys .l1HandlerTransaction) (transactionNames .l1HandlerTransaction) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated DeployAccountTransaction factory allocates the specified table. -/
theorem wp_makeDeployAccountTransactionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeployAccountTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .deployAccountTransaction) ∗
       names ↦* transactionNames .deployAccountTransaction ∗ order ↦* transactionKeys .deployAccountTransaction }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .deployAccountTransaction)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .deployAccountTransaction)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (14 : Int) 0) 0 = 14 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 14) (W64 14)) (slice.mk order (W64 14) (W64 14))
    (by change 0 ≤ sint.Z (W64 14); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDeployAccountTransactionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeDeployAccountTransactionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .deployAccountTransaction) (transactionNames .deployAccountTransaction) (transactionMask .deployAccountTransaction) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDeployAccountTransactionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .deployAccountTransaction))
    (transactionKeys .deployAccountTransaction) (transactionNames .deployAccountTransaction) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated TransactionHashProjection factory allocates the specified table. -/
theorem wp_makeTransactionHashProjectionFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeTransactionHashProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (transactionMask .transactionHashProjection) ∗
       names ↦* transactionNames .transactionHashProjection ∗ order ↦* transactionKeys .transactionHashProjection }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (transactionNames .transactionHashProjection)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (transactionKeys .transactionHashProjection)
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
  isimp only [transactionMask, transactionNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeTransactionHashProjectionFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cbortransactions.cborMakeTransactionHashProjectionFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (transactionKeys .transactionHashProjection) (transactionNames .transactionHashProjection) (transactionMask .transactionHashProjection) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeTransactionHashProjectionFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (transactionMask .transactionHashProjection))
    (transactionKeys .transactionHashProjection) (transactionNames .transactionHashProjection) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeResourceBoundsFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeResourceBoundsFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeclareTransactionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeclareTransactionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeployTransactionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeployTransactionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeInvokeTransactionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeInvokeTransactionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL1HandlerTransactionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeL1HandlerTransactionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeployAccountTransactionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeployAccountTransactionFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeTransactionHashProjectionFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeTransactionHashProjectionFields_shared
