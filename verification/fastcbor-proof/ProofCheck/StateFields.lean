import ProofCheck.StateMetadata
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

/-- The actual generated StateDiff factory allocates the specified table. -/
theorem wp_makeStateDiffFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborstate.cborMakeStateDiffFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (stateMask .stateDiff) ∗
       names ↦* stateNames .stateDiff ∗ order ↦* stateKeys .stateDiff }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (stateNames .stateDiff)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (stateKeys .stateDiff)
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
  isimp only [stateMask, stateNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeStateDiffFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborstate.cborMakeStateDiffFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (stateKeys .stateDiff) (stateNames .stateDiff) (stateMask .stateDiff) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeStateDiffFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (stateMask .stateDiff))
    (stateKeys .stateDiff) (stateNames .stateDiff) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual generated StateUpdate factory allocates the specified table. -/
theorem wp_makeStateUpdateFields :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborstate.cborMakeStateUpdateFields)) (Val #()))
    {{ (fields : Loc), RET #fields; ∃ (names order : GoSlice),
       fields ↦ cursor.Fields.mk order names (stateMask .stateUpdate) ∗
       names ↦* stateNames .stateUpdate ∗ order ↦* stateKeys .stateUpdate }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_literal (V := GoString) (stateNames .stateUpdate)
  isplitr
  · ipureintro; rfl
  iintro %names ⟨Hnames, -⟩
  wp_auto
  wp_apply wp_slice_literal (V := cursor.StaticField) (stateKeys .stateUpdate)
  isplitr
  · ipureintro; rfl
  iintro %order ⟨Horder, -⟩
  wp_auto
  simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd, show max (max (4 : Int) 0) 0 = 4 from by decide]
  wp_apply wp_staticFields (slice.mk names (W64 4) (W64 4)) (slice.mk order (W64 4) (W64 4))
    (by change 0 ≤ sint.Z (W64 4); decide) as %fields Hfields
  iapply HΦ
  iexists _, _
  iframe Hnames Horder
  isimp only [stateMask, stateNames, List.length_cons, List.length_nil, Nat.reduceAdd]
  iframe Hfields

theorem wp_makeStateUpdateFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborstate.cborMakeStateUpdateFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields (stateKeys .stateUpdate) (stateNames .stateUpdate) (stateMask .stateUpdate) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeStateUpdateFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (stateMask .stateUpdate))
    (stateKeys .stateUpdate) (stateNames .stateUpdate) (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeStateDiffFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeStateDiffFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeStateUpdateFields
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeStateUpdateFields_shared
