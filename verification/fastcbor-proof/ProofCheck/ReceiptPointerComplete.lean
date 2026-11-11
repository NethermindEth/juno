import ProofCheck.ReceiptPointerRead

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

def ReceiptPointerReadOwned (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : ReceiptPointerOutcome → IProp GF
  | .success value rest => iprop(∃ (out : Loc) (consumed : Nat),
      dst ↦ out ∗ ReceiptPointerOwned out value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure retained consumed offset extraDepth reason => iprop(∃ (temporary : Loc),
      dst ↦ old ∗ ReceiptOwned temporary retained ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      ReceiptFailureOwned result (v.pos' + W64 offset) reason ∗
      ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

/-- Every input and old pointer has the independently unique result. Successful
    allocations are published; failed partial records remain privately owned.
    The only helper premise is the shared UTF-8 contract of key and string decoding. -/
theorem wp_readReceiptPointer_complete (validator : Utf8ValidatorContract (GF := GF))
    (c dst old : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadTransactionReceiptPointer)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ outcome : ReceiptPointerOutcome,
       ⌜ReceiptPointerSpec d wire outcome ∧ ∀ other, ReceiptPointerSpec d wire other → other = outcome⌝ ∗
       ReceiptPointerReadOwned c dst old v xs wire result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  obtain ⟨outcome, spec, unique⟩ := receiptPointerSpec_total_unique d wire depth
  cases outcome with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_readReceiptPointer_valid validator c dst old v xs item rest value dq d valid hd hp (hbytes.trans split)
      $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists ReceiptPointerOutcome.success value rest
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [ReceiptPointerReadOwned]
    iexists out, item.length
    iframe Hdst Howned Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure retained consumed offset extraDepth reason =>
    wp_apply wp_readReceiptPointer_rejected validator c dst old v xs wire retained dq d consumed offset extraDepth reason
      spec hd hp hbytes $$ [Hdst Hc Hs] as %err ⟨Hdst, ⟨%temporary, Howned⟩, Hc, Hs, Herr, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists ReceiptPointerOutcome.failure retained consumed offset extraDepth reason
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [ReceiptPointerReadOwned]
    iexists temporary
    iframe Hdst Howned Hc Herr
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readReceiptPointer_complete
