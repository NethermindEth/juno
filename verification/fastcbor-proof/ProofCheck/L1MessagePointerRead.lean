import ProofCheck.L1MessagePointerSpec
import ProofCheck.L1MessageRecordComplete

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

def L1MessagePointerOwned (p : Loc) : Option L1MessageValue → IProp GF
  | none => iprop(⌜p = null⌝)
  | some value => L1MessageOwned p value

theorem l1MessagePointerOwned_zero (p : Loc) :
    p ↦ zero_val L1ToL2Message ⊢ (L1MessagePointerOwned p (some l1MessageZero) : IProp GF) := by
  iintro Hp
  unfold L1MessagePointerOwned L1MessageOwned
  iexists zero_val L1ToL2Message
  iframe Hp
  iapply l1MessageChildren_zero

/-- Actual successful allocation/publication, or nullish pointer clearing.
    No existing record is mutated; a nonnil result starts from a fresh zero record. -/
theorem wp_readL1MessagePointer_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst old : Loc) (v : cursor.Cursor) (xs item rest : List w8)
    (value : Option L1MessageValue) (dq : DFrac) (d : Nat)
    (valid : L1MessagePointerValid d item rest value) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadL1ToL2MessagePointer)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ L1MessagePointerOwned out value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases valid with
  | @nullish head suffix hn =>
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ $$ %null
    isimp only [L1MessagePointerOwned, List.length_singleton, Int.natCast_one]
    iframe Hdst Hc Hs
    ipureintro
    exact Hnext
  | record hn hu record =>
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    ihave Howned := l1MessagePointerOwned_zero «$r0_ptr» $$ «$r0»
    isimp only [L1MessagePointerOwned] at Howned
    wp_apply wp_readL1Message_record_valid validator c «$r0_ptr» l1MessageZero _ v xs item rest dq d record hd hp hbytes
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ $$ %«$r0_ptr»
    isimp only [L1MessagePointerOwned]
    iframe Hdst Howned Hc Hs
    ipureintro; exact Hstep

/-- A failed fresh record remains private. The caller's original pointer is
    preserved, with exact owned partial state, diagnostic and cursor/depth. -/
theorem wp_readL1MessagePointer_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8) (retained : L1MessageValue)
    (dq : DFrac) (d consumed offset : Nat) (entered : Bool) (reason : L1MessageFailureReason)
    (failure : L1MessagePointerRejected d wire retained consumed offset entered reason)
    (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadL1ToL2MessagePointer)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ (∃ temporary : Loc, L1MessageOwned temporary retained) ∗
       c ↦ {v with pos' := v.pos' + W64 consumed, depth' := if entered then v.depth' + W64 1 else v.depth'} ∗
       v.buf' ↦*{dq} xs ∗ L1MessageFailureOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  cases failure with
  | record hn hu failure =>
    wp_auto
    wp_apply wp_null_not_nullish c v xs wire dq hn hu hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    ihave Howned := l1MessagePointerOwned_zero «$r0_ptr» $$ «$r0»
    isimp only [L1MessagePointerOwned] at Howned
    wp_apply wp_readL1Message_record_rejected validator c «$r0_ptr» l1MessageZero retained v xs wire dq d consumed offset entered reason
      failure hd hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hstep⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hdst Hc Hs Herr
    isplitl [Howned]
    · iexists «$r0_ptr»; iexact Howned
    ipureintro; exact Hstep

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessagePointerOwned_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readL1MessagePointer_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readL1MessagePointer_rejected
