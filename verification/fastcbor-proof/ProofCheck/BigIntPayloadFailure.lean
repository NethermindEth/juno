import ProofCheck.BigIntErrorIs
import ProofCheck.StringFailure

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- An expected-payload diagnostic retains its object and argument ownership
    when errors.Is inspects its category. -/
theorem wp_expectedFailure_is_category (err kind target : Loc) (pos : w64)
    (want : w8) (reason : ExpectedFailureReason) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       ExpectedFailureError err pos want reason (interface.mkOk (go.PointerType errors.errorString.ty) #kind) }}
      (App (App (Val (@! errors.Is))
        (Val #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)))
        (Val #(interface.mkOk (go.PointerType errors.errorString.ty) #target)))
    {{ RET #(decide (kind = target));
       ExpectedFailureError err pos want reason (interface.mkOk (go.PointerType errors.errorString.ty) #kind) }} := by
  iintro %Φ ⟨#Hpkg, Herr⟩ HΦ
  cases reason with
  | head bad reason =>
    cases reason <;> isimp only [ExpectedFailureError, HeadFaultError] at Herr HΦ
    all_goals
      first
      | icases Herr with ⟨%args, Herr, Hargs⟩
        wp_apply wp_cursorError_is_category err kind target pos _ args $$ [Herr] with Herr
        · iframe Herr
        iapply HΦ; iexists args; iframe
      | wp_apply wp_cursorError_is_category err kind target pos _ slice.nil $$ [Herr] with Herr
        · iframe Herr
        iapply HΦ; iframe
  | incomplete | eof =>
    isimp only [ExpectedFailureError] at Herr HΦ
    wp_apply wp_cursorError_is_category err kind target pos _ slice.nil $$ [Herr] with Herr
    · iframe Herr
    iapply HΦ; iframe
  | typeMismatch bad | tag =>
    isimp only [ExpectedFailureError] at Herr HΦ
    icases Herr with ⟨%args, Herr, Hargs⟩
    wp_apply wp_cursorError_is_category err kind target pos _ args $$ [Herr] with Herr
    · iframe Herr
    iapply HΦ; iexists args; iframe

/-- Failed expectations inside an allowed bignum tag restore the outer cursor.
    Type errors are rewritten as syntax errors at the outer start; all other
    categories retain their inner diagnostic offset and owned arguments. -/
theorem wp_bigInt_payload_expected (c : Loc) (v : cursor.Cursor) (xs tagBytes tail : List w8)
    (tagHead : w8) (tag : w64) (dq : DFrac) (reason : ExpectedFailureReason)
    (kind target : Loc) (syntaxKind : GoInterface)
    (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
    (allowed : tag = W64 2 ∨ tag = W64 3) (failure : ExpectedFailure (W8 64) tail reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (tagHead :: tagBytes) ++ tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ (interface.mkOk (go.PointerType errors.errorString.ty) #kind) ∗
       globalAddr cursor.ErrType ↦□ (interface.mkOk (go.PointerType errors.errorString.ty) #target) ∗
       globalAddr cursor.ErrSyntax ↦□ syntaxKind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       if kind = target then
         err ↦ cursor.DecodeError.mk syntaxKind v.pos' (go!"tag 2 or 3 must hold a byte string") slice.nil
       else ExpectedFailureError err (v.pos' + W64 (1 + tagBytes.length)) (W8 64) reason
         (interface.mkOk (go.PointerType errors.errorString.ty) #kind) }} := by
  wp_start as ⟨#Hkind, #Htype, #Hsyntax, Hc, Hs⟩
  wp_auto
  wp_apply wp_head_prefix c v xs tagBytes tail tagHead tag dq tagEnc hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [tagMajor]
  wp_auto
  wp_join (iprop(c ↦ {v with pos' := v.pos' + W64 (1 + tagBytes.length)} ∗
      v.buf' ↦*{dq} xs ∗ argument_ptr ↦ tag) : IProp GF)
      at (if: _ then _ else _) with [Hc Hs argument] as ⟨Hc, Hs, argument⟩
  · rcases allowed with rfl | rfl <;> wp_auto <;> wp_join_done
  wp_apply wp_payload_expected_failure c {v with pos' := v.pos' + W64 (1 + tagBytes.length)}
    xs tail (W8 64) dq reason (interface.mkOk (go.PointerType errors.errorString.ty) #kind)
    (by decide) failure (by dsimp only; constructor; rw [Hnext.1]; omega; exact Hnext.2.1)
    Hnext.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_apply wp_expectedFailure_is_category err kind target (v.pos' + W64 (1 + tagBytes.length))
    (W8 64) reason $$ [Herr] with Herr
  · iframe Herr
  by_cases same : kind = target
  · simp only [same, _root_.decide_true]
    wp_auto
    wp_apply wp_fail c v syntaxKind (go!"tag 2 or 3 must hold a byte string") slice.nil
      $$ [Hc] as %newerr ⟨Hc, Hnew⟩
    · iframe Hc
    isimp only [same, ite_true] at HΦ
    wp_end
  · simp only [same, _root_.decide_false]
    isimp only [same, ite_false] at HΦ
    wp_auto
    wp_end

/-- A truncated tagged magnitude retains the payload's diagnostic offset while
    restoring the public decoder's original position. -/
theorem wp_bigInt_payload_truncated (c : Loc) (v : cursor.Cursor) (xs tagBytes lengthBytes tail : List w8)
    (tagHead bodyHead : w8) (tag count : w64) (dq : DFrac) (kind target : Loc)
    (different : kind ≠ target)
    (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
    (allowed : tag = W64 2 ∨ tag = W64 3)
    (bodyEnc : HeadArgument bodyHead lengthBytes count) (bodyMajor : bodyHead &&& W8 224 = W8 64)
    (short : tail.length < uint.Z count)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ tail) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrTruncated ↦□ (interface.mkOk (go.PointerType errors.errorString.ty) #kind) ∗
       globalAddr cursor.ErrType ↦□ (interface.mkOk (go.PointerType errors.errorString.ty) #target) ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk
         (interface.mkOk (go.PointerType errors.errorString.ty) #kind)
         (v.pos' + W64 (1 + tagBytes.length)) (go!"byte or text string longer than the input") slice.nil }} := by
  wp_start as ⟨#Hkind, #Htype, Hc, Hs⟩
  wp_auto
  wp_apply wp_head_prefix c v xs tagBytes ((bodyHead :: lengthBytes) ++ tail) tagHead tag dq tagEnc hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [tagMajor]
  wp_auto
  wp_join (iprop(c ↦ {v with pos' := v.pos' + W64 (1 + tagBytes.length)} ∗
      v.buf' ↦*{dq} xs ∗ argument_ptr ↦ tag) : IProp GF)
      at (if: _ then _ else _) with [Hc Hs argument] as ⟨Hc, Hs, argument⟩
  · rcases allowed with rfl | rfl <;> wp_auto <;> wp_join_done
  have hpay := wp_payload_truncated (GF := GF) c {v with pos' := v.pos' + W64 (1 + tagBytes.length)}
    xs lengthBytes tail bodyHead count dq (interface.mkOk (go.PointerType errors.errorString.ty) #kind)
    bodyEnc (by dsimp only; constructor; rw [Hnext.1]; omega; exact Hnext.2.1) Hnext.2.2 short
  simp only [bodyMajor] at hpay
  wp_apply hpay $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_apply wp_cursorError_is_category err kind target (v.pos' + W64 (1 + tagBytes.length))
    (go!"byte or text string longer than the input") slice.nil $$ [Herr] with Herr
  · iframe Herr
  simp only [different, _root_.decide_false]
  wp_auto
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expectedFailure_is_category
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_payload_expected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_payload_truncated
