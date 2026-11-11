import ProofCheck.ProjectionTagChecked
import ProofCheck.InitializationErrors

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

def ProjectionTagError (result : GoInterface) (pos : w64) (kinds : FeltErrorKinds) : ProjectionTagStatus → IProp GF
  | .success => iprop(⌜result = interface.nil⌝)
  | .unsupported => iprop(⌜result = kinds.unsupportedKind⌝)
  | .malformed reason => iprop(∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      ExpectedFailureError err pos (W8 192) reason (expectedFailureKind reason kinds))

/-- Each independently specified tag-prelude outcome is realized by the actual
    implementation, with exact suffix, unchanged depth and concrete diagnostics. -/
theorem wp_projectionTag_spec (c : Loc) (v : cursor.Cursor) (xs wire rest : List w8)
    (dq : DFrac) (consumed : Nat) (status : ProjectionTagStatus) (kinds : FeltErrorKinds)
    (spec : ProjectionTagSpec wire consumed rest status)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ (result : GoInterface), RET #result;
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ v.buf' ↦*{dq} xs ∗
       ProjectionTagError result v.pos' kinds status ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨#Hglobals, Hc, Hs⟩
  ihave #Htruncated : (globalAddr cursor.ErrTruncated ↦□ kinds.truncatedKind : IProp GF) $$ []
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, #Htruncated, _, _⟩, _⟩
    iexact Htruncated
  ihave #Hunsupported : (globalAddr cursor.ErrUnsupported ↦□ kinds.unsupportedKind : IProp GF) $$ []
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, #Hunsupported, _⟩, _⟩
    iexact Hunsupported
  have hz : v.pos' + W64 0 = v.pos' := by word
  cases spec with
  | untagged plain =>
    wp_apply wp_projectionTag_untagged c v xs wire dq kinds.truncatedKind plain hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Htruncated Hc Hs
    iapply HΦ
    isimp only [Int.natCast_zero, hz, ProjectionTagError]
    iframe Hc Hs
    ipureintro; exact ⟨trivial, by omega, hp.2, hbytes⟩
  | @allowed head payload rest tag enc major allowed =>
    wp_apply wp_projectionTag_checked c v xs payload rest head tag dq kinds.truncatedKind kinds.unsupportedKind
      enc major hp hbytes $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Htruncated Hunsupported Hc Hs
    have hsize : ((head :: payload).length : Int) = 1 + payload.length := by simp; omega
    iapply HΦ
    isimp only [hsize, ProjectionTagError, projectionTagResult, if_neg allowed]
    iframe Hc Hs
    ipureintro; exact ⟨trivial, by simpa only [Int.add_assoc] using Hnext⟩
  | @malformed wire reason tagged failure =>
    ihave #Hkind := feltErrorGlobals_lookup (GF := GF) kinds (.expected (W8 192) reason) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_projectionTag_malformed c v xs wire dq reason (expectedFailureKind reason kinds)
      tagged failure hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [Int.natCast_zero, hz]
    iframe Hc Hs
    isplitl [Herr]
    · isimp only [ProjectionTagError]
      iexists err
      iframe Herr
      ipureintro; rfl
    · ipureintro; exact ⟨by omega, hp.2, hbytes⟩
  | @unsupported head payload rest tag enc major rejected =>
    wp_apply wp_projectionTag_checked c v xs payload rest head tag dq kinds.truncatedKind kinds.unsupportedKind
      enc major hp hbytes $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Htruncated Hunsupported Hc Hs
    have hsize : ((head :: payload).length : Int) = 1 + payload.length := by simp; omega
    iapply HΦ
    isimp only [hsize, ProjectionTagError, projectionTagResult, if_pos rejected]
    iframe Hc Hs
    ipureintro; exact ⟨trivial, by simpa only [Int.add_assoc] using Hnext⟩

/-- Every byte input is classified; actual cursor initialization supplies the
    diagnostic categories. No tag validity or helper-correctness premise remains. -/
theorem wp_projectionTag_complete (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (@! cborReadProjectionTag)) (Val #c))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (consumed : Nat) (rest : List w8) (status : ProjectionTagStatus) (kinds : FeltErrorKinds),
       FeltErrorGlobals kinds ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
       ProjectionTagError result v.pos' kinds status ∗
       ⌜ProjectionTagSpec wire consumed rest status ∧
         sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply cursor_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨consumed, rest, status, spec⟩ := projectionTag_classify wire
  wp_apply wp_projectionTag_spec c v xs wire rest dq consumed status kinds spec hp hbytes
    $$ [Hc Hs] as %result ⟨Hc, Hs, Herror, %Hnext⟩
  · iframe Hglobals Hc Hs
  iapply HΦ
  iframe Hs
  iexists consumed, rest, status, kinds
  iframe Hglobals Hc Herror
  ipureintro; exact ⟨spec, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_projectionTag_complete
