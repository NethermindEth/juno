import ProofCheck.ReceiptStatusFieldRead
import ProofCheck.InitializationErrors

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- The record loop can consume this all-input field contract without assuming
    a successful encoding. Success owns the independent decoded value and exact
    suffix; rejection owns the old value and a diagnostic at the specified offset. -/
def ReceiptStatusFieldOutcome (field : ReceiptStatusField)
    (c dst : Loc) (old : ReceiptStatusValue) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : IProp GF := iprop(
  (∃ (item rest : List w8) (final : ReceiptStatusValue),
    ReceiptStatusOwned dst final ∗ c ↦ {v with pos' := v.pos' + W64 item.length} ∗
    ⌜result = interface.nil ∧ wire = item ++ rest ∧ ReceiptStatusFieldValid field old item final ∧
      sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
      sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝) ∨
  (∃ (offset : Nat) (reason : ReceiptStatusFieldFailure) (err : Loc),
    ReceiptStatusOwned dst old ∗ c ↦ v ∗
    ReceiptStatusFieldError err (v.pos' + W64 offset) reason (receiptStatusFieldFailureKind reason kinds) ∗
    ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
      ReceiptStatusFieldRejected field wire offset reason ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧ sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝))

/-- All actual projection field helpers on arbitrary bytes. Initialized cursor
    state supplies the canonical diagnostic categories. String decoding retains
    the existing explicit UTF-8 validator premise. -/
theorem wp_receiptStatusField_complete (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptStatusField)
    (c dst : Loc) (old : ReceiptStatusValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptStatusFieldReader field))) (Val #c))
        (Val #(receiptStatusFieldRef (GF := GF) field dst)))
    {{ (result : GoInterface), RET #result; v.buf' ↦*{dq} xs ∗ ∃ kinds : FeltErrorKinds,
       FeltErrorGlobals kinds ∗ ReceiptStatusFieldOutcome field c dst old v xs wire kinds result }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
  · iapply header_initialized_felt_globals
    solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  rcases receiptStatusField_classify field old wire with
    ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
  · wp_apply wp_receiptStatusField_valid validator field c dst old final v xs item rest dq valid hp (hbytes.trans split)
      $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hnext⟩
    · iframe Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    isimp only [ReceiptStatusFieldOutcome]
    ileft
    iexists item, rest, final
    iframe Howned Hc
    ipureintro
    exact ⟨trivial, split, valid, Hnext⟩
  · ihave #Hcategory : (receiptStatusFieldFailureAddr (GF := GF) reason ↦□ receiptStatusFieldFailureKind reason kinds : IProp GF) $$ []
    · iapply receiptStatusFieldErrorGlobals_lookup kinds reason
      iexact Hglobals
    wp_apply wp_receiptStatusField_rejected validator field c dst old v xs wire dq offset reason
      (receiptStatusFieldFailureKind reason kinds) failure hp hbytes
      $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr⟩
    · iframe Hcategory Howned Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds
    iframe Hglobals
    isimp only [ReceiptStatusFieldOutcome]
    iright
    iexists offset, reason, err
    iframe Howned Hc Herr
    ipureintro
    have hoff := receiptStatusField_rejected_offset field wire offset reason failure
    subst offset
    refine ⟨rfl, failure, ?_, ?_⟩ <;> word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptStatusField_complete
