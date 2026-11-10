import ProofCheck.FeltComplete
import ProofCheck.NullableFailure

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

/-- The scalar field policy is stateful: null and undefined preserve the old
    word; every unsigned CBOR width stores its decoded word. Mode uses the same
    policy on the pinned 64-bit target, including unnamed enum values. -/
inductive ScalarValid (old : w64) : List w8 → w64 → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : ScalarValid old [head] old
  | unsigned {head : w8} {payload : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0) :
      ScalarValid old (head :: payload) argument

def ScalarFailure (wire : List w8) (reason : ExpectedFailureReason) : Prop :=
  wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) ∧ ExpectedFailure (W8 0) wire reason

theorem scalar_classify (old : w64) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ ScalarValid old item value) ∨
    (∃ reason, ScalarFailure wire reason) := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact Or.inl ⟨[head], tail, old, rfl, .nullish (by simpa using nullish)⟩
  · have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
    have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
    rcases expected_classify (W8 0) wire with ⟨head, payload, body, value, split, enc, major⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨head :: payload, body, value, split, .unsigned enc major⟩
    · exact Or.inr ⟨reason, hn, hu, failure⟩

/-- Both actual scalar readers realize the independent successful relation. -/
theorem wp_readScalar_valid (mode : Bool) (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : w64) (dq : DFrac) (valid : ScalarValid old wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if mode then (@! cborReadMode) else (@! cborReadUint64))) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    cases mode
    · simp only [Bool.false_eq_true, ↓reduceIte]
      wp_apply wp_readUint64_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
      · iframe Hdst Hc Hs
      iapply HΦ
      iframe Hdst Hc Hs
      ipureintro
      exact Hnext
    · simp only [Bool.false_eq_true, ↓reduceIte]
      wp_apply wp_readMode_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
      · iframe Hdst Hc Hs
      iapply HΦ
      iframe Hdst Hc Hs
      ipureintro
      exact Hnext
  | @unsigned head payload argument enc major =>
    have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
      simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
    have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
      have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
      rw [hn]
      exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
    have hlength : (head :: payload).length = 1 + payload.length := by simp only [List.length_cons]; omega
    isimp only [hlength, Int.natCast_add, Int.natCast_one] at HΦ
    simp only [hlength, Int.natCast_add, Int.natCast_one] at Hnext
    cases mode
    · simp only [Bool.false_eq_true, ↓reduceIte]
      wp_apply wp_readUint64_encoded c dst old v xs payload rem head value dq enc major hp.1 hbound hi hafter
        $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
      · iframe Hdst Hc Hs
      iapply HΦ
      iframe Hdst Hc Hs
      ipureintro
      exact Hnext
    · simp only [Bool.false_eq_true, ↓reduceIte]
      wp_apply wp_readMode_encoded c dst old v xs payload rem head value dq enc major hp.1 hbound hi hafter
        $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
      · iframe Hdst Hc Hs
      iapply HΦ
      iframe Hdst Hc Hs
      ipureintro
      exact Hnext

/-- Every rejected scalar preserves the destination and cursor, with the exact
    diagnostic from the actual unsigned expectation. -/
theorem wp_readScalar_failure (mode : Bool) (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if mode then (@! cborReadMode) else (@! cborReadUint64))) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  cases mode <;> simp only [Bool.false_eq_true, ↓reduceIte]
  all_goals
    wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
    wp_auto
    wp_apply wp_null_not_nullish c v xs wire dq failure.1 failure.2.1 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_uint64_failure c v xs wire dq reason kind failure.2.2 hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

/-- Success returns the specified scalar and exact suffix; failure returns the
    unchanged destination/cursor and an owned diagnostic at the original offset. -/
def ScalarOutcome (c dst : Loc) (old : w64) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : w64), dst ↦ value ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ ScalarValid old item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (reason : ExpectedFailureReason) (err : Loc), dst ↦ old ∗ c ↦ v ∗
    ExpectedFailureError err v.pos' (W8 0) reason (expectedFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ ScalarFailure wire reason⌝))

theorem wp_readScalar_complete (mode : Bool) (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (if mode then (@! cborReadMode) else (@! cborReadUint64))) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ScalarOutcome c dst old v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hdst, Hc, Hs⟩ HΦ
  rcases scalar_classify old wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · wp_apply wp_readScalar_valid mode c dst old v xs item rest value dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold ScalarOutcome
    ileft
    iexists item, rest, value
    iframe Hdst Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) reason) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_readScalar_failure mode c dst old v xs wire dq reason (expectedFailureKind reason kinds)
      failure hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold ScalarOutcome
    iright
    iexists reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.scalar_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readScalar_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readScalar_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readScalar_complete
