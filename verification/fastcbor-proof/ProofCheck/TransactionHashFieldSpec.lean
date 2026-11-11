import ProofCheck.TransactionHashFieldRead
import ProofCheck.TransactionHashValues

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

def transactionHashFieldReader : TransactionHashField → GoString
  | .hash => cborReadFeltValue

def transactionHashFieldRef (field : TransactionHashField) (dst : Loc) : Loc :=
  structFieldRef transactionHashProjection (transactionHashFieldName field) dst

def transactionHashFieldFailureAddr : TransactionHashFieldFailure → Loc
  | .felt reason => feltFailureAddr (GF := GF) reason

def transactionHashFieldFailureKind (reason : TransactionHashFieldFailure) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .felt reason => feltFailureKind reason kinds

def TransactionHashFieldError (err : Loc) (pos : w64) (reason : TransactionHashFieldFailure) (category : GoInterface) : IProp GF :=
  match reason with
  | .felt reason => FeltFailureError err pos reason category

theorem transactionHashFieldErrorGlobals_lookup (kinds : FeltErrorKinds) (reason : TransactionHashFieldFailure) :
    FeltErrorGlobals kinds ⊢ (transactionHashFieldFailureAddr (GF := GF) reason ↦□ transactionHashFieldFailureKind reason kinds : IProp GF) := by
  cases reason with
  | felt reason => exact feltErrorGlobals_lookup kinds reason

/-- The actual field helper realizes the independent selected-field relation. -/
theorem wp_transactionHashField_valid_spec (d : Nat) (field : TransactionHashField)
    (c dst : Loc) (old final : TransactionHashValue) (v : cursor.Cursor) (xs wire rem : List w8) (dq : DFrac)
    (member : field ∈ transactionHashFields) (valid : TransactionHashFieldValid d field old wire final)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (transactionHashFieldReader field))) (Val #c))
        (Val #(transactionHashFieldRef (GF := GF) field dst)))
    {{ RET #interface.nil; TransactionHashOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  cases valid with
  | hash valid =>
    simpa only [transactionHashFieldReader, transactionHashFieldRef, transactionHashFieldName] using
      wp_transactionHashField_valid (GF := GF) c dst old final v xs wire rem dq d valid hd depth hp hbytes

/-- Selected-field failures restore the old hash and full cursor, with the
    exact owned diagnostic required by the independent record specification. -/
theorem wp_transactionHashField_rejected_spec (d offset : Nat) (field : TransactionHashField)
    (c dst : Loc) (old : TransactionHashValue) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : TransactionHashFieldFailure) (category : GoInterface)
    (member : field ∈ transactionHashFields) (failure : TransactionHashFieldRejected d field wire offset reason)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       transactionHashFieldFailureAddr (GF := GF) reason ↦□ category ∗
       TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! (transactionHashFieldReader field))) (Val #c))
        (Val #(transactionHashFieldRef (GF := GF) field dst)))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       TransactionHashOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       TransactionHashFieldError err (v.pos' + W64 offset) reason category ∗
       ⌜sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧ sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  cases failure with
  | hash failure =>
    simpa only [transactionHashFieldReader, transactionHashFieldRef, transactionHashFieldName,
      transactionHashFieldFailureAddr, TransactionHashFieldError] using
      wp_transactionHashField_rejected (GF := GF) c dst old v xs wire dq d offset _ category failure hd depth hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionHashFieldErrorGlobals_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transactionHashField_valid_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_transactionHashField_rejected_spec
