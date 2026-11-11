import ProofCheck.ProjectionTagSuccess
import ProofCheck.InitializationErrors
import ProofCheck.ClassTagProfile
import ProofCheck.ProjectionTagRead

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Null/undefined clears the interface before inspecting tags, consuming one byte. -/
theorem wp_class_nullish (c dst : Loc) (old : GoInterface) (tags : CBORClassTags)
    (v : cursor.Cursor) (xs rest : List w8) (head : w8) (dq : DFrac)
    (hn : head = W8 246 ∨ head = W8 247)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = head :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ RET #interface.nil; dst ↦ interface.nil ∗ c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 1) = sint.Z v.pos' + 1 ∧ sint.Z (v.pos' + W64 1) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs [head] rest v.pos' v.buf'.len hp Hlen.1 hbytes
  have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  wp_auto
  wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs
  ipureintro; simpa only [List.length_singleton, Int.natCast_one] using Hnext

/-- Malformed or absent required tags preserve the old interface and entire
    cursor, returning the initialized structured error before allocation. -/
theorem wp_class_tag_rejected (c dst : Loc) (old : GoInterface) (tags : CBORClassTags)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason)
    (hn : wire.head? ≠ some (W8 246)) (hu : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 192) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
       ExpectedFailureError err v.pos' (W8 192) reason (expectedFailureKind reason kinds) }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals; solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 192) reason) $$ Hglobals
  isimp only [feltFailureAddr, feltFailureKind] at Hkind
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_tag_failure c v xs wire dq reason (expectedFailureKind reason kinds) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  iapply HΦ
  iframe Hdst Hc Hs
  iexists kinds
  iframe Hglobals Herr

/-- An unknown tag is consumed in full, then the bare unsupported sentinel is
    returned without allocating a record or modifying the old interface. -/
theorem wp_class_unknown_tag (c dst : Loc) (old : GoInterface) (tags : CBORClassTags)
    (v : cursor.Cursor) (xs payload rest : List w8) (head : w8) (tag : w64) (dq : DFrac)
    (enc : HeadArgument head payload tag) (major : head &&& W8 224 = W8 192)
    (unknown : ClassTagMeaning tags tag none)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadClassDefinition)) (Val #c)) (Val #dst)) (Val #tags))
    {{ (result : GoInterface), RET #result;
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       (∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ ⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals; solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %HnonNil := projection_unsupported_nonNil kinds $$ [Hglobals]
  · iframe Hglobals; solve_pkg_init
  ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 192) .tag) $$ Hglobals
  isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
  have hn : ((head :: payload) ++ rest).head? ≠ some (W8 246) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have hu : ((head :: payload) ++ rest).head? ≠ some (W8 247) := by
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    intro eq; subst head; contradiction
  have HDeprecatedClass : tag ≠ tags.DeprecatedCairoClass' := unknown .deprecated
  have HSierraClass : tag ≠ tags.SierraClass' := unknown .sierra
  wp_auto
  wp_apply wp_null_not_nullish c v xs ((head :: payload) ++ rest) dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_tag_wire c v xs payload rest head tag dq enc major hp hbytes $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [HDeprecatedClass, _root_.decide_false]
  wp_auto
  simp only [HSierraClass, _root_.decide_false]
  wp_auto
  iapply HΦ
  iframe Hdst Hc Hs
  isplitr
  · iexists kinds
    iframe Hglobals
    ipureintro; exact ⟨rfl, HnonNil⟩
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_tag_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_class_unknown_tag
