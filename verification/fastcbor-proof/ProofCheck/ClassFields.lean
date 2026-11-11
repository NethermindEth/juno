import ProofCheck.ClassMetadata
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

/-- The actual generated DeprecatedEntryPoint factory allocates the specified table. -/
theorem wp_makeDeprecatedEntryPointFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeDeprecatedEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .deprecatedEntryPoint) ∗
       names ↦* classNames .deprecatedEntryPoint ∗ order ↦* classKeys .deprecatedEntryPoint }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .deprecatedEntryPoint)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .deprecatedEntryPoint)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDeprecatedEntryPointFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeDeprecatedEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .deprecatedEntryPoint) (classNames .deprecatedEntryPoint) (classMask .deprecatedEntryPoint) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDeprecatedEntryPointFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .deprecatedEntryPoint))
    (classKeys .deprecatedEntryPoint) (classNames .deprecatedEntryPoint) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated DeprecatedCairoClass factory allocates the specified table. -/
theorem wp_makeDeprecatedCairoClassFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeDeprecatedCairoClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .deprecatedCairoClass) ∗
       names ↦* classNames .deprecatedCairoClass ∗ order ↦* classKeys .deprecatedCairoClass }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .deprecatedCairoClass)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .deprecatedCairoClass)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeDeprecatedCairoClassFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeDeprecatedCairoClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .deprecatedCairoClass) (classNames .deprecatedCairoClass) (classMask .deprecatedCairoClass) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeDeprecatedCairoClassFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .deprecatedCairoClass))
    (classKeys .deprecatedCairoClass) (classNames .deprecatedCairoClass) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated SierraEntryPoint factory allocates the specified table. -/
theorem wp_makeSierraEntryPointFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .sierraEntryPoint) ∗
       names ↦* classNames .sierraEntryPoint ∗ order ↦* classKeys .sierraEntryPoint }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .sierraEntryPoint)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .sierraEntryPoint)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeSierraEntryPointFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .sierraEntryPoint) (classNames .sierraEntryPoint) (classMask .sierraEntryPoint) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeSierraEntryPointFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .sierraEntryPoint))
    (classKeys .sierraEntryPoint) (classNames .sierraEntryPoint) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated SierraEntryPointsByType factory allocates the specified table. -/
theorem wp_makeSierraEntryPointsByTypeFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraEntryPointsByTypeFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .sierraEntryPointsByType) ∗
       names ↦* classNames .sierraEntryPointsByType ∗ order ↦* classKeys .sierraEntryPointsByType }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .sierraEntryPointsByType)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .sierraEntryPointsByType)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeSierraEntryPointsByTypeFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraEntryPointsByTypeFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .sierraEntryPointsByType) (classNames .sierraEntryPointsByType) (classMask .sierraEntryPointsByType) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeSierraEntryPointsByTypeFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .sierraEntryPointsByType))
    (classKeys .sierraEntryPointsByType) (classNames .sierraEntryPointsByType) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated CasmEntryPoint factory allocates the specified table. -/
theorem wp_makeCasmEntryPointFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeCasmEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .casmEntryPoint) ∗
       names ↦* classNames .casmEntryPoint ∗ order ↦* classKeys .casmEntryPoint }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .casmEntryPoint)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .casmEntryPoint)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeCasmEntryPointFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeCasmEntryPointFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .casmEntryPoint) (classNames .casmEntryPoint) (classMask .casmEntryPoint) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeCasmEntryPointFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .casmEntryPoint))
    (classKeys .casmEntryPoint) (classNames .casmEntryPoint) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated SegmentLengths factory allocates the specified table. -/
theorem wp_makeSegmentLengthsFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSegmentLengthsFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .segmentLengths) ∗
       names ↦* classNames .segmentLengths ∗ order ↦* classKeys .segmentLengths }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .segmentLengths)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .segmentLengths)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeSegmentLengthsFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSegmentLengthsFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .segmentLengths) (classNames .segmentLengths) (classMask .segmentLengths) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeSegmentLengthsFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .segmentLengths))
    (classKeys .segmentLengths) (classNames .segmentLengths) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated CasmClass factory allocates the specified table. -/
theorem wp_makeCasmClassFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeCasmClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .casmClass) ∗
       names ↦* classNames .casmClass ∗ order ↦* classKeys .casmClass }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .casmClass)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .casmClass)
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
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeCasmClassFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeCasmClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .casmClass) (classNames .casmClass) (classMask .casmClass) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeCasmClassFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .casmClass))
    (classKeys .casmClass) (classNames .casmClass) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated SierraClass factory allocates the specified table. -/
theorem wp_makeSierraClassFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (classMask .sierraClass) ∗
       names ↦* classNames .sierraClass ∗ order ↦* classKeys .sierraClass }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (classNames .sierraClass)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (classKeys .sierraClass)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (7 : Int) 0) 0 = 7 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 7) (W64 7)) (slice.mk order (W64 7) (W64 7))
    (by change 0 ≤ sint.Z (W64 7); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [classMask, classNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeSierraClassFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborclasses.cborMakeSierraClassFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (classKeys .sierraClass) (classNames .sierraClass) (classMask .sierraClass) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeSierraClassFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (classMask .sierraClass))
    (classKeys .sierraClass) (classNames .sierraClass) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeprecatedEntryPointFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeprecatedEntryPointFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeprecatedCairoClassFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeDeprecatedCairoClassFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraEntryPointFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraEntryPointFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraEntryPointsByTypeFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraEntryPointsByTypeFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeCasmEntryPointFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeCasmEntryPointFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSegmentLengthsFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSegmentLengthsFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeCasmClassFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeCasmClassFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraClassFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeSierraClassFields_shared
