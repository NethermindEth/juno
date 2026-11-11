import ProofCheck.TransactionResultSpec
import ProofCheck.TransactionDispatchBoundary
import ProofCheck.TransactionDeclareDispatch
import ProofCheck.TransactionDeployDispatch
import ProofCheck.TransactionInvokeDispatch
import ProofCheck.TransactionL1HandlerDispatch
import ProofCheck.TransactionDeployAccountDispatch

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

def TransactionValueOwned (out : GoInterface) : TransactionValue → IProp GF
  | .nullish => iprop(⌜out = interface.nil⌝)
  | .declare value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType DeclareTransaction.ty) #ptr⌝ ∗ DeclareOwned ptr value)
  | .deploy value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType DeployTransaction.ty) #ptr⌝ ∗ DeployOwned ptr value)
  | .invoke value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType InvokeTransaction.ty) #ptr⌝ ∗ InvokeOwned ptr value)
  | .l1Handler value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType L1HandlerTransaction.ty) #ptr⌝ ∗ L1HandlerOwned ptr value)
  | .deployAccount value => iprop(∃ ptr : Loc,
      ⌜out = interface.mkOk (go.PointerType DeployAccountTransaction.ty) #ptr⌝ ∗ DeployAccountOwned ptr value)

/-- Failed record allocations remain owned and private. Error positions refer
    to actual diagnostics; bare sentinel branches intentionally ignore them. -/
def TransactionErrorOwned (result : GoInterface) (pos : w64) : TransactionDispatchResult → IProp GF
  | .tagFailure reason => iprop(∃ (err : Loc) (kinds : FeltErrorKinds),
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ FeltErrorGlobals kinds ∗
      ExpectedFailureError err pos (W8 192) reason (expectedFailureKind reason kinds))
  | .unknown _ _ _ => iprop(∃ kinds : FeltErrorKinds,
      FeltErrorGlobals kinds ∗ ⌜result = kinds.unsupportedKind ∧ result ≠ interface.nil⌝)
  | .payload _ (.declare (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, DeclareOwned temporary retained) ∗
      DeclareFailureOwned result pos reason)
  | .payload _ (.deploy (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, DeployOwned temporary retained) ∗
      ∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ DeployFailureOwned err pos reason)
  | .payload _ (.invoke (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, InvokeOwned temporary retained) ∗
      InvokeFailureOwned result pos reason)
  | .payload _ (.l1Handler (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, L1HandlerOwned temporary retained) ∗
      ∃ err : Loc, ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗ L1HandlerFailureOwned err pos reason)
  | .payload _ (.deployAccount (.failure retained _ _ _ reason)) => iprop(
      (∃ temporary : Loc, DeployAccountOwned temporary retained) ∗
      DeployAccountFailureOwned result pos reason)
  | _ => iprop(False)

theorem transactionErrorOwned_nonNil (result : GoInterface) (pos : w64) (detail : TransactionDispatchResult) :
    TransactionErrorOwned (GF := GF) result pos detail ⊢ ⌜result ≠ interface.nil⌝ := by
  cases detail with
  | nullish => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
  | tagFailure reason =>
    isimp only [TransactionErrorOwned]
    iintro ⟨%err, %kinds, %Hresult, _⟩
    ipureintro; rw [Hresult]; simp
  | unknown =>
    isimp only [TransactionErrorOwned]
    iintro ⟨%kinds, _, %Hresult⟩
    ipureintro; exact Hresult.2
  | payload tagConsumed value =>
    cases value with
    | declare value =>
      cases value with
      | success => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [TransactionErrorOwned]
        iintro ⟨_, Herr⟩
        iapply declareFailureOwned_nonNil result pos reason
        iexact Herr
    | deploy value =>
      cases value with
      | success => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [TransactionErrorOwned]
        iintro ⟨_, %err, %Hresult, _⟩
        ipureintro; rw [Hresult]; simp
    | invoke value =>
      cases value with
      | success => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [TransactionErrorOwned]
        iintro ⟨_, Herr⟩
        iapply invokeFailureOwned_nonNil result pos reason
        iexact Herr
    | l1Handler value =>
      cases value with
      | success => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [TransactionErrorOwned]
        iintro ⟨_, %err, %Hresult, _⟩
        ipureintro; rw [Hresult]; simp
    | deployAccount value =>
      cases value with
      | success => isimp only [TransactionErrorOwned]; iintro %H; exact False.elim H
      | failure retained consumed offset depth reason =>
        isimp only [TransactionErrorOwned]
        iintro ⟨_, Herr⟩
        iapply deployAccountFailureOwned_nonNil result pos reason
        iexact Herr

/-- Caller-facing state for the complete decoder: exact value and suffix on
    success, or original interface plus private partial state and exact error on
    failure. Fractional ownership of the input is returned separately. -/
def TransactionReadOwned (c dst : Loc) (old : GoInterface) (v : cursor.Cursor) (xs wire : List w8)
    (result : GoInterface) : TransactionResult → IProp GF
  | .success value rest => iprop(∃ (out : GoInterface) (consumed : Nat),
      dst ↦ out ∗ TransactionValueOwned out value ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure detail consumed offset extraDepth => iprop(dst ↦ old ∗
      c ↦ {v with pos' := v.pos' + W64 consumed, depth' := v.depth' + W64 extraDepth} ∗
      TransactionErrorOwned result (v.pos' + W64 offset) detail ∗
      ⌜result ≠ interface.nil ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z (v.pos' + W64 offset) ∧
        sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionErrorOwned_nonNil
