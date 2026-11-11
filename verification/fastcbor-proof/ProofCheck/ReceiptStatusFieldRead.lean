import ProofCheck.ReceiptStatusOwnership
import ProofCheck.BoolComplete
import ProofCheck.StringComplete

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

def receiptStatusFieldReader : ReceiptStatusField → GoString
  | .reverted => cborReadBool
  | .reason => cborReadString

def receiptStatusFieldRef (field : ReceiptStatusField) (dst : Loc) : Loc :=
  structFieldRef receiptExecutionStatusProjection (receiptStatusFieldName field) dst

def receiptStatusFieldFailureAddr : ReceiptStatusFieldFailure → Loc
  | .boolean reason => expectedFailureAddr (GF := GF) (boolExpectedReason reason)
  | .string reason => stringFailureAddr (GF := GF) reason

def receiptStatusFieldFailureKind (reason : ReceiptStatusFieldFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .boolean reason => expectedFailureKind (boolExpectedReason reason) kinds
  | .string reason => stringFailureKind reason kinds

def ReceiptStatusFieldError (err : Loc) (pos : w64) (reason : ReceiptStatusFieldFailure)
    (kind : GoInterface) : IProp GF :=
  match reason with
  | .boolean reason => BoolFailureError err pos reason kind
  | .string reason => StringFailureError err pos reason kind

theorem receiptStatusFieldErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : ReceiptStatusFieldFailure) :
    FeltErrorGlobals kinds ⊢ (receiptStatusFieldFailureAddr (GF := GF) reason ↦□
      receiptStatusFieldFailureKind reason kinds : IProp GF) := by
  iintro Hglobals
  cases reason with
  | boolean reason =>
    ihave Hcategory := feltErrorGlobals_lookup kinds (.expected (W8 0) (boolExpectedReason reason)) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hcategory
    isimp only [receiptStatusFieldFailureAddr, receiptStatusFieldFailureKind]
    iexact Hcategory
  | string reason =>
    isimp only [receiptStatusFieldFailureAddr, receiptStatusFieldFailureKind]
    iapply stringErrorGlobals_lookup kinds reason
    iexact Hglobals

/-- Both real selected-field calls preserve the other field and implement the
    independent relation. Only the string branch uses the validator premise. -/
