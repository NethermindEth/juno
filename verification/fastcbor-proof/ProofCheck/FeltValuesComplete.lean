import ProofCheck.FeltValuesFailure

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

/-- Concrete ownership for each independent slice outcome. Failed decoding
    returns nil and restores the cursor while retaining the inner error offset. -/
def FeltValuesDecoded (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (kinds : FeltErrorKinds)
    (out : GoSlice) (result : GoInterface) : FeltValuesOutcome → IProp GF
  | .success values rest => iprop(⌜result = interface.nil⌝ ∗ FeltValuesSuccess c v xs wire out values rest)
  | .failure offset reason => iprop(c ↦ v ∗ ∃ err : Loc,
      FeltValuesFailureError err (v.pos' + W64 offset) reason kinds ∗
      ⌜out = slice.nil ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Refinement of every independent outcome, with no assumed parser, allocator
    or unsafe-view helper correctness. -/
theorem wp_limbSlice_spec (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (d : Nat) (kinds : FeltErrorKinds) (outcome : FeltValuesOutcome)
    (spec : FeltValuesSpec d wire outcome) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (out : GoSlice) (result : GoInterface), RET (PairV #out #result);
       v.buf' ↦*{dq} xs ∗ FeltValuesDecoded c v xs wire kinds out result outcome }} := by
  wp_start_folded as ⟨#Hglobals, Hc, Hs⟩
  cases outcome with
  | success values rest =>
    wp_apply wp_limbSlice_valid c v xs wire rest values dq d spec hd hp hbytes
      $$ [Hc Hs] as %out ⟨Hs, Hvalue⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [FeltValuesDecoded]
    iframe Hvalue
  | failure offset reason =>
    wp_apply wp_limbSlice_failure c v xs wire dq d offset reason kinds spec hd hdepth hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoffset⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    isimp only [FeltValuesDecoded]
    iframe Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨trivial, rfl, Hoffset⟩

/-- Every byte input has a checked value/suffix or precise rejection contract.
    Actual initialization supplies all diagnostic categories. -/
theorem wp_limbSlice_complete (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val #(functions cursor.LimbSlice [felt.Felt.ty])) (Val #c))
    {{ (out : GoSlice) (result : GoInterface), RET (PairV #out #result);
       v.buf' ↦*{dq} xs ∗ ∃ (outcome : FeltValuesOutcome) (kinds : FeltErrorKinds),
       FeltErrorGlobals kinds ∗ ⌜FeltValuesSpec d wire outcome⌝ ∗
       FeltValuesDecoded c v xs wire kinds out result outcome }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec⟩ := feltValues_classify d wire hdepth
  wp_apply wp_limbSlice_spec c v xs wire dq d kinds outcome spec hd hdepth hp hbytes
    $$ [Hc Hs] as %out %result ⟨Hs, Houtcome⟩
  · iframe Hglobals Hc Hs
  iapply HΦ
  iframe Hs
  iexists outcome, kinds
  iframe Hglobals Houtcome
  ipureintro; exact spec

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbSlice_complete
