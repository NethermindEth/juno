import ProofCheck.FeltFailureSound

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

/-- Felt adds type errors to the four categories already used by Skip. -/
structure FeltErrorKinds extends SkipErrorKinds where
  typeKind : GoInterface

def FeltErrorGlobals (kinds : FeltErrorKinds) : IProp GF := iprop(
  SkipErrorGlobals kinds.toSkipErrorKinds ∗ globalAddr cursor.ErrType ↦□ kinds.typeKind)

def expectedFailureKind (reason : ExpectedFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .head bad reason => skipFailureKind (.head bad reason) kinds.toSkipErrorKinds
  | .incomplete | .eof => kinds.truncatedKind
  | .typeMismatch _ => kinds.typeKind
  | .tag => kinds.unsupportedKind

def feltFailureKind (reason : FeltFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .expected _ reason => expectedFailureKind reason kinds
  | .depth => kinds.depthKind
  | .skipped reason => skipFailureKind reason kinds.toSkipErrorKinds

theorem feltErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : FeltFailureReason) :
    FeltErrorGlobals kinds ⊢ (feltFailureAddr (GF := GF) reason ↦□ feltFailureKind reason kinds : IProp GF) := by
  unfold FeltErrorGlobals
  iintro ⟨Hskip, #Htype⟩
  cases reason with
  | skipped reason =>
    isimp only [feltFailureAddr, feltFailureKind]
    iapply skipErrorGlobals_lookup kinds.toSkipErrorKinds reason
    iexact Hskip
  | depth =>
    isimp only [feltFailureAddr, feltFailureKind]
    ihave Hkind := skipErrorGlobals_lookup kinds.toSkipErrorKinds .depth $$ Hskip
    isimp only [skipFailureAddr, skipFailureKind] at Hkind
    iexact Hkind
  | expected want reason =>
    cases reason with
    | head bad reason =>
      isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind]
      ihave Hkind := skipErrorGlobals_lookup kinds.toSkipErrorKinds (.head bad reason) $$ Hskip
      isimp only [skipFailureAddr] at Hkind
      iexact Hkind
    | incomplete | eof =>
      isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind]
      ihave Hkind := skipErrorGlobals_lookup kinds.toSkipErrorKinds .eof $$ Hskip
      isimp only [skipFailureAddr, skipFailureKind] at Hkind
      iexact Hkind
    | tag =>
      isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind]
      ihave Hkind := skipErrorGlobals_lookup kinds.toSkipErrorKinds (.atomic .tag) $$ Hskip
      isimp only [skipFailureAddr, skipAtomErrorAddr, skipFailureKind] at Hkind
      iexact Hkind
    | typeMismatch bad =>
      isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind]
      iexact Htype

/-- A successful felt owns its independent output (or is nil), and consumes the
    exact specified prefix. A failed felt preserves its prior pointer and cursor
    while owning the exact inner diagnostic. Full input is framed separately. -/
def FeltOutcome (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (GoArray w64 4)) (p : Loc),
    dst ↦ p ∗ FeltResult (GF := GF) p value ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ FeltValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : FeltFailureReason) (err : Loc), dst ↦ old ∗ c ↦ v ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      FeltFailure d wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The translated production typed felt reader has a specified value or error
    outcome on arbitrary input. This composes exhaustive classification and all
    actual success/failure proofs, with no chosen-outcome or helper-WP premise. -/
theorem wp_readFelt_complete (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ FeltOutcome c dst old v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hdst, Hc, Hs⟩
  rcases felt_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_readFelt_valid c dst old v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold FeltOutcome
    ileft
    iexists item, rest, value, p
    iframe Hdst Hvalue Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readFelt_failure_bounded c dst old v xs wire dq d offset reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold FeltOutcome
    iright
    iexists offset, reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltErrorGlobals_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_complete
