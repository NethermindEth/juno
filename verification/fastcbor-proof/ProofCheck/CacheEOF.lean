import ProofCheck.CacheFast

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

/-- Empty input cannot match a cache entry, regardless of the table's current search index. -/
theorem wp_find_empty (st fields : Loc) (s : cursor.Struct) (f : cursor.Fields) (buf : GoSlice) (qf : DFrac)
    (hfields : s.fields' = fields) (hempty : buf.len = W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       st ↦ s ∗ fields ↦{qf} f }}
      (App (Val (st @!! go.PointerType cursor.Struct.ty @!! go!"find")) (Val #buf))
    {{ RET #(W64 (-1)); st ↦ s ∗ fields ↦{qf} f }} := by
  wp_start as ⟨Hst, Hfields⟩
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  simp only [hempty]
  wp_auto
  wp_for
  by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
  all_goals
    simp only [hnext, _root_.decide_true, _root_.decide_false, ite_true, ite_false]
    wp_auto
    try simp only [hempty, Int.lt_irrefl, _root_.decide_false]
    try wp_auto
    have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
    simp only [hf, Bool.false_eq_true, ite_false, _root_.decide_true, _root_.decide_false, ite_true]
    wp_auto
    ihave Hfields := Hrestore $$ Hfo
    wp_end

/-- EOF decrements the outstanding pair count but neither consumes input nor closes the map. -/
theorem wp_nextField_slow_eof (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (kind : GoInterface) (qf : DFrac) (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (heof : v.pos' = v.buf'.len) (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields⟩
  wp_auto
  have hn : s.left' ≠ W64 0 := by word
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true (by rw [heof]; exact ⟨hwf.1, Int.le_refl _, hwf.2⟩))]
  wp_auto
  wp_apply wp_find_empty st fields {s with left' := s.left' - W64 1} f
    (slice.slice v.buf' w8 v.pos' v.buf'.len) qf hfields
    (by simp only [slice.slice, heof, BitVec.sub_self]; rfl) $$ [Hst Hfields] with ⟨Hst, Hfields⟩
  · iframe Hst Hfields
  wp_apply wp_otherField_eof c st v kind heof hwf $$ [Hc] as %err ⟨Hc, Herr⟩
  · iframe Hc Hkind
  wp_end

/-- Public NextField handles EOF at every nonnegative cursor position, including the signed limit.
    No backing-input ownership is needed because this path performs no input byte load. -/
theorem wp_nextField_eof (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (kind : GoInterface) (qf : DFrac) (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (heof : v.pos' = v.buf'.len) (hwf : 0 ≤ sint.Z v.buf'.len ∧ sint.Z v.buf'.len ≤ sint.Z v.buf'.cap) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"input ends before the item") slice.nil }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields⟩
  wp_auto
  have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
  simp only [hleft', _root_.decide_true]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
  all_goals
    simp only [hnext, _root_.decide_true, _root_.decide_false]
    wp_auto
    try simp only [heof, BitVec.sub_self]
    try wp_auto
    ihave Hfields := Hrestore $$ Hfo
    wp_apply wp_nextField_slow_eof c st fields v s f kind qf hfields hleft heof hwf
      $$ [Hc Hst Hfields] as %err ⟨Hc, Hst, Hfields, Herr⟩
    · iframe Hc Hst Hfields Hkind
    isimp only [hfields] at Hst
    isimp only [heof] at Herr
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_find_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_slow_eof
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_eof
