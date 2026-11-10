import ProofCheck.Fields

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Freezing the allocated metadata retains its exact contents and mask. -/
theorem staticFields_share (fields : Loc) (f : cursor.Fields)
    (keys : List cursor.StaticField) (names : List GoString) (qf qo qn : DFrac) :
    (fields ↦{qf} f ∗ f.order' ↦*{qo} keys ∗ f.names' ↦*{qn} names : IProp GF) ⊢
      |==> StaticFieldsShared fields keys names f.all' := by
  iintro ⟨Hfields, Horder, Hnames⟩
  ipersist Hfields
  ipersist Horder
  ipersist Hnames
  imodintro
  isimp only [StaticFieldsShared]
  iexists f
  iframe Hfields Horder Hnames
  ipureintro; rfl

/-- The actual GasPrice factory establishes shareable metadata. It requires only
    the cursor dependency, so it is usable while core itself is initializing. -/
theorem wp_makeGasPriceFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborschema.cborMakeGasPriceFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields gasFieldKeys gasFieldNames (W64 3) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeGasPriceFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (W64 3))
    gasFieldKeys gasFieldNames (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- The actual Header factory likewise establishes immutable ownership without a
    circular assumption that core initialization has already completed. -/
theorem wp_makeHeaderFields_shared :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor }}
      (App (Val (@! internal.cborschema.cborMakeHeaderFields)) (Val #()))
    {{ (fields : Loc), RET #fields;
       StaticFieldsShared fields headerFieldKeys headerFieldNames (W64 32767) }} := by
  wp_start_folded
  iapply wp_fupd
  wp_apply wp_makeHeaderFields as %fields ⟨%names, %order, Hfields, Hnames, Horder⟩
  imod staticFields_share fields (cursor.Fields.mk order names (W64 32767))
    headerFieldKeys headerFieldNames (DFrac.own 1) (DFrac.own 1) (DFrac.own 1)
    $$ [Hfields Horder Hnames] with Hshared
  · iframe Hfields Horder Hnames
  imodintro
  iapply HΦ
  iframe Hshared

/-- After initialization has stored the factory results, the globals and tables
    can be retained as persistent read-only resources for all future calls. -/
theorem headerMetadata_publish (fields gasFields : Loc) :
    (globalAddr internal.cborschema.cborHeaderFields ↦ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦ gasFields ∗
     StaticFieldsShared fields headerFieldKeys headerFieldNames (W64 32767) ∗
     StaticFieldsShared gasFields gasFieldKeys gasFieldNames (W64 3) : IProp GF) ⊢
      |==> HeaderMetadata (GF := GF) := by
  unfold HeaderMetadata
  iintro ⟨Hglobal, HgasGlobal, #Hfields, #HgasFields⟩
  ipersist Hglobal
  ipersist HgasGlobal
  imodintro
  iexists fields, gasFields
  iframe Hglobal HgasGlobal Hfields HgasFields

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.staticFields_share
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeGasPriceFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_makeHeaderFields_shared
#print axioms Perennial.github_com.NethermindEth.juno.core.headerMetadata_publish
