import ProofCheck.SegmentRecordRejectedStep
import ProofCheck.SegmentArrayRejectedStep

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

/-- Actual recursive failures are proved jointly by decreasing depth. Success
    prefixes use the already closed success proof; no child WP remains assumed. -/
theorem segment_rejected_contracts_depth (validator : Utf8ValidatorContract (GF := GF)) (fuel : Nat) :
    ∀ d : Nat, d ≤ 32 → 32 - d = fuel →
      SegmentRecordRejectedContract (GF := GF) d ∧ SegmentArrayRejectedContract (GF := GF) d := by
  induction fuel with
  | zero =>
    intro d depth remaining
    have impossible : ¬ d < 32 := by omega
    exact ⟨wp_readSegment_record_rejected_allocated_of_array validator d (fun inside => False.elim (impossible inside)),
      wp_readSegmentArray_rejected_of_record validator d (fun inside => False.elim (impossible inside))⟩
  | succ fuel ih =>
    intro d depth remaining
    have next : d < 32 → SegmentRecordRejectedContract (GF := GF) (d + 1) ∧ SegmentArrayRejectedContract (GF := GF) (d + 1) := by
      intro inside
      exact ih (d + 1) (by omega) (by omega)
    exact ⟨wp_readSegment_record_rejected_allocated_of_array validator d (fun inside => (next inside).2),
      wp_readSegmentArray_rejected_of_record validator d (fun inside => (next inside).1)⟩

theorem wp_readSegment_record_rejected_allocated (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (d consumed offset : Nat) (depth : d ≤ 32) (extraDepth : Nat) (reason : SegmentFailureReason)
    (failure : SegmentRecordRejected d old wire final consumed offset extraDepth reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       SegmentOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentDiagnosticOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} :=
  (segment_rejected_contracts_depth validator (32 - d) d depth rfl).1
    c dst old final v xs wire dq consumed offset extraDepth reason failure hd hp hbytes

theorem wp_readSegmentArray_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (d consumed offset extraDepth : Nat) (reason : SegmentArrayFailureReason)
    (failure : SegmentArraySpec d wire (.failure consumed offset extraDepth reason))
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentArrayFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝ }} :=
  (segment_rejected_contracts_depth validator (32 - d) d hdepth rfl).2
    c dst old v xs wire dq consumed offset extraDepth reason failure hd hdepth hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_rejected_contracts_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_record_rejected_allocated
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_rejected
