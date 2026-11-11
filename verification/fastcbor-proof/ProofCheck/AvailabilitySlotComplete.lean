import ProofCheck.AvailabilitySlotRead

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

def AvailabilitySlotReadOwned (c dst old slot : Loc) (initial : AvailabilityValue) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : AvailabilitySlotOutcome → IProp GF
  | .success value rest => iprop(∃ (out : Loc) (consumed : Nat),
      dst ↦ out ∗ AvailabilityOwned slot (value.getD initial) ∗
      ⌜out = if value.isSome then slot else null⌝ ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure retained consumed offset entered reason => iprop(∃ (err : Loc),
      dst ↦ old ∗ AvailabilityOwned slot retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
      AvailabilityFailureOwned err (v.pos' + W64 offset) reason ∗
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every input, old pointer and initial supplied storage has an independently
    unique result. Nullish clears only the pointer; successful records publish the
    slot; failures retain its partial value without changing the pointer cell.
    Key decoding retains the shared UTF-8 premise. -/
theorem wp_readAvailabilitySlot_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst old slot : Loc) (initial : AvailabilityValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ AvailabilityOwned slot initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadDataAvailabilitySlot)) (Val #c)) (Val #dst)) (Val #slot))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : AvailabilitySlotOutcome,
       ⌜AvailabilitySlotSpec d initial wire outcome ∧ ∀ other, AvailabilitySlotSpec d initial wire other → other = outcome⌝ ∗
       AvailabilitySlotReadOwned c dst old slot initial v xs wire result outcome }} := by
  wp_start_folded as ⟨Hdst, Howned, Hc, Hs⟩
  obtain ⟨outcome, spec, unique⟩ := availabilitySlotSpec_total_unique d initial wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readAvailabilitySlot_valid validator c dst old slot initial v xs item rest value dq d valid hd hp (hbytes.trans split)
      $$ [Hdst Howned Hc Hs] as %out ⟨Hdst, Howned, %Hout, Hc, Hs, %Hnext⟩
    · iframe Hdst Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists AvailabilitySlotOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [AvailabilitySlotReadOwned]
    iexists out, item.length
    iframe Hdst Howned Hc
    ipureintro; exact ⟨Hout, trivial, by rw [split, List.length_append], Hnext⟩
  | failure retained consumed offset entered reason =>
    wp_apply wp_readAvailabilitySlot_rejected validator c dst old slot initial v xs wire retained dq d consumed offset entered reason
      spec hd hp hbytes $$ [Hdst Howned Hc Hs] as %err ⟨Hdst, Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Hdst Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists AvailabilitySlotOutcome.failure retained consumed offset entered reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [AvailabilitySlotReadOwned]
    iexists err
    iframe Hdst Howned Hc Herr
    ipureintro; exact ⟨rfl, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readAvailabilitySlot_complete
