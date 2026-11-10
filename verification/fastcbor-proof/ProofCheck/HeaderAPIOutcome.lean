import ProofCheck.HeaderZeroComplete

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

/-- Concrete owned public errors, tied to initialized canonical sentinels and
    the independent error reason. Parse offsets are represented without wrap. -/
def HeaderAPIErrorOwned (result : GoInterface) (error : HeaderAPIError) : IProp GF :=
  match error with
  | .nonzero => iprop(∃ (kind : GoInterface) (err : Loc), globalAddr cborerr.ErrUnsupported ↦□ kind ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := kind, Detail' := go!"the destination is not the zero value"} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)
  | .decode offset reason => iprop(∃ (kinds : FeltErrorKinds) (err : Loc), FeltErrorGlobals kinds ∗
      HeaderRecordFailureError err (W64 offset) reason (headerRecordFailureKind reason kinds) ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧ sint.Z (W64 offset) = offset⌝)
  | .trailing extra => iprop(∃ (kind : GoInterface) (err : Loc), globalAddr cborerr.ErrExtraneousData ↦□ kind ∗
      err ↦ {(zero_val cborerr.DecodeError) with Kind' := kind, Extra' := W64 extra} ∗
      ⌜result = interface.mkOk (go.PointerType cborerr.DecodeError.ty) #err⌝)

/-- Representation of an independent First result in the actual output heap
    and return values. Successful suffixes alias exactly the original input. -/
def HeaderFirstResultOwned (dst : Loc) (input remaining : GoSlice) (wire : List w8)
    (result : GoInterface) (outcome : HeaderFirstResult) : IProp GF :=
  match outcome with
  | .success final rest => iprop(HeaderOwned dst final ∗ ∃ item : List w8,
      ⌜result = interface.nil ∧ wire = item ++ rest ∧ remaining = slice.slice input w8 (W64 item.length) input.len ∧
        sint.Z remaining.len = rest.length ∧ subslice (sint.nat (W64 item.length)) (sint.nat input.len) wire = rest⌝)
  | .failure final error => iprop(HeaderOwned dst final ∗ HeaderAPIErrorOwned result error ∗ ⌜remaining = slice.nil⌝)

/-- Representation of an independent full-decode result, with ownership of
    the resulting Header and, on failure, the exact concrete diagnostic. -/
def HeaderFullResultOwned (dst : Loc) (result : GoInterface) (outcome : HeaderDecodeResult) : IProp GF :=
  match outcome with
  | .success final => iprop(HeaderOwned dst final ∗ ⌜result = interface.nil⌝)
  | .failure final error => iprop(HeaderOwned dst final ∗ HeaderAPIErrorOwned result error)

/-- The checked zero-destination First outcome has the common public-result
    representation used by both the decode branch and the destination guard. -/
theorem headerFirstZeroOutcome_spec (dst : Loc) (input remaining : GoSlice) (wire : List w8) (result : GoInterface) :
    HeaderFirstZeroOutcome (GF := GF) dst input remaining wire result ⊢
      ∃ outcome : HeaderFirstResult, ⌜HeaderFirstSpec headerZeroValue wire outcome⌝ ∗
        HeaderFirstResultOwned dst input remaining wire result outcome := by
  unfold HeaderFirstZeroOutcome
  iintro ⟨%kinds, Hglobals, Hout⟩
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%item, %rest, %final, Hdst, %Hresult⟩
    rcases Hresult with ⟨hr, hs, split, spec, length, suffix⟩
    iexists HeaderFirstResult.success final rest
    isplitl []
    · ipureintro; exact spec
    isimp only [HeaderFirstResultOwned]
    iframe Hdst
    iexists item
    ipureintro; exact ⟨hr, split, hs, length, suffix⟩
  · icases Hfailure with ⟨%offset, %reason, %err, Hdst, Herr, %Hresult⟩
    rcases Hresult with ⟨hr, hs, spec, offsetFit, _⟩
    iexists HeaderFirstResult.failure headerZeroValue (.decode offset reason)
    isplitl []
    · ipureintro; exact spec
    isimp only [HeaderFirstResultOwned]
    iframe Hdst
    isplitl [Hglobals Herr]
    · isimp only [HeaderAPIErrorOwned]
      iexists kinds, err
      iframe Hglobals Herr
      ipureintro; exact ⟨hr, offsetFit⟩
    ipureintro; exact hs

/-- The checked zero-destination full outcome has the common public-result
    representation, preserving parser versus trailing-data error distinctions. -/
theorem headerFullZeroOutcome_spec (dst : Loc) (wire : List w8) (result : GoInterface) :
    HeaderFullZeroOutcome (GF := GF) dst wire result ⊢
      ∃ outcome : HeaderDecodeResult, ⌜HeaderFullSpec headerZeroValue wire outcome⌝ ∗
        HeaderFullResultOwned dst result outcome := by
  unfold HeaderFullZeroOutcome
  iintro ⟨%kinds, %extraKind, Hglobals, Hextra, Hout⟩
  icases Hout with (Hsuccess | Hfailure)
  · icases Hsuccess with ⟨%final, Hdst, %Hresult⟩
    iexists HeaderDecodeResult.success final
    isplitl []
    · ipureintro; exact Hresult.2
    isimp only [HeaderFullResultOwned]
    iframe Hdst
    ipureintro; exact Hresult.1
  · icases Hfailure with (Hparse | Htrailing)
    · icases Hparse with ⟨%offset, %reason, %err, Hdst, Herr, %Hresult⟩
      iexists HeaderDecodeResult.failure headerZeroValue (.decode offset reason)
      isplitl []
      · ipureintro; exact Hresult.2.1
      isimp only [HeaderFullResultOwned]
      iframe Hdst
      isimp only [HeaderAPIErrorOwned]
      iexists kinds, err
      iframe Hglobals Herr
      ipureintro; exact ⟨Hresult.1, Hresult.2.2.1⟩
    · icases Htrailing with ⟨%extra, %err, Hdst, Herr, %Hresult⟩
      iexists HeaderDecodeResult.failure headerZeroValue (.trailing extra)
      isplitl []
      · ipureintro; exact Hresult.2
      isimp only [HeaderFullResultOwned]
      iframe Hdst
      isimp only [HeaderAPIErrorOwned]
      iexists extraKind, err
      iframe Hextra Herr
      ipureintro; exact Hresult.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFirstZeroOutcome_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.headerFullZeroOutcome_spec
