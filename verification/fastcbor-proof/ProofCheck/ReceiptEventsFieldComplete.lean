import ProofCheck.ReceiptEventsFieldRead

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

/-- Field outcomes retain the entire projection's ownership and the independent
    contextual value/failure relation, including nested failure consumption. -/
def ReceiptEventsFieldOutcome (d : Nat) (field : ReceiptEventsField)
    (c dst : Loc) (old : ReceiptEventsValue) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : ReceiptEventsValue),
    ReceiptEventsOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ ReceiptEventsFieldValid d field old item rest final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (consumed offset extraDepth : Nat) (reason : ReceiptEventsFieldFailure) (err : Loc),
    ReceiptEventsOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
    ReceiptEventsFieldErrorOwned err (v.pos' + W64 offset) reason ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason ∧
      sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝))

/-- Both actual generated selected-field calls cover every byte input and
    arbitrary prior owned projection. No successful encoding is assumed. -/
theorem wp_receiptEventsField_complete (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptEventsField) (c dst : Loc) (old : ReceiptEventsValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptEventsOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptEventsFieldReader field))) (Val #c))
        (Val #(receiptEventsFieldRef (GF := GF) field dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗
       ReceiptEventsFieldOutcome d field c dst old v xs wire result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  rcases receiptEventsField_classify d field old wire depth with
    ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
  · wp_apply wp_receiptEventsField_valid validator field c dst old final v xs item rest dq d
      valid hd depth hp (hbytes.trans split) $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    unfold ReceiptEventsFieldOutcome
    ileft
    iexists item, rest, final
    iframe Howned Hc
    ipureintro; exact ⟨rfl, split, valid, Hnext⟩
  · wp_apply wp_receiptEventsField_rejected validator field c dst old v xs wire dq d consumed offset extraDepth reason
      failure hd depth hp hbytes $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    unfold ReceiptEventsFieldOutcome
    iright
    iexists consumed, offset, extraDepth, reason, err
    iframe Howned Hc Herr
    ipureintro; exact ⟨rfl, failure, Hnext⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptEventsField_complete
