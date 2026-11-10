import ProofCheck.NullableFailure
import ProofCheck.Payload

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

/-- A failed payload expectation returns before borrowing bytes or validating
    UTF-8, preserving the cursor and the exact owned expectation diagnostic. -/
theorem wp_payload_expected_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (want : w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (hw : want &&& W8 224 = want) (failure : ExpectedFailure want wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"payload")) (Val #want))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' want reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_failure c v xs wire want dq reason kind hw failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

theorem wp_string_expected_failure (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ExpectedFailure (W8 96) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"String")) (Val #()))
    {{ (err : Loc), RET (PairV #(go!"") #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 96) reason kind }} := by
  wp_start as ⟨#Hkind, Hc, Hs⟩
  wp_auto
  wp_apply wp_payload_expected_failure c v xs wire (W8 96) dq reason kind (by decide) failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Null and undefined succeed before string expectation in the typed wrapper. -/
theorem wp_readString_expected_failure (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (failure : ExpectedFailure (W8 96) wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 96) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq notNull notUndefined hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_string_expected_failure c v xs wire dq reason kind failure hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Only failures before UTF-8 validation. Invalid UTF-8 and successful strings
    require the separate validator contract; no exhaustive classification is claimed. -/
inductive StringFailureReason where
  | expected (reason : ExpectedFailureReason)
  | truncated

inductive StringFailure : List w8 → StringFailureReason → Prop where
  | expected {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 96) wire reason) : StringFailure wire (.expected reason)
  | truncated {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 96)
      (short : tail.length < uint.Z argument) : StringFailure ((head :: payload) ++ tail) .truncated

def stringFailureAddr (reason : StringFailureReason) : Loc :=
  match reason with
  | .expected reason => expectedFailureAddr (GF := GF) reason
  | .truncated => globalAddr cursor.ErrTruncated

def StringFailureError (err : Loc) (pos : w64) (reason : StringFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .expected reason => ExpectedFailureError err pos (W8 96) reason kind
  | .truncated => iprop(err ↦ cursor.DecodeError.mk kind pos (go!"byte or text string longer than the input") slice.nil)

/-- Every specified pre-validation string error is realized by the actual
    typed reader with unchanged destination, cursor and input. -/
theorem wp_readString_failure (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : StringFailureReason) (kind : GoInterface) (failure : StringFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       stringFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StringFailureError err v.pos' reason kind }} := by
  cases failure with
  | expected hn hu failure =>
    simpa only [stringFailureAddr, StringFailureError] using
      wp_readString_expected_failure (GF := GF) c dst old v xs wire dq _ kind hn hu failure hp hbytes
  | @truncated head payload tail argument enc major short =>
    simpa only [stringFailureAddr, StringFailureError] using
      wp_readString_truncated (GF := GF) c dst old v xs payload tail head major argument dq kind enc hp hbytes short

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_payload_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_string_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_expected_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_failure
