import ProofCheck.FeltLimbReject

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

/-- The application distinguishes unsupported indefinite containers from
    malformed break/reserved heads. No argument bytes are consumed in any case. -/
inductive HeadError where
  | indefinite
  | breakCode
  | invalid

inductive HeadFault (head : w8) : HeadError → Prop where
  | indefinite (info : head &&& W8 31 = W8 31)
      (major : 64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160) :
      HeadFault head .indefinite
  | breakCode (info : head &&& W8 31 = W8 31) (major : head &&& W8 224 = W8 224) :
      HeadFault head .breakCode
  | invalid (info : 27 < uint.Z (head &&& W8 31))
      (indefinite : ¬ (head &&& W8 31 = W8 31 ∧
        64 ≤ uint.Z (head &&& W8 224) ∧ uint.Z (head &&& W8 224) ≤ 160))
      (breakCode : ¬ (head &&& W8 31 = W8 31 ∧ head &&& W8 224 = W8 224)) :
      HeadFault head .invalid

def headFaultAddr (reason : HeadError) : Loc :=
  match reason with
  | .indefinite => globalAddr cursor.ErrUnsupported
  | .breakCode | .invalid => globalAddr cursor.ErrSyntax

def HeadFaultError (err : Loc) (pos : w64) (head : w8) (reason : HeadError) (kind : GoInterface) : IProp GF :=
  match reason with
  | .indefinite => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"indefinite-length item") slice.nil)
  | .breakCode => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"break code outside an indefinite-length item") slice.nil)
  | .invalid => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kind pos (go!"invalid additional information %d for %s") args ∗
      args ↦* [interface.mkOk go.byte #(head &&& W8 31), interface.mkOk go.string #(headDescription head)])

theorem headFault_info (head : w8) (reason : HeadError) (fault : HeadFault head reason) :
    27 < uint.Z (head &&& W8 31) := by
  cases fault with
  | indefinite hi _ => rw [hi]; decide
  | breakCode hi _ => rw [hi]; decide
  | invalid hi _ _ => exact hi

/-- Closed head classification, including owned variadic diagnostic arguments. -/
theorem wp_headSlow_fault (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  have hinfo := headFault_info head reason fault
  cases fault with
  | indefinite hindef hmajor =>
    exact wp_headSlow_indefinite c v xs head dq kind hp hlt hi hinfo hindef hmajor
  | breakCode hindef hmajor =>
    exact wp_headSlow_break c v xs head dq kind hp hlt hi hinfo hindef hmajor
  | invalid hinfo hindef hbreak =>
    exact wp_headSlow_invalid c v xs head dq kind hp hlt hi hinfo hindef hbreak

/-- Expectation propagates a proved head error before checking the requested type. -/
theorem wp_expectSlow_head_reject (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (want : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  wp_auto
  wp_apply hslow $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- The inline expectation guard cannot hide a proved extended-head failure. -/
theorem wp_expect_head_reject (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head want : w8) (dq : DFrac) (R : IProp GF) (E : Loc → IProp GF)
    (hw : want &&& W8 224 = want)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head)
    (hinfo : 24 ≤ uint.Z (head &&& W8 31))
    (hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"headSlow")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #(W8 0) #(W64 0)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  have hsmall : ¬ uint.Z (head - want) < uint.Z (W8 24) := by
    by_cases hm : head &&& W8 224 = want
    · rw [← hm, byte_major_remainder]
      change ¬ uint.Z (head &&& W8 31) < 24
      omega
    · exact byte_wrong_major_slow head want hw hm
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq head hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hsmall, _root_.decide_false]
  wp_auto
  wp_apply wp_expectSlow_head_reject c v xs want dq R E hslow $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe HR Hc Hs
  wp_end

/-- Every expected major type preserves malformed-head classification and offset. -/
theorem wp_expect_fault (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head want : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason) (hw : want &&& W8 224 = want)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  have hinfo := headFault_info head reason fault
  exact wp_expect_head_reject c v xs head want dq iprop(headFaultAddr (GF := GF) reason ↦□ kind)
    (fun err => HeadFaultError err v.pos' head reason kind) hw hp hlt hi (by omega)
    (wp_headSlow_fault c v xs head dq reason kind fault hp hlt hi)

/-- Public Uint64 propagates all malformed-head cases without consuming input. -/
theorem wp_uint64_fault (c : Loc) (v : cursor.Cursor) (xs : List w8)
    (head : w8) (dq : DFrac) (reason : HeadError) (kind : GoInterface)
    (fault : HeadFault head reason)
    (hp : 0 ≤ sint.Z v.pos') (hlt : sint.Z v.pos' < sint.Z v.buf'.len)
    (hi : xs[(sint.Z v.pos').toNat]? = some head) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       headFaultAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ HeadFaultError err v.pos' head reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_fault c v xs head (W8 0) dq reason kind fault (by decide) hp hlt hi
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headFault_info
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headSlow_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_head_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_fault
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_fault
