import ProofCheck.SkipClassify

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

/-- The four initialized error globals used by Skip. Their initialization is a
    separate obligation; no equality or distinctness of their values is assumed. -/
structure SkipErrorKinds where
  syntaxKind : GoInterface
  truncatedKind : GoInterface
  unsupportedKind : GoInterface
  depthKind : GoInterface

def SkipErrorGlobals (kinds : SkipErrorKinds) : IProp GF := iprop(
  globalAddr cursor.ErrSyntax ↦□ kinds.syntaxKind ∗
  globalAddr cursor.ErrTruncated ↦□ kinds.truncatedKind ∗
  globalAddr cursor.ErrUnsupported ↦□ kinds.unsupportedKind ∗
  globalAddr cursor.ErrDepth ↦□ kinds.depthKind)

def skipFailureKind (reason : SkipFailureReason) (kinds : SkipErrorKinds) : GoInterface :=
  match reason with
  | .head _ .indefinite | .atomic .tag => kinds.unsupportedKind
  | .head _ .breakCode | .head _ .invalid | .atomic .simple => kinds.syntaxKind
  | .incomplete | .atomic .truncated | .eof | .count => kinds.truncatedKind
  | .depth => kinds.depthKind

theorem skipErrorGlobals_lookup (kinds : SkipErrorKinds) (reason : SkipFailureReason) :
    SkipErrorGlobals kinds ⊢ (skipFailureAddr (GF := GF) reason ↦□ skipFailureKind reason kinds : IProp GF) := by
  unfold SkipErrorGlobals
  iintro ⟨#Hsyntax, #Htruncated, #Hunsupported, #Hdepth⟩
  cases reason with
  | head bad reason =>
    cases reason <;> isimp only [skipFailureAddr, headFaultAddr, skipFailureKind] <;> (first | iexact Hsyntax | iexact Htruncated | iexact Hunsupported | iexact Hdepth)
  | atomic reason =>
    cases reason <;> isimp only [skipFailureAddr, skipAtomErrorAddr, skipFailureKind] <;> (first | iexact Hsyntax | iexact Htruncated | iexact Hunsupported | iexact Hdepth)
  | incomplete | eof | count | depth =>
    isimp only [skipFailureAddr, skipFailureKind]
    (first | iexact Hsyntax | iexact Htruncated | iexact Hunsupported | iexact Hdepth)

/-- Both possible public outcomes, with the unchanged full input framed outside.
    Success consumes exactly a valid item; failure restores the original cursor
    and owns the specified inner diagnostic, whose offset cannot wrap. -/
def SkipOutcome (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (d : Nat)
    (kinds : SkipErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ item rest : List w8, c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ SkipValid d item ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : SkipFailureReason) (err : Loc), c ↦ v ∗
    SkipFailureError err (v.pos' + W64 offset) reason (skipFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      SkipFailure d wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- Public Skip on arbitrary input. Classification supplies every recursive
    success/failure premise, then the checked production contracts discharge it.
    The precondition imposes no well-formedness or chosen-outcome requirement. -/
theorem wp_Skip_complete (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d : Nat) (kinds : SkipErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       SkipErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ SkipOutcome c v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hc, Hs⟩
  rcases skip_classify d wire hdepth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_Skip_valid c v xs item rest dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hc Hs] as ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    unfold SkipOutcome
    ileft
    iexists item, rest
    iframe Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := skipErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_Skip_failure_bounded c v xs wire dq d offset reason (skipFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    iframe Hs
    unfold SkipOutcome
    iright
    iexists offset, reason, err
    iframe Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipErrorGlobals_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_complete
