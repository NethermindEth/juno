import ProofCheck.ReceiptEventsMapRejected
import ProofCheck.ReceiptEventsTagBoundary
import ProofCheck.ReceiptEventsRecordError

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 12800000

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

/-- Every rejected body after an independently accepted optional tag is realized
    by the actual reader. Earlier published fields, retained map depth and both
    absolute positions agree with the independent application specification. -/
theorem wp_readReceiptEvents_record_body_rejected (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (c dst : Loc) (old final : ReceiptEventsValue)
    (v : cursor.Cursor) (xs wire body : List w8) (dq : DFrac) (tagConsumed consumed offset : Nat)
    (extraDepth : Nat) (reason : ReceiptEventsRecordFailure) (kinds : FeltErrorKinds)
    (tag : ProjectionTagSpec wire tagConsumed body .success)
    (failure : ReceiptEventsBodyRejected d old body final consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       FeltErrorGlobals kinds ∗ ReceiptEventsOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptEventsProjection)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ReceiptEventsOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (tagConsumed + consumed), depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗
       ReceiptEventsRecordDiagnostic err (v.pos' + W64 (tagConsumed + offset)) reason kinds ∗
       ⌜sint.Z (v.pos' + W64 (tagConsumed + consumed)) = sint.Z v.pos' + tagConsumed + consumed ∧
         sint.Z (v.pos' + W64 (tagConsumed + consumed)) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (tagConsumed + offset)) = sint.Z v.pos' + tagConsumed + offset ∧
         sint.Z (v.pos' + W64 (tagConsumed + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start_folded as ⟨#Hglobals, Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  obtain ⟨tagBytes, splitTag, lengthTag⟩ := projectionTag_suffix wire body tagConsumed .success tag
  have Htag := input_consumption xs tagBytes body v.pos' v.buf'.len hp Hlen.1 (by rw [← splitTag]; exact hbytes)
  rw [lengthTag] at Htag
  have Hpos : 0 ≤ sint.Z (v.pos' + W64 tagConsumed) ∧ sint.Z (v.pos' + W64 tagConsumed) ≤ sint.Z v.buf'.len :=
    ⟨by rw [Htag.1]; omega, Htag.2.1⟩
  cases failure with
  | @opening body reason failure =>
    ihave #Hcategory := feltErrorGlobals_lookup kinds reason $$ Hglobals
    wp_apply wp_readReceiptEvents_open_failure_after_tag d c dst old
      {v with pos' := v.pos' + W64 tagConsumed} xs body dq reason (feltFailureKind reason kinds) failure hd Hpos Htag.2.2
      v wire tagConsumed kinds tag hp hbytes rfl $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
    · iframe Hglobals Hcategory Howned Hc Hs
    iapply HΦ
    have hz : v.depth' + W64 0 = v.depth' := by word
    isimp only [ReceiptEventsRecordDiagnostic, Bool.false_eq_true, ite_false, Int.natCast_zero, Int.add_zero, hz]
    iframe Howned Hc Hs Herr
    ipureintro
    exact ⟨Htag.1, Htag.2.1, Htag.1, Htag.2.1⟩
  | @map head payload body argument final consumed offset childDepth reason count enc major length depth fit failure =>
    have argumentEq : argument = W64 count := by word
    subst argument
    have position : v.pos' + W64 tagConsumed + W64 (1 + payload.length + consumed) =
        v.pos' + W64 (tagConsumed + (1 + payload.length + consumed)) := by word
    have errorPosition : v.pos' + W64 tagConsumed + W64 (1 + payload.length + offset) =
        v.pos' + W64 (tagConsumed + (1 + payload.length + offset)) := by word
    wp_apply wp_readReceiptEventsProjection_map_rejected_after_tag validator c dst old final
      {v with pos' := v.pos' + W64 tagConsumed} xs payload body head dq d count consumed offset childDepth reason
      enc major hd depth fit failure Hpos Htag.2.2 v wire tagConsumed tag hp hbytes rfl
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    isimp only [position] at Hc
    isimp only [errorPosition] at Herr
    iapply HΦ
    isimp only [ReceiptEventsRecordDiagnostic, ite_true, Int.natCast_add, Int.natCast_one]
    iframe Howned Hc Hs Herr
    ipureintro
    dsimp only at Hnext
    rw [position, errorPosition] at Hnext
    refine ⟨?_, Hnext.2.1, ?_, Hnext.2.2.2⟩
    · rw [Hnext.1, Htag.1]
      omega
    · rw [Hnext.2.2.1, Htag.1]
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptEvents_record_body_rejected
