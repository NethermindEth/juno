import ProofCheck.ReceiptEventsOwnership

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

def receiptEventsFieldReader : ReceiptEventsField → GoString
  | .events => cborReadEventPointers
  | .hash => cborReadFelt

def receiptEventsFieldRef (field : ReceiptEventsField) (dst : Loc) : Loc :=
  structFieldRef receiptEventsProjection (receiptEventsFieldName field) dst

def ReceiptEventsFieldErrorOwned (err : Loc) (pos : w64) : ReceiptEventsFieldFailure → IProp GF
  | .events reason => EventPointersFailureOwned err pos reason
  | .felt reason => iprop(∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
      FeltFailureError err pos reason (feltFailureKind reason kinds))

/-- Actual selected-field calls publish only their own successful result,
    preserving the other field's cell and all of its child ownership. -/
theorem wp_receiptEventsField_valid (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptEventsField) (c dst : Loc) (old final : ReceiptEventsValue)
    (v : cursor.Cursor) (xs item rem : List w8) (dq : DFrac) (d : Nat)
    (valid : ReceiptEventsFieldValid d field old item rem final) (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptEventsOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptEventsFieldReader field))) (Val #c))
        (Val #(receiptEventsFieldRef (GF := GF) field dst)))
    {{ RET #interface.nil; ReceiptEventsOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [ReceiptEventsOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  cases valid with
  | events valid =>
    simp only [receiptEventsFieldReader, receiptEventsFieldRef, receiptEventsFieldName]
    icases receiptEventsSlice_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
    wp_apply wp_readEventPointers_valid validator c (structFieldRef receiptEventsProjection go!"Events" dst) h.Events'
      v xs (item ++ rem) rem _ dq d valid hd hp hbytes $$ [Hfield Hc Hs] as %out ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := receiptEventsChildren_events h old out _ $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    have hlength : (item ++ rem).length - rem.length = item.length := by simp
    isimp only [hlength] at Hc
    simp only [hlength] at Hnext
    iapply HΦ
    iframe Hc Hs
    isplitl [Hh Hchildren]
    · unfold ReceiptEventsOwned
      iexists {h with Events' := out}
      iframe Hh Hchildren
    · ipureintro; exact Hnext
  | hash valid =>
    simp only [receiptEventsFieldReader, receiptEventsFieldRef, receiptEventsFieldName]
    icases receiptEventsHash_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_valid c (structFieldRef receiptEventsProjection go!"TransactionHash" dst) h.TransactionHash'
      v xs item rem _ dq d hd depth valid hp hbytes $$ [Hfield Hc Hs] as %out ⟨Hfield, Hvalue, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hh := Hclose $$ %out Hfield
    ihave Hchildren := receiptEventsChildren_hash h old out _ $$ [Hchildren Hvalue]
    · iframe Hchildren Hvalue
    iapply HΦ
    iframe Hc Hs
    isplitl [Hh Hchildren]
    · unfold ReceiptEventsOwned
      iexists {h with TransactionHash' := out}
      iframe Hh Hchildren
    · ipureintro; exact Hnext

/-- Selected-field rejection preserves both owned application fields. Events
    retain their consumed input and nesting; the hash helper restores both. -/
theorem wp_receiptEventsField_rejected (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptEventsField) (c dst : Loc) (old : ReceiptEventsValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (d consumed offset extraDepth : Nat)
    (reason : ReceiptEventsFieldFailure)
    (failure : ReceiptEventsFieldRejected d field wire consumed offset extraDepth reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborreceipts ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptEventsOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptEventsFieldReader field))) (Val #c))
        (Val #(receiptEventsFieldRef (GF := GF) field dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ReceiptEventsOwned dst old ∗ c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ ReceiptEventsFieldErrorOwned err (v.pos' + W64 offset) reason ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.depth' + W64 extraDepth) = d + extraDepth ∧ d + extraDepth ≤ 32⌝ }} := by
  wp_start_folded as ⟨Howned, Hc, Hs⟩
  isimp only [ReceiptEventsOwned] at Howned
  icases Howned with ⟨%h, Hh, Hchildren⟩
  cases failure with
  | events failure =>
    simp only [receiptEventsFieldReader, receiptEventsFieldRef, receiptEventsFieldName]
    icases receiptEventsSlice_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
    wp_apply wp_readEventPointers_rejected validator c (structFieldRef receiptEventsProjection go!"Events" dst) h.Events'
      v xs wire dq d consumed offset extraDepth _ failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Hh := Hclose $$ %h.Events' Hfield
    iapply HΦ
    isimp only [ReceiptEventsFieldErrorOwned]
    iframe Hc Hs Herr
    isplitl [Hh Hchildren]
    · unfold ReceiptEventsOwned
      iexists h
      iframe Hh Hchildren
    · ipureintro; exact Hnext
  | hash failure =>
    simp only [receiptEventsFieldReader, receiptEventsFieldRef, receiptEventsFieldName]
    ihave Hglobals : (∃ kinds : FeltErrorKinds, FeltErrorGlobals (GF := GF) kinds : IProp GF) $$ []
    · iapply cursor_initialized_felt_globals
      solve_pkg_init
    icases Hglobals with ⟨%kinds, #Hglobals⟩
    ihave #Hkind := feltErrorGlobals_lookup kinds _ $$ Hglobals
    icases receiptEventsHash_acc dst h $$ Hh with ⟨Hfield, Hclose⟩
    wp_apply wp_readFelt_failure_bounded c (structFieldRef receiptEventsProjection go!"TransactionHash" dst) h.TransactionHash'
      v xs wire dq d offset _ _ failure hd depth hp hbytes
      $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr, %Hnext⟩
    · iframe Hkind Hfield Hc Hs
    ihave Hh := Hclose $$ %h.TransactionHash' Hfield
    have hz (x : w64) : x + W64 0 = x := by word
    isimp only [Int.natCast_zero, Int.add_zero, hz] at HΦ
    iapply HΦ
    iframe Hc Hs
    isplitl [Hh Hchildren]
    · unfold ReceiptEventsOwned
      iexists h
      iframe Hh Hchildren
    isplitl [Herr]
    · unfold ReceiptEventsFieldErrorOwned
      iexists kinds
      iframe Hglobals Herr
    · ipureintro
      exact ⟨trivial, Hnext.1, by rw [Hnext.1]; omega, Hnext.2, hd, depth⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptEventsField_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptEventsField_rejected
