import ProofCheck.SignaturesClassify

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

/-- Successful signature decoding owns the outer headers and every nested
    pointer table/backing array, or yields nil. Failure preserves the old outer
    destination and retains the specified nested cursor state and diagnostic.
    Full input ownership is framed separately in both outcomes. -/
def SignaturesOutcome (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (d : Nat) (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (value : Option (List SignatureValue)) (p : GoSlice),
    dst ↦ p ∗ SignaturesResult (GF := GF) p value ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ SignaturesValid d item value ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset : Nat) (extraDepth : Nat) (reason : FeltFailureReason) (err : Loc), dst ↦ old ∗
    c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
    FeltFailureError err (v.pos' + W64 offset) reason (feltFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      SignaturesFailure d wire consumed offset extraDepth reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- The actual signature reader has a specified owned nested value or failure
    on arbitrary input at supported depth, with unchanged input and precise
    consumption/error positions. All child and outer loop proofs are discharged. -/
theorem wp_readSignatures_complete (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d : Nat) (kinds : FeltErrorKinds)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ SignaturesOutcome c dst old v xs wire d kinds result }} := by
  wp_start_folded as ⟨Hglobals, Hdst, Hc, Hs⟩
  rcases signatures_classify d wire hdepth with ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
  · wp_apply wp_readSignatures_valid c dst old v xs item rest value dq d hd hdepth valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold SignaturesOutcome
    ileft
    iexists item, rest, value, p
    iframe Hdst Hvalue Hc
    ipureintro
    exact ⟨rfl, split, valid, Hnext⟩
  · ihave #Hkind := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readSignatures_failure_bounded c dst old v xs wire dq d consumed offset extraDepth reason (feltFailureKind reason kinds)
      failure hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hoffset⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    iframe Hs
    unfold SignaturesOutcome
    iright
    iexists consumed, offset, extraDepth, reason, err
    iframe Hdst Hc Herr
    ipureintro
    exact ⟨rfl, failure, Hoffset⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_complete
