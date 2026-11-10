import ProofCheck.FeltPointersFailure

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual major-type expectation realizes every specified failure with
    unchanged cursor/input and owned diagnostic arguments. -/
theorem wp_expectSlow_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (want : w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure want wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expectSlow")) (Val #want))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' want reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  cases failure with
  | @head head tail reason fault =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_cons, List.length_nil] at Hnext; omega
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    have hslow := wp_headSlow_fault (GF := GF) c v xs head dq reason kind fault hp.1 hlt hi
    wp_apply wp_expectSlow_head_reject c v xs want dq
      (iprop(headFaultAddr (GF := GF) reason ↦□ kind))
      (fun err => HeadFaultError err v.pos' head reason kind) hslow
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @incomplete head tail size info sizeEq short =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_cons, List.length_nil] at Hnext; omega
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    have hshort : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen short ⊢; omega
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_expectSlow_truncated c v xs head want size dq kind hp.1 hlt hi info sizeEq hshort
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | eof =>
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_method_call
    wp_call
    wp_auto
    wp_method_call
    wp_call
    wp_auto
    simp only [heof, Int.le_refl, _root_.decide_true]
    wp_auto
    wp_apply wp_fail c v kind (go!"input ends before the item") slice.nil $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hc
    isimp only [heof] at Herr
    wp_end
  | @typeMismatch head payload tail argument enc major tag =>
    have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_expectSlow_type c v xs payload tail head want argument dq kind enc major tag hp.1 hbound hi hafter
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @tag head payload tail argument enc major tag =>
    have hi := input_suffix_head xs (payload ++ tail) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs (head :: payload) tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ tail := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ tail) _ _ hbytes
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_expectSlow_tag c v xs payload tail head want argument dq kind enc major tag hp.1 hbound hi hafter
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- A failed expectation cannot take an inline successful-head shortcut. -/
theorem expectedFailure_inline_reject (want head : w8) (tail : List w8)
    (reason : ExpectedFailureReason) (hw : want &&& W8 224 = want)
    (failure : ExpectedFailure want (head :: tail) reason) :
    ¬ uint.Z (head - want) < uint.Z (W8 24) := by
  cases failure with
  | head tail fault =>
    have hi := headFault_info _ _ fault
    by_cases hm : head &&& W8 224 = want
    · rw [← hm, byte_major_remainder]
      change ¬ uint.Z (head &&& W8 31) < 24
      omega
    · exact byte_wrong_major_slow head want hw hm
  | incomplete tail info sizeEq short =>
    by_cases hm : head &&& W8 224 = want
    · rw [← hm, byte_major_remainder]
      change ¬ uint.Z (head &&& W8 31) < 24
      omega
    · exact byte_wrong_major_slow head want hw hm
  | typeMismatch tail enc major tag => exact byte_wrong_major_slow head want hw major
  | tag tail enc major tag => exact byte_wrong_major_slow head want hw major

/-- Container parsing propagates expectation failures before count/depth checks. -/
theorem wp_containerLen_expected_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (want : w8) (perItem : w64) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure want wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"containerLen")) (Val #want)) (Val #perItem))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' want reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expectSlow_failure c v xs wire want dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- ArrayLen rejects every malformed, incomplete, missing or wrong-type head,
    including through its inline-array shortcut, with the original cursor. -/
theorem wp_ArrayLen_expected_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 128) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"ArrayLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 128) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hslow := wp_containerLen_expected_failure (GF := GF) c v xs wire (W8 128) (W64 1) dq reason kind
    failure hp hbytes
  wp_auto
  cases wire with
  | nil =>
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    simp only [heof, Int.lt_irrefl, _root_.decide_false]
    wp_auto
    wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    isimp only [heof] at Herr
    wp_end
  | cons head tail =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_cons, List.length_nil] at Hnext; omega
    simp only [hlt, _root_.decide_true]
    wp_auto
    by_cases hd : sint.Z v.depth' < sint.Z (W64 32)
    · simp only [hd, _root_.decide_true]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hlt⟩)]
      wp_apply wp_load_slice_index v.buf' _ xs dq head hp.1 $$ [Hs] with Hs
      · iframe Hs; ipureintro; exact hi
      have hsmall := expectedFailure_inline_reject (W8 128) head tail reason (by decide) failure
      simp only [hsmall, _root_.decide_false]
      wp_auto
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    · simp only [hd, _root_.decide_false]
      wp_auto
      wp_apply hslow $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectSlow_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.expectedFailure_inline_reject
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_containerLen_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_ArrayLen_expected_failure
