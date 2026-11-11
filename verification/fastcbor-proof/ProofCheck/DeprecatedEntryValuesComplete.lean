import ProofCheck.DeprecatedEntryValuesReadValid
import ProofCheck.DeprecatedEntryValuesReadRejected
import ProofCheck.DeprecatedEntryValuesUnique

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

/-- Concrete caller-visible outcomes. Successful slices own their exact records;
    failed reads preserve the original destination and own the exact diagnostic.
    Temporary failed allocations may be forgotten by the caller, as permitted
    by the garbage-collected model; their ownership is proved in the loop. -/
def DeprecatedEntryValuesReadOwned (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (d : Nat) (result : GoInterface) : DeprecatedEntryValuesOutcome → IProp GF
  | .success values rest => iprop(∃ out : GoSlice,
      dst ↦ out ∗ DeprecatedEntryValuesResult out values ∗
      c ↦ {v with pos' := v.pos' + W64 (wire.length - rest.length : Nat)} ∗
      ⌜result = interface.nil ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) = sint.Z v.pos' + (wire.length - rest.length : Nat) ∧
        sint.Z (v.pos' + W64 (wire.length - rest.length : Nat)) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 (wire.length - rest.length : Nat))) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure consumed offset extraDepth reason => iprop(∃ err : Loc,
      dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      DeprecatedEntryValuesFailureOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
        sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝)

/-- All byte inputs and arbitrary old slice headers are covered by the actual
    generated decoder. Classification and outcome uniqueness are independent
    proofs; the only unproved helper dependency is the existing UTF-8 validator
    used by DeprecatedEntryPoint record key decoding. -/
theorem wp_readDeprecatedEntryValues_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadDeprecatedEntryPointValues)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : DeprecatedEntryValuesOutcome,
       ⌜DeprecatedEntryValuesSpec d wire outcome ∧
         ∀ other, DeprecatedEntryValuesSpec d wire other → other = outcome⌝ ∗
       DeprecatedEntryValuesReadOwned c dst old v xs wire d result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  obtain ⟨outcome, spec, unique⟩ := deprecatedEntryValuesSpec_total_unique d wire hdepth
  cases outcome with
  | success values rest =>
    wp_apply wp_readDeprecatedEntryValues_valid validator c dst old v xs wire rest values dq d spec hd hp hbytes
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeprecatedEntryValuesOutcome.success values rest
    isplitr
    · ipureintro; exact ⟨spec, unique⟩
    unfold DeprecatedEntryValuesReadOwned
    iexists out
    iframe Hdst Howned Hc
    ipureintro; exact ⟨rfl, Hstep⟩
  | failure consumed offset extraDepth reason =>
    wp_apply wp_readDeprecatedEntryValues_rejected validator c dst old v xs wire dq d consumed offset extraDepth reason
      spec hd hdepth hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hstep⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists DeprecatedEntryValuesOutcome.failure consumed offset extraDepth reason
    isplitr
    · ipureintro; exact ⟨spec, unique⟩
    unfold DeprecatedEntryValuesReadOwned
    iexists err
    iframe Hdst Hc Herr
    ipureintro; exact ⟨rfl, Hstep⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readDeprecatedEntryValues_complete
