import ProofCheck.CasmEntryPointRecordComplete

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

/-- Every CASM record error is an allocated diagnostic, including failures
    propagated from builtin arrays. -/
theorem casmEntryPointFailureOwned_allocated (result : GoInterface) (pos : w64) (reason : CasmEntryPointFailureReason) :
    CasmEntryPointFailureOwned (GF := GF) result pos reason ⊢
      ⌜∃ err : Loc, result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ := by
  isimp only [CasmEntryPointFailureOwned]
  iintro ⟨%kinds, _, Herr⟩
  cases reason with
  | opening | key =>
    isimp only [CasmEntryPointFailureError] at Herr
    icases Herr with ⟨%err, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | duplicate =>
    isimp only [CasmEntryPointFailureError] at Herr
    icases Herr with ⟨%err, %args, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | value reason => cases reason with
    | ignored =>
      isimp only [CasmEntryPointFailureError] at Herr
      icases Herr with ⟨%err, %Heq, _⟩
      ipureintro; exact ⟨err, Heq⟩
    | field reason =>
      cases reason <;> isimp only [CasmEntryPointFailureError, CasmEntryPointFieldError] at Herr
      all_goals
        icases Herr with ⟨%err, %Heq, _⟩
        ipureintro; exact ⟨err, Heq⟩

def CasmEntryPointDiagnosticOwned (err : Loc) (pos : w64) (reason : CasmEntryPointFailureReason) : IProp GF :=
  CasmEntryPointFailureOwned (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err) pos reason

theorem wp_readCasmEntryPoint_record_rejected_allocated (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : CasmEntryPointValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (extraDepth : Nat) (reason : CasmEntryPointFailureReason)
    (failure : CasmEntryPointRecordRejected d old wire final consumed offset extraDepth reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmEntryPointOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadCasmEntryPoint)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       CasmEntryPointOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ CasmEntryPointDiagnosticOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply wp_readCasmEntryPoint_record_rejected validator c dst old final v xs wire dq d consumed offset extraDepth reason
    failure hd hp hbytes $$ [Howned Hc Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
  · iframe Howned Hc Hs
  ihave %Hallocated := casmEntryPointFailureOwned_allocated result (v.pos' + W64 offset) reason $$ Herr
  obtain ⟨err, rfl⟩ := Hallocated
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold CasmEntryPointDiagnosticOwned; iexact Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.casmEntryPointFailureOwned_allocated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readCasmEntryPoint_record_rejected_allocated
