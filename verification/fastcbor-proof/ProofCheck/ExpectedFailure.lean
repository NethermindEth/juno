import ProofCheck.FeltComplete

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

/-- Every failed unsigned expectation declines the mixed-width shortcut. -/
theorem expectedFailure_unsignedReject (wire : List w8) (reason : ExpectedFailureReason)
    (failure : ExpectedFailure (W8 0) wire reason) : UnsignedReject wire := by
  cases failure with
  | head tail fault => exact unsignedReject_fault _ tail _ fault
  | incomplete tail info sizeEq short => exact unsignedReject_truncated _ tail _ info sizeEq short
  | eof => trivial
  | typeMismatch tail enc major notTag => exact unsignedReject_nonuint _ _ major
  | tag tail enc major isTag => exact unsignedReject_nonuint _ _ major

/-- An array expectation failure cannot begin with the inline-four array head. -/
theorem expectedFailure_not_four (wire : List w8) (reason : ExpectedFailureReason)
    (failure : ExpectedFailure (W8 128) wire reason) : wire.head? ≠ some (W8 132) := by
  cases failure with
  | head tail fault =>
    have hi := headFault_info _ _ fault
    intro heq
    have heq' := Option.some.inj heq
    rw [heq'] at hi
    contradiction
  | incomplete tail info sizeEq short =>
    intro heq
    have heq' := Option.some.inj heq
    rw [heq'] at info
    contradiction
  | eof => simp
  | typeMismatch tail enc major notTag =>
    intro heq
    have heq' := Option.some.inj heq
    rw [heq'] at major
    contradiction
  | tag tail enc major isTag =>
    intro heq
    have heq' := Option.some.inj heq
    rw [heq'] at major
    contradiction

/-- The actual major-type expectation realizes every specified failure with
    unchanged cursor/input and owned diagnostic arguments. -/
theorem wp_expect_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (want : w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hw : want &&& W8 224 = want) (failure : ExpectedFailure want wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #want))
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
    wp_apply wp_expect_fault c v xs head want dq reason kind fault hw hp.1 hlt hi
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
    wp_apply wp_expect_truncated c v xs head want size dq kind hw hp.1 hlt hi info sizeEq hshort
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | eof =>
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [expectedFailureAddr] at Hkind
    isimp only [ExpectedFailureError] at HΦ
    wp_apply wp_expect_eof c v want kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
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
    wp_apply wp_expect_type c v xs payload tail head want argument dq kind enc hw major tag hp.1 hbound hi hafter
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
    wp_apply wp_expect_tag c v xs payload tail head want argument dq kind enc hw major tag hp.1 hbound hi hafter
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- Public Uint64 composes the complete unsigned-expectation rejection contract. -/
theorem wp_uint64_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 0) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_failure c v xs wire (W8 0) dq reason kind (by decide) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.expectedFailure_unsignedReject
#print axioms Perennial.github_com.NethermindEth.juno.core.expectedFailure_not_four
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_failure
