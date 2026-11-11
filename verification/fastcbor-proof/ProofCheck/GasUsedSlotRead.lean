import ProofCheck.GasUsedSlotSpec
import ProofCheck.GasUsedRecordComplete

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

theorem wp_readGasUsedSlot_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst old slot : Loc) (initial : GasUsedValue) (v : cursor.Cursor) (xs item rest : List w8)
    (value : Option GasUsedValue) (dq : DFrac) (d : Nat)
    (valid : GasUsedSlotValid d initial item rest value) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ GasUsedOwned slot initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadGasConsumedSlot)) (Val #c)) (Val #dst)) (Val #slot))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ GasUsedOwned slot (value.getD initial) ∗
       ⌜out = if value.isSome then slot else null⌝ ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Howned, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases valid with
  | @nullish head suffix hn =>
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ $$ %null
    isimp only [Option.getD_none, Option.isSome_none, Bool.false_eq_true, ite_false, List.length_singleton, Int.natCast_one]
    iframe Hdst Howned Hc Hs
    ipureintro
    exact Hnext
  | record hn hu record =>
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_readGasUsed_record_valid validator c slot initial _ v xs item rest dq d record hd hp hbytes
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ $$ %slot
    isimp only [Option.getD_some, Option.isSome_some, ite_true]
    iframe Hdst Howned Hc Hs
    ipureintro; exact Hstep

/-- Failed parsing retains the supplied partial record and leaves the old
    destination pointer untouched. Storage aliases may observe the partial update. -/
theorem wp_readGasUsedSlot_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst old slot : Loc) (initial : GasUsedValue) (v : cursor.Cursor) (xs wire : List w8) (retained : GasUsedValue)
    (dq : DFrac) (d consumed offset : Nat) (entered : Bool) (reason : GasUsedFailureReason)
    (failure : GasUsedSlotRejected d initial wire retained consumed offset entered reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ GasUsedOwned slot initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (Val (@! cborReadGasConsumedSlot)) (Val #c)) (Val #dst)) (Val #slot))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ GasUsedOwned slot retained ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ GasUsedFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Howned, Hc, Hs⟩
  cases failure with
  | record hn hu failure =>
    wp_auto
    wp_apply wp_null_not_nullish c v xs wire dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_readGasUsed_record_rejected validator c slot initial retained v xs wire dq d consumed offset entered reason
      failure hd hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hdst Howned Hc Hs Herr
    ipureintro; exact Hstep

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasUsedSlot_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasUsedSlot_rejected
