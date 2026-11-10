import ProofCheck.ArrayExpectedFailure

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

/-- MapLen rejects every malformed, incomplete, missing or wrong-type head,
    including through its inline-map shortcut, with the original cursor. -/
theorem wp_MapLen_expected_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 160) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"MapLen")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 160) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hslow := wp_containerLen_expected_failure (GF := GF) c v xs wire (W8 160) (W64 2) dq reason kind
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
      have hsmall := expectedFailure_inline_reject (W8 160) head tail reason (by decide) failure
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

/-- Null and undefined take the success branch before map expectation. Every
    other failed expectation preserves the original cursor and closed state. -/
theorem wp_openStruct_expected_failure (c fields : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 160) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ (err : Loc), RET (PairV #({(zero_val cursor.Struct) with fields' := fields}) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 160) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_expected_failure c v xs wire dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- At any valid cursor offset and nesting depth, null/undefined opens an empty
    closed struct without entering a map. -/
theorem wp_openStruct_nullish (c fields : Loc) (v : cursor.Cursor) (xs : List w8) (head : w8)
    (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some head) (hn : head = W8 246 ∨ head = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"OpenStruct")) (Val #fields))
    {{ RET (PairV #({(zero_val cursor.Struct) with fields' := fields}) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs head dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_MapLen_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_openStruct_nullish
