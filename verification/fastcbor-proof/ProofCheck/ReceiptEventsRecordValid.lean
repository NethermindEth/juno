import ProofCheck.ReceiptEventsMap
import ProofCheck.ReceiptEventsTagBoundary
import ProofCheck.ReceiptEventsRecord

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

/-- Every independently valid projection record, including an optional allowed
    tag and arbitrary unread suffix, is decoded by the actual generated reader.
    The result owns all published fields and restores the enclosing depth. -/
theorem wp_readReceiptEvents_record_valid (validator : Utf8ValidatorContract (GF := GF))
    (d : Nat) (c dst : Loc) (old final : ReceiptEventsValue)
    (v : cursor.Cursor) (xs wire body rest : List w8) (dq : DFrac) (tagConsumed : Nat)
    (tag : ProjectionTagSpec wire tagConsumed (body ++ rest) .success)
    (valid : ReceiptEventsBodyValid d old body rest final) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       ReceiptEventsOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadReceiptEventsProjection)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; ReceiptEventsOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (tagConsumed + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (tagConsumed + body.length)) = sint.Z v.pos' + tagConsumed + body.length ∧
         sint.Z (v.pos' + W64 (tagConsumed + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (tagConsumed + body.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  obtain ⟨tagBytes, splitTag, lengthTag⟩ := projectionTag_suffix wire (body ++ rest) tagConsumed .success tag
  have Htag := input_consumption xs tagBytes (body ++ rest) v.pos' v.buf'.len hp Hlen.1 (by rw [← splitTag]; exact hbytes)
  rw [lengthTag] at Htag
  have Hpos : 0 ≤ sint.Z (v.pos' + W64 tagConsumed) ∧ sint.Z (v.pos' + W64 tagConsumed) ≤ sint.Z v.buf'.len :=
    ⟨by rw [Htag.1]; omega, Htag.2.1⟩
  cases valid with
  | @nullish head suffix nullish =>
    ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
    · solve_pkg_init
    icases Hglobals with ⟨%kinds, Hglobals⟩
    wp_apply wp_readReceiptEvents_nullish_after_tag c dst old
      {v with pos' := v.pos' + W64 tagConsumed} xs rest head dq nullish Hpos Htag.2.2
      v wire tagConsumed kinds tag hp hbytes rfl $$ [Hglobals Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe
    have position : v.pos' + W64 tagConsumed + W64 1 = v.pos' + W64 (tagConsumed + [head].length) := by simp; word
    isimp only [position] at Hc
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro
    dsimp only at Hnext
    rw [position] at Hnext
    simp only [List.length_singleton, Int.natCast_one] at Hnext ⊢
    exact ⟨by rw [Hnext.1, Htag.1], Hnext.2⟩
  | @map head payload body suffix argument final count enc major length depth valid =>
    have argumentEq : argument = W64 count := by word
    subst argument
    have position : v.pos' + W64 tagConsumed + W64 (1 + payload.length + body.length) =
        v.pos' + W64 (tagConsumed + ((head :: payload) ++ body).length) := by simp; word
    wp_apply wp_readReceiptEventsProjection_map_valid_after_tag validator c dst old final
      {v with pos' := v.pos' + W64 tagConsumed} xs payload body rest head dq d count enc major hd depth valid Hpos Htag.2.2
      v wire tagConsumed tag hp hbytes rfl $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe
    isimp only [position] at Hc
    iapply HΦ
    iframe Howned Hc Hs
    ipureintro
    dsimp only at Hnext
    rw [position] at Hnext
    refine ⟨?_, Hnext.2⟩
    rw [Hnext.1, Htag.1]
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptEvents_record_valid
