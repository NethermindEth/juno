import ProofCheck.IndexesMapRejected
import ProofCheck.IndexesOpen

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

/-- Every specified record rejection, including opening failures and failures
    after arbitrary published prefixes, has its exact owned output and error. -/
theorem wp_readIndexes_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (h : BlockTransactionsIndexes) (initial : IndexesValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat) (final : IndexesValue)
    (reason : IndexesRecordFailureReason) (k : BigIntErrorKinds) (depthKind : Loc)
    (failure : IndexesRecordRejected d initial wire final consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ h ∗ IndexesChildren h initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBlockTransactionsIndexes)) (Val #c)) (Val #dst))
    {{ (result : GoInterface) (out : BlockTransactionsIndexes), RET #result;
       dst ↦ out ∗ IndexesChildren out final ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ IndexesRecordError result (v.pos' + W64 offset) reason k depthKind ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + extraDepth ≤ 32⌝ }} := by
  wp_start_folded as ⟨#Hglobals, #Hdepth, Hdst, Hchildren, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have bounds := indexesRecordRejected_bounds d consumed offset extraDepth initial final wire reason hdepth failure
  have hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧ d + extraDepth ≤ 32 := by
    exact ⟨by word, by word, by word, by word, bounds.2.2⟩
  cases failure with
  | @opening wire reason failure =>
    ihave Hfelt := indexesFelt_globals k depthKind $$ []
    · iframe Hglobals Hdepth
    ihave #Hkind := feltErrorGlobals_lookup (indexesFeltKinds k depthKind) reason $$ Hfelt
    wp_apply wp_readIndexes_open_failure c dst h v xs wire dq d reason (feltFailureKind reason (indexesFeltKinds k depthKind))
      failure hd hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    have hz (x : w64) : x + W64 0 = x := by word
    isimp only [IndexesRecordError, Int.natCast_zero, hz] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs
    isplitl [Herr]
    · iexists err
      iframe Herr
      ipureintro; rfl
    ipureintro
    simpa only [Int.natCast_zero, hz] using hprogress
  | @map head payload body argument count consumed offset extraDepth final reason enc major countEq depth fit failure =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readIndexes_map_rejected validator c dst h initial v xs payload body head dq d count consumed offset extraDepth
      final reason k depthKind enc major hd depth fit failure hp hbytes $$ [Hdst Hchildren Hc Hs]
      as %result %out ⟨Hdst, Hchildren, Hc, Hs, Herr, %Hpositions⟩
    · iframe Hglobals Hdepth Hdst Hchildren Hc Hs
    have hdepthStep : v.depth' + W64 (1 + extraDepth : Nat) = v.depth' + W64 1 + W64 extraDepth := by word
    simp only [Int.natCast_add, Int.natCast_one] at hdepthStep
    isimp only [Int.natCast_add, Int.natCast_one, hdepthStep] at HΦ
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Herr
    ipureintro
    simpa only [Int.natCast_add, Int.natCast_one] using hprogress

/-- The complete error relation retains a concrete nonnil representation. -/
theorem indexesRecordError_shape (result : GoInterface) (pos : w64) (reason : IndexesRecordFailureReason)
    (k : BigIntErrorKinds) (depthKind : Loc) :
    IndexesRecordError (GF := GF) result pos reason k depthKind ⊢ ⌜IndexesErrorShape result k⌝ := by
  cases reason with
  | opening reason =>
    unfold IndexesRecordError
    iintro ⟨%err, %h, _⟩
    ipureintro
    exact Or.inr ⟨err, h⟩
  | key reason => exact integerKeyError_shape result pos reason k
  | value reason =>
    cases reason with
    | selected reason => exact intsFailureError_shape result pos reason k depthKind
    | ignored reason =>
      unfold IndexesRecordError
      iintro ⟨%err, %h, _⟩
      ipureintro
      exact Or.inr ⟨err, h⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIndexes_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordError_shape
