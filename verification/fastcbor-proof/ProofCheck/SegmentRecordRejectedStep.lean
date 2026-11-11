import ProofCheck.SegmentMapRejected

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

theorem wp_readSegment_record_rejected_of_array (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (next : d < 32 → SegmentArrayRejectedContract (GF := GF) (d + 1))
    (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (consumed offset : Nat) (extraDepth : Nat) (reason : SegmentFailureReason)
    (failure : SegmentRecordRejected d old wire final consumed offset extraDepth reason) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       SegmentOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ SegmentFailureOwned result (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  cases failure with
  | opening failure =>
    wp_apply wp_readSegment_open_rejected c dst old v xs wire dq d _ failure hd hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
    · iframe Howned Hc Hs
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [Int.natCast_zero, Int.add_zero, hz, show v.depth' + W64 0 = v.depth' from by word] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro; exact ⟨trivial, trivial, Int.le_refl _, hp.2⟩
  | @map _ _ head payload body argument final consumed offset nestedDepth reason count enc major size depth fit pairs =>
    have argumentEq : argument = W64 count := by word
    subst argument
    wp_apply wp_readSegment_map_rejected_of_array validator c dst old final v xs payload body head dq d (next depth) count consumed offset nestedDepth reason
      enc major hd depth fit pairs hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    isimp only [Int.natCast_add, Int.natCast_one, ite_true] at HΦ
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro; simpa only [Int.add_assoc] using Hnext

theorem wp_readSegment_record_rejected_allocated_of_array (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (next : d < 32 → SegmentArrayRejectedContract (GF := GF) (d + 1))
    (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (consumed offset : Nat) (extraDepth : Nat) (reason : SegmentFailureReason)
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
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  wp_apply wp_readSegment_record_rejected_of_array validator d next c dst old final v xs wire dq consumed offset extraDepth reason
    failure hd hp hbytes $$ [Howned Hc Hs] as %result ⟨Howned, Hc, Hs, Herr, %Hnext⟩
  · iframe Howned Hc Hs
  ihave %Hallocated := segmentFailureOwned_allocated result (v.pos' + W64 offset) reason $$ Herr
  obtain ⟨err, rfl⟩ := Hallocated
  iapply HΦ
  iframe Howned Hc Hs
  isplitl [Herr]
  · unfold SegmentDiagnosticOwned; iexact Herr
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_record_rejected_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_record_rejected_allocated_of_array
