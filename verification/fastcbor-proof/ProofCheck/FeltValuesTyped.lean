import ProofCheck.FeltValuesComplete

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

/-- Field publication is atomic at the helper boundary: successful decoding
    replaces the slice, while every failure preserves the original field. -/
def FeltValuesReadOwned (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : FeltValuesOutcome → IProp GF
  | .success values rest => iprop(⌜result = interface.nil⌝ ∗ ∃ out : GoSlice,
      dst ↦ out ∗ FeltValuesSuccess c v xs wire out values rest)
  | .failure offset reason => iprop(dst ↦ old ∗
      FeltValuesDecoded c v xs wire kinds slice.nil result (.failure offset reason))

/-- The actual typed field reader has a contract for every byte input and old
    slice value. Old backing ownership may be framed across all outcomes. -/
theorem wp_readFeltValues_complete (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltValues)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (outcome : FeltValuesOutcome) (kinds : FeltErrorKinds),
       FeltErrorGlobals kinds ∗ ⌜FeltValuesSpec d wire outcome⌝ ∗
       FeltValuesReadOwned c dst old v xs wire kinds result outcome }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply +noauto wp_limbSlice_complete c v xs wire dq d hd hdepth hp hbytes
    $$ [Hc Hs] as %out %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hc Hs
  cases outcome with
  | success values rest =>
    isimp only [FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨%Hresult, Hvalue⟩
    subst result
    wp_auto
    iapply HΦ
    iframe Hs
    iexists (FeltValuesOutcome.success values rest), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact Hspec
    isimp only [FeltValuesReadOwned]
    isplitr
    · ipureintro; trivial
    iexists out
    iframe Hdst Hvalue
  | failure offset reason =>
    isimp only [FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hc, %err, Herr, %Hresult⟩
    rcases Hresult with ⟨rfl, rfl, Hoffset⟩
    wp_auto
    iapply HΦ
    iframe Hs
    iexists (FeltValuesOutcome.failure offset reason), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact Hspec
    isimp only [FeltValuesReadOwned, FeltValuesDecoded]
    iframe Hdst Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨trivial, rfl, Hoffset⟩

/-- The actual typed field reader has a contract for every byte input and old
    slice value. Old backing ownership may be framed across all outcomes. -/
theorem wp_readPlainFeltValues_complete (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadPlainFeltValues)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (outcome : FeltValuesOutcome) (kinds : FeltErrorKinds),
       FeltErrorGlobals kinds ∗ ⌜FeltValuesSpec d wire outcome⌝ ∗
       FeltValuesReadOwned c dst old v xs wire kinds result outcome }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply +noauto wp_limbSlice_complete c v xs wire dq d hd hdepth hp hbytes
    $$ [Hc Hs] as %out %result ⟨Hs, %outcome, %kinds, Hglobals, %Hspec, Houtcome⟩
  · iframe Hc Hs
  cases outcome with
  | success values rest =>
    isimp only [FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨%Hresult, Hvalue⟩
    subst result
    wp_auto
    iapply HΦ
    iframe Hs
    iexists (FeltValuesOutcome.success values rest), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact Hspec
    isimp only [FeltValuesReadOwned]
    isplitr
    · ipureintro; trivial
    iexists out
    iframe Hdst Hvalue
  | failure offset reason =>
    isimp only [FeltValuesDecoded] at Houtcome
    icases Houtcome with ⟨Hc, %err, Herr, %Hresult⟩
    rcases Hresult with ⟨rfl, rfl, Hoffset⟩
    wp_auto
    iapply HΦ
    iframe Hs
    iexists (FeltValuesOutcome.failure offset reason), kinds
    iframe Hglobals
    isplitr
    · ipureintro; exact Hspec
    isimp only [FeltValuesReadOwned, FeltValuesDecoded]
    iframe Hdst Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨trivial, rfl, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltValues_complete
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readPlainFeltValues_complete
