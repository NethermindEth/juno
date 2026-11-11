import ProofCheck.SegmentRecordComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def SegmentArrayReadOwned (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (d : Nat) (result : GoInterface) : SegmentArrayOutcome → IProp GF
  | .success values rest => iprop(∃ out : GoSlice,
      dst ↦ out ∗ SegmentArrayResult out values ∗
      c ↦ {v with pos' := v.pos' + W64 (wire.length - rest.length : Nat)} ∗
      ⌜result = interface.nil ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) = sint.Z v.pos' + (wire.length - rest.length : Nat) ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 (wire.length - rest.length : Nat))) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset extraDepth reason => iprop(∃ err : Loc,
      dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      SegmentArrayFailureOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝)

/-- All byte inputs and arbitrary old slice headers are covered by the actual
    generated decoder. Classification and outcome uniqueness are independent
    proofs; the only unproved helper dependency is the existing UTF-8 validator
    used by SegmentLengths record key decoding. -/
theorem wp_readSegmentArray_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : SegmentArrayOutcome,
       ⌜SegmentArraySpec d wire outcome ∧
         ∀ other, SegmentArraySpec d wire other → other = outcome⌝ ∗
       SegmentArrayReadOwned c dst old v xs wire d result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  obtain ⟨outcome, spec, unique⟩ := segmentArraySpec_total_unique d wire hdepth
  cases outcome with
  | success values rest =>
    wp_apply wp_readSegmentArray_valid validator c dst old v xs wire rest values dq d hdepth spec hd hp hbytes
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists SegmentArrayOutcome.success values rest
    isplitr
    · ipureintro; exact ⟨spec, unique⟩
    unfold SegmentArrayReadOwned
    iexists out
    iframe Hdst Howned Hc
    ipureintro; exact ⟨rfl, Hstep⟩
  | failure consumed offset extraDepth reason =>
    wp_apply wp_readSegmentArray_rejected validator c dst old v xs wire dq d consumed offset extraDepth reason
      spec hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists SegmentArrayOutcome.failure consumed offset extraDepth reason
    isplitr
    · ipureintro; exact ⟨spec, unique⟩
    unfold SegmentArrayReadOwned
    iexists err
    iframe Hdst Hc Herr
    ipureintro; exact ⟨rfl, Hstep⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_complete
