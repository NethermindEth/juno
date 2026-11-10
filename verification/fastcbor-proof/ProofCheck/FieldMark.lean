import ProofCheck.Payload

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A previously unseen field sets exactly its bit and preserves all other struct progress. -/
theorem wp_mark_new (c st : Loc) (s : cursor.Struct) (field : w64)
    (hfield : 0 ≤ sint.Z field ∧ sint.Z field < 64)
    (hnew : s.seen' &&& (W64 1 <<< field) = W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ st ↦ s }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"mark")) (Val #st)) (Val #field))
    {{ RET (PairV #field #interface.nil); st ↦ {s with seen' := s.seen' ||| (W64 1 <<< field)} }} := by
  wp_start as Hst
  wp_auto
  wp_apply wp_shiftCount_int
  · ipureintro; exact hfield.1
  simp only [hnew]
  wp_auto
  wp_end

/-- Duplicate rejection preserves cursor and struct progress and owns the diagnostic's field name.
Read-only fractional metadata suffices, so separate decoder calls may share the names. -/
theorem wp_mark_duplicate (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (name : GoString) (field : w64) (kind : GoInterface) (qf qn : DFrac)
    (hfields : s.fields' = fields)
    (hfield : 0 ≤ sint.Z field ∧ sint.Z field < 64)
    (hlookup : names[(sint.Z field).toNat]? = some name)
    (hseen : s.seen' &&& (W64 1 <<< field) ≠ W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"mark")) (Val #st)) (Val #field))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ ∃ args : GoSlice,
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"field %s named twice") args ∗
       args ↦* [interface.mkOk go.string #name] }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Hnames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hnames
  have hindex : sint.Z field < sint.Z f.names'.len := by
    have h := (List.getElem?_eq_some_iff.mp hlookup).choose
    simp only [sint.nat, sint.Z] at Hlen h hfield ⊢
    omega
  wp_auto
  wp_apply wp_shiftCount_int
  · ipureintro; exact hfield.1
  simp only [hseen, _root_.decide_false]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_names fields f qf).access_strict $$ Hfields with ⟨Hfn, Hrestore⟩
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hfield.1, hindex⟩)]
  wp_apply wp_load_slice_index f.names' (sint.Z field) names qn name hfield.1 $$ [Hnames] with Hnames
  · iframe Hnames; ipureintro; exact hlookup
  ihave Hfields := Hrestore $$ Hfn
  wp_apply wp_slice_literal (V := GoInterface) [interface.mkOk go.string #name]
  isplitr
  · ipureintro; rfl
  iintro %args ⟨Hargs, -⟩
  wp_auto
  wp_apply wp_fail c v kind (go!"field %s named twice") _ $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_mark_new
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_mark_duplicate