theorem wp_receiptStatusField_valid (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptStatusField) (c dst : Loc) (old final : ReceiptStatusValue)
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac)
    (valid : ReceiptStatusFieldValid field old item final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptStatusFieldReader field))) (Val #c))
        (Val #(receiptStatusFieldRef (GF := GF) field dst)))
    {{ RET #interface.nil; ReceiptStatusOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  cases valid with
  | reverted valid =>
    rename_i value
    simp only [receiptStatusFieldReader, receiptStatusFieldRef, receiptStatusFieldName]
    wp_start_folded as ⟨Howned, Hc, Hs⟩
    isimp only [ReceiptStatusOwned] at Howned
    icases receiptStatusReverted_acc dst (receiptStatusLayout old) $$ Howned with ⟨Hfield, Hclose⟩
    isimp only [receiptStatusLayout] at Hfield
    wp_apply wp_readBool_valid c (structFieldRef receiptExecutionStatusProjection go!"Reverted" dst)
      old.reverted v xs item rest value dq valid hp hbytes $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    isimp only [receiptStatusLayout] at Howned
    iapply HΦ
    isimp only [ReceiptStatusOwned, receiptStatusLayout]
    iframe Howned Hc Hs
    ipureintro; exact Hnext
  | reason valid =>
    rename_i value
    simp only [receiptStatusFieldReader, receiptStatusFieldRef, receiptStatusFieldName]
    wp_start_folded as ⟨Howned, Hc, Hs⟩
    isimp only [ReceiptStatusOwned] at Howned
    icases receiptStatusReason_acc dst (receiptStatusLayout old) $$ Howned with ⟨Hfield, Hclose⟩
    isimp only [receiptStatusLayout] at Hfield
    wp_apply wp_readString_valid validator c (structFieldRef receiptExecutionStatusProjection go!"RevertReason" dst)
      old.reason v xs item rest value dq valid hp hbytes $$ [Hfield Hc Hs] with ⟨Hfield, Hc, Hs, %Hnext⟩
    · iframe Hfield Hc Hs
    ihave Howned := Hclose $$ %value Hfield
    isimp only [receiptStatusLayout] at Howned
    iapply HΦ
    isimp only [ReceiptStatusOwned, receiptStatusLayout]
    iframe Howned Hc Hs
    ipureintro; exact Hnext

/-- Field rejection preserves the whole projection and cursor, while retaining
    the actual diagnostic. Both field families report offset zero. -/
theorem wp_receiptStatusField_rejected (validator : Utf8ValidatorContract (GF := GF))
    (field : ReceiptStatusField) (c dst : Loc) (old : ReceiptStatusValue)
    (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac) (offset : Nat)
    (reason : ReceiptStatusFieldFailure) (kind : GoInterface)
    (rejected : ReceiptStatusFieldRejected field wire offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       receiptStatusFieldFailureAddr (GF := GF) reason ↦□ kind ∗
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (receiptStatusFieldReader field))) (Val #c))
        (Val #(receiptStatusFieldRef (GF := GF) field dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       ReceiptStatusOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       ReceiptStatusFieldError err (v.pos' + W64 offset) reason kind }} := by
  cases rejected with
  | reverted failure =>
    simp only [receiptStatusFieldReader, receiptStatusFieldRef, receiptStatusFieldName]
    wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
    isimp only [ReceiptStatusOwned] at Howned
    isimp only [receiptStatusFieldFailureAddr] at Hcategory
    icases receiptStatusReverted_acc dst (receiptStatusLayout old) $$ Howned with ⟨Hfield, Hclose⟩
    isimp only [receiptStatusLayout] at Hfield
    wp_apply wp_readBool_rejected c (structFieldRef receiptExecutionStatusProjection go!"Reverted" dst)
      old.reverted v xs wire dq _ kind failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hcategory Hfield Hc Hs
    ihave Howned := Hclose $$ %old.reverted Hfield
    isimp only [receiptStatusLayout] at Howned
    iapply HΦ
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [ReceiptStatusOwned, receiptStatusLayout, ReceiptStatusFieldError, Int.natCast_zero, hz]
    iframe Howned Hc Hs Herr
  | reason failure =>
    simp only [receiptStatusFieldReader, receiptStatusFieldRef, receiptStatusFieldName]
    wp_start_folded as ⟨#Hcategory, Howned, Hc, Hs⟩
    isimp only [ReceiptStatusOwned] at Howned
    isimp only [receiptStatusFieldFailureAddr] at Hcategory
    icases receiptStatusReason_acc dst (receiptStatusLayout old) $$ Howned with ⟨Hfield, Hclose⟩
    isimp only [receiptStatusLayout] at Hfield
    wp_apply wp_readString_rejected validator c (structFieldRef receiptExecutionStatusProjection go!"RevertReason" dst)
      old.reason v xs wire dq _ kind failure hp hbytes $$ [Hfield Hc Hs] as %err ⟨Hfield, Hc, Hs, Herr⟩
    · iframe Hcategory Hfield Hc Hs
    ihave Howned := Hclose $$ %old.reason Hfield
    isimp only [receiptStatusLayout] at Howned
    iapply HΦ
    have hz : v.pos' + W64 0 = v.pos' := by word
    isimp only [ReceiptStatusOwned, receiptStatusLayout, ReceiptStatusFieldError, Int.natCast_zero, hz]
    iframe Howned Hc Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.receiptStatusFieldErrorGlobals_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptStatusField_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_receiptStatusField_rejected
