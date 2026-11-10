import ProofCheck.StringText
import ProofCheck.HeaderValueFailureSpec

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

/-- The typed string reader realizes the full successful relation when supplied
    with a proof of the actual validator. Nullish fields preserve the old bytes. -/
theorem wp_readString_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire rem : List w8)
    (value : GoString) (dq : DFrac) (valid : StringValid old wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 bytes
  cases valid with
  | @nullish head nullish =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head bytes
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    wp_apply wp_readString_null_at c dst old v xs head dq hp.1 hi nullish
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs
    ipureintro; exact Hnext
  | @text head payload value argument enc major count valid =>
    have argumentEq : argument = W64 value.length := by word
    subst argument
    have length : ((head :: payload) ++ value).length = 1 + payload.length + value.length := by simp; omega
    isimp only [length, Int.natCast_add, Int.natCast_one] at HΦ
    have hi := input_suffix_head xs (payload ++ value ++ rem)
      (sint.nat v.pos') (sint.nat v.buf'.len) head bytes
    have hn : head ≠ W8 246 := by intro h; subst head; contradiction
    have hu : head ≠ W8 247 := by intro h; subst head; contradiction
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_string_text validator c v xs payload value rem head dq enc major valid hp bytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hadvance⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs
    ipureintro; exact ⟨by omega, Hadvance.2⟩

/-- General malformed UTF-8 reaches the typed reader's error return without
    publishing a new string or changing the input/cursor. -/
theorem wp_readString_invalid_utf8 (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs payload body rem : List w8)
    (head : w8) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload (W64 body.length)) (major : head &&& W8 224 = W8 96)
    (invalid : ¬ Utf8Valid body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"text string is not valid UTF-8") slice.nil }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  have hi := input_suffix_head xs (payload ++ body ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head bytes
  have hn : head ≠ W8 246 := by intro h; subst head; contradiction
  have hu : head ≠ W8 247 := by intro h; subst head; contradiction
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_string_invalid_utf8 validator c v xs payload body rem head dq kind enc major invalid hp bytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- All specified typed-string failures compose without restricting malformed
    UTF-8 to a particular byte position, conditional on the validator contract. -/
theorem wp_readString_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : StringFailureReason) (kind : GoInterface) (failure : StringRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       stringFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ StringFailureError err v.pos' reason kind }} := by
  cases failure with
  | expected hn hu failure =>
    simpa only [stringFailureAddr, StringFailureError] using
      wp_readString_expected_failure (GF := GF) c dst old v xs wire dq _ kind hn hu failure hp bytes
  | @truncated head payload tail argument enc major short =>
    simpa only [stringFailureAddr, StringFailureError] using
      wp_readString_truncated (GF := GF) c dst old v xs payload tail head major argument dq kind enc hp bytes short
  | @invalidUTF8 head payload body rem argument enc major count invalid =>
    have argumentEq : argument = W64 body.length := by word
    subst argument
    simpa only [stringFailureAddr, StringFailureError] using
      wp_readString_invalid_utf8 (GF := GF) validator c dst old v xs payload body rem head dq kind enc major invalid hp bytes

/-- Full independent typed-string outcomes, including old-value preservation on
    nullish input and every failure, exact successful suffixes and owned errors. -/
def StringOutcome (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : GoString), dst ↦ value ∗
    c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ StringValid old item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (reason : StringFailureReason) (err : Loc), dst ↦ old ∗ c ↦ v ∗
    StringFailureError err v.pos' reason (stringFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ StringRejected wire reason⌝))

/-- All-input typed-string composition. Its explicit validator premise must
    still be proved before this becomes a closed production contract. -/
theorem wp_readString_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoString) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (kinds : FeltErrorKinds)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (bytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ StringOutcome c dst old v xs wire kinds result }} := by
  iintro %Φ ⟨#Hpkg, Hglobals, Hdst, Hc, Hs⟩ HΦ
  rcases string_classify old wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
  · wp_apply wp_readString_valid validator c dst old v xs item rest value dq valid hp (bytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold StringOutcome
    ileft
    iexists item, rest, value
    iframe Hdst Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := stringErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readString_rejected validator c dst old v xs wire dq reason (stringFailureKind reason kinds)
      failure hp bytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold StringOutcome
    iright
    iexists reason, err
    iframe Hdst Hc Herr
    ipureintro; exact ⟨rfl, failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_invalid_utf8
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_complete
