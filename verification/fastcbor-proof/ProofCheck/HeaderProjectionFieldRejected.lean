import ProofCheck.HeaderProjectionFieldRead

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

def headerProjectionFieldFailureAddr : HeaderProjectionFieldFailure → Loc
  | .felt reason => feltFailureAddr (GF := GF) reason
  | .scalar reason => expectedFailureAddr (GF := GF) reason

def headerProjectionFieldFailureKind (reason : HeaderProjectionFieldFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .felt reason => feltFailureKind reason kinds
  | .scalar reason => expectedFailureKind reason kinds

def HeaderProjectionFieldError (err : Loc) (pos : w64) (reason : HeaderProjectionFieldFailure) (category : GoInterface) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason category
  | .scalar reason => ExpectedFailureError err pos (W8 0) reason category

/-- Every selected projection field realizes the independent rejection policy,
    preserving the entire owned destination and cursor while returning an exact
    owned diagnostic with a non-wrapping position within the input or at EOF. -/
theorem wp_headerProjectionField_rejected (d offset : Nat) (kind : HeaderProjectionKind) (field : HeaderProjectionField)
    (c dst : Loc) (old : HeaderProjectionValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : HeaderProjectionFieldFailure) (category : GoInterface)
    (member : field ∈ headerProjectionFields kind) (failure : HeaderProjectionFieldRejected d field wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       headerProjectionFieldFailureAddr (GF := GF) reason ↦□ category ∗
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (headerProjectionFieldReader field))) (Val #c))
        (Val #(headerProjectionFieldRef (GF := GF) kind field dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionOwned kind dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       HeaderProjectionFieldError err (v.pos' + W64 offset) reason category ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧ sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  cases failure with
  | hash failure =>
    cases kind <;> simp [headerProjectionFields] at member
    case hash =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_failure (GF := GF) .hash c dst h old v xs wire dq d offset _ category failure hd depth hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr, %Hoffset⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      iapply HΦ
      isimp only [HeaderProjectionFieldError]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        iframe Hh Hchildren
      ipureintro
      exact Hoffset
    case hashAndRoot =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_failure (GF := GF) .pairHash c dst h old v xs wire dq d offset _ category failure hd depth hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr, %Hoffset⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      iapply HΦ
      isimp only [HeaderProjectionFieldError]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        iframe Hh Hchildren
      ipureintro
      exact Hoffset
  | root failure =>
    cases kind <;> simp [headerProjectionFields] at member
    case root =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_failure (GF := GF) .root c dst h old v xs wire dq d offset _ category failure hd depth hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr, %Hoffset⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      iapply HΦ
      isimp only [HeaderProjectionFieldError]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        iframe Hh Hchildren
      ipureintro
      exact Hoffset
    case hashAndRoot =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      have hread := wp_headerProjectionFelt_failure (GF := GF) .pairRoot c dst h old v xs wire dq d offset _ category failure hd depth hp hbytes
      simp only [headerProjectionFeltRef] at hread
      wp_apply hread $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr, %Hoffset⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      iapply HΦ
      isimp only [HeaderProjectionFieldError]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        iframe Hh Hchildren
      ipureintro
      exact Hoffset
  | count failure =>
    cases kind <;> simp [headerProjectionFields] at member
    case count =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      isimp only [HeaderProjectionCells] at Hh
      wp_apply wp_headerProjectionCount_failure c dst h old v xs wire dq _ category failure hp hbytes
        $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      have hzero : v.pos' + W64 0 = v.pos' := by word
      iapply HΦ
      isimp only [HeaderProjectionFieldError, Int.natCast_zero, hzero, Int.add_zero]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        isimp only [HeaderProjectionCells]
        iframe Hh Hchildren
      ipureintro
      exact ⟨trivial, hp.2⟩
  | timestamp failure =>
    cases kind <;> simp [headerProjectionFields] at member
    case timestamp =>
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName]
      wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
      isimp only [headerProjectionFieldFailureAddr] at Hcategory
      isimp only [HeaderProjectionOwned] at Howned
      icases Howned with ⟨%h, Hh, Hchildren⟩
      isimp only [HeaderProjectionCells] at Hh
      wp_apply wp_headerProjectionTimestamp_failure c dst h old v xs wire dq _ category failure hp hbytes
        $$ [Hh Hchildren Hc Hs] as %err ⟨Hh, Hchildren, Hc, Hs, Herr⟩
      · iframe Hcategory Hh Hchildren Hc Hs
      have hzero : v.pos' + W64 0 = v.pos' := by word
      iapply HΦ
      isimp only [HeaderProjectionFieldError, Int.natCast_zero, hzero, Int.add_zero]
      iframe Hc Hs Herr
      isplitl [Hh Hchildren]
      · unfold HeaderProjectionOwned
        iexists h
        isimp only [HeaderProjectionCells]
        iframe Hh Hchildren
      ipureintro
      exact ⟨trivial, hp.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerProjectionField_rejected
