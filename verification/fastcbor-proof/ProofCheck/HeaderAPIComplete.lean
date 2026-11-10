import ProofCheck.HeaderOwnedZero
import ProofCheck.HeaderAPIOutcome

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

/-- Actual First decoding for every owned initial Header and every byte input.
    Nonzero destinations are preserved and rejected before reading input; zero
    destinations have the complete record contract. UTF-8 validation remains an
    explicit, unresolved premise, not an assumed axiom or completed milestone. -/
theorem wp_decodeHeaderFirst_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : HeaderValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       HeaderOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeaderFirst)) (Val #input)) (Val #dst))
    {{ (remaining : GoSlice) (result : GoInterface), RET (PairV #remaining #result);
       input ↦*{dq} wire ∗ ∃ outcome : HeaderFirstResult,
       ⌜HeaderFirstSpec old wire outcome⌝ ∗ HeaderFirstResultOwned dst input remaining wire result outcome }} := by
  classical
  iintro %Φ ⟨#Hpkg, #HentryPkg, Howned, Hs⟩ HΦ
  isimp only [HeaderOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hzero := headerChildren_zero_iff h old $$ Hchildren
  by_cases zero : old = headerZeroValue
  · subst old
    have hz := Hzero.mpr rfl
    subst h
    wp_apply wp_decodeHeaderFirst_zero_complete validator dst input wire dq $$ [Hdst Hs]
      as %remaining %result ⟨Hs, Hout⟩
    · iframe Hdst Hs
    ihave Hout := headerFirstZeroOutcome_spec dst input remaining wire result $$ Hout
    iapply HΦ
    iframe Hs Hout
  · have hne : h ≠ zero_val Header := fun hz => zero (Hzero.mp hz)
    wp_apply wp_decodeHeaderFirst_nonzero_initialized dst h input hne $$ [Hdst]
      as %err ⟨%kind, Hkind, Hdst, Herr⟩
    · iframe Hdst
    iapply HΦ
    iframe Hs
    iexists HeaderFirstResult.failure old .nonzero
    isplitl []
    · ipureintro; exact .nonzero old wire zero
    isimp only [HeaderFirstResultOwned, HeaderAPIErrorOwned]
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists h
      iframe Hdst Hchildren
    isplitl [Hkind Herr]
    · iexists kind, err
      iframe Hkind Herr
      ipureintro; rfl
    ipureintro; trivial

/-- Actual full decoding for every owned initial Header and byte input, with
    independent specified results, exact owned diagnostics and unchanged input.
    This is conditional partial correctness until the validator premise is proved. -/
theorem wp_decodeHeader_complete (validator : Utf8ValidatorContract (GF := GF))
    (dst : Loc) (old : HeaderValue) (input : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cborerr ∗
       HeaderOwned dst old ∗ input ↦*{dq} wire }}
      (App (App (Val (@! DecodeHeader)) (Val #input)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       input ↦*{dq} wire ∗ ∃ outcome : HeaderDecodeResult,
       ⌜HeaderFullSpec old wire outcome⌝ ∗ HeaderFullResultOwned dst result outcome }} := by
  classical
  iintro %Φ ⟨#Hpkg, #HentryPkg, Howned, Hs⟩ HΦ
  isimp only [HeaderOwned] at Howned
  icases Howned with ⟨%h, Hdst, Hchildren⟩
  ihave %Hzero := headerChildren_zero_iff h old $$ Hchildren
  by_cases zero : old = headerZeroValue
  · subst old
    have hz := Hzero.mpr rfl
    subst h
    wp_apply wp_decodeHeader_zero_complete validator dst input wire dq $$ [Hdst Hs]
      as %result ⟨Hs, Hout⟩
    · iframe Hdst Hs
    ihave Hout := headerFullZeroOutcome_spec dst wire result $$ Hout
    iapply HΦ
    iframe Hs Hout
  · have hne : h ≠ zero_val Header := fun hz => zero (Hzero.mp hz)
    wp_apply wp_decodeHeader_nonzero_initialized dst h input hne $$ [Hdst]
      as %err ⟨%kind, Hkind, Hdst, Herr⟩
    · iframe Hdst
    iapply HΦ
    iframe Hs
    iexists HeaderDecodeResult.failure old .nonzero
    isplitl []
    · ipureintro; exact .firstError (.nonzero old wire zero)
    isimp only [HeaderFullResultOwned, HeaderAPIErrorOwned]
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists h
      iframe Hdst Hchildren
    iexists kind, err
    iframe Hkind Herr
    ipureintro; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeaderFirst_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_decodeHeader_complete
