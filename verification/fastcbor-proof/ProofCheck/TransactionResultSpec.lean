import ProofCheck.TransactionDispatchUnique

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Application transaction values distinguish a nil interface from an allocated
    zero record of any concrete variant. -/
inductive TransactionValue where
  | nullish
  | declare (value : DeclareValue)
  | deploy (value : DeployValue)
  | invoke (value : InvokeValue)
  | l1Handler (value : L1HandlerValue)
  | deployAccount (value : DeployAccountValue)

/-- Failed outcomes retain the exact independent dispatch state and its absolute
    increments from the initial cursor. No failed record is published. -/
inductive TransactionResult where
  | success (value : TransactionValue) (rest : List w8)
  | failure (detail : TransactionDispatchResult) (consumed offset extraDepth : Nat)

def transactionResult : TransactionDispatchResult → TransactionResult
  | .nullish rest => .success .nullish rest
  | .tagFailure reason => .failure (.tagFailure reason) 0 0 0
  | .unknown tag consumed rest => .failure (.unknown tag consumed rest) consumed consumed 0
  | .payload tagConsumed (.declare (.success value rest)) => .success (.declare value) rest
  | .payload tagConsumed (.declare (.failure retained consumed offset extraDepth reason)) =>
      .failure (.payload tagConsumed (.declare (.failure retained consumed offset extraDepth reason)))
        (tagConsumed + consumed) (tagConsumed + offset) extraDepth
  | .payload tagConsumed (.deploy (.success value rest)) => .success (.deploy value) rest
  | .payload tagConsumed (.deploy (.failure retained consumed offset entered reason)) =>
      .failure (.payload tagConsumed (.deploy (.failure retained consumed offset entered reason)))
        (tagConsumed + consumed) (tagConsumed + offset) (if entered then 1 else 0)
  | .payload tagConsumed (.invoke (.success value rest)) => .success (.invoke value) rest
  | .payload tagConsumed (.invoke (.failure retained consumed offset extraDepth reason)) =>
      .failure (.payload tagConsumed (.invoke (.failure retained consumed offset extraDepth reason)))
        (tagConsumed + consumed) (tagConsumed + offset) extraDepth
  | .payload tagConsumed (.l1Handler (.success value rest)) => .success (.l1Handler value) rest
  | .payload tagConsumed (.l1Handler (.failure retained consumed offset entered reason)) =>
      .failure (.payload tagConsumed (.l1Handler (.failure retained consumed offset entered reason)))
        (tagConsumed + consumed) (tagConsumed + offset) (if entered then 1 else 0)
  | .payload tagConsumed (.deployAccount (.success value rest)) => .success (.deployAccount value) rest
  | .payload tagConsumed (.deployAccount (.failure retained consumed offset extraDepth reason)) =>
      .failure (.payload tagConsumed (.deployAccount (.failure retained consumed offset extraDepth reason)))
        (tagConsumed + consumed) (tagConsumed + offset) extraDepth

def TransactionSpec (d : Nat) (tags : CBORTransactionTags) (wire : List w8) (result : TransactionResult) : Prop :=
  ∃ raw, TransactionDispatchSpec d tags wire raw ∧ transactionResult raw = result

theorem transactionSpec_unique (d : Nat) (tags : CBORTransactionTags) (wire : List w8)
    (distinct : TransactionTagsDistinct tags) (a b : TransactionResult)
    (first : TransactionSpec d tags wire a) (second : TransactionSpec d tags wire b) : a = b := by
  obtain ⟨raw, spec, rfl⟩ := first
  obtain ⟨raw2, spec2, rfl⟩ := second
  obtain rfl := transactionDispatchSpec_unique d tags wire distinct raw raw2 spec spec2
  rfl

theorem transactionSpec_total_unique (d : Nat) (tags : CBORTransactionTags) (wire : List w8)
    (depth : d ≤ 32) (distinct : TransactionTagsDistinct tags) :
    ∃ result, TransactionSpec d tags wire result ∧ ∀ other, TransactionSpec d tags wire other → other = result := by
  obtain ⟨raw, spec⟩ := transactionDispatch_classify d tags wire depth
  refine ⟨transactionResult raw, ⟨raw, spec, rfl⟩, ?_⟩
  intro other otherSpec
  exact transactionSpec_unique d tags wire distinct other _ otherSpec ⟨raw, spec, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionSpec_total_unique
