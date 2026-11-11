import ProofCheck.TransactionState

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
/-- The five application-level tagged transaction alternatives. -/
inductive TransactionVariant where
  | declare | deploy | invoke | l1Handler | deployAccount
  deriving DecidableEq

def transactionVariantTag (tags : CBORTransactionTags) : TransactionVariant → w64
  | .declare => tags.DeclareTransaction'
  | .deploy => tags.DeployTransaction'
  | .invoke => tags.InvokeTransaction'
  | .l1Handler => tags.L1HandlerTransaction'
  | .deployAccount => tags.DeployAccountTransaction'

/-- The direct API's documented caller requirement; registration validates it
    outside this decoder proof boundary. -/
def TransactionTagsDistinct (tags : CBORTransactionTags) : Prop :=
  ∀ a b : TransactionVariant, a ≠ b → transactionVariantTag tags a ≠ transactionVariantTag tags b

def TransactionTagMeaning (tags : CBORTransactionTags) (tag : w64) : Option TransactionVariant → Prop
  | some variant => tag = transactionVariantTag tags variant
  | none => ∀ variant, tag ≠ transactionVariantTag tags variant

theorem transactionVariantTag_injective (tags : CBORTransactionTags) (distinct : TransactionTagsDistinct tags)
    (a b : TransactionVariant) (same : transactionVariantTag tags a = transactionVariantTag tags b) : a = b := by
  by_cases eq : a = b
  · exact eq
  · exact False.elim (distinct a b eq same)

theorem transactionTagMeaning_total (tags : CBORTransactionTags) (tag : w64) :
    ∃ choice, TransactionTagMeaning tags tag choice := by
  classical
  by_cases known : ∃ variant, tag = transactionVariantTag tags variant
  · obtain ⟨variant, found⟩ := known
    exact ⟨some variant, found⟩
  · exact ⟨none, fun variant same => known ⟨variant, same⟩⟩

theorem transactionTagMeaning_unique (tags : CBORTransactionTags) (tag : w64)
    (distinct : TransactionTagsDistinct tags) (a b : Option TransactionVariant)
    (first : TransactionTagMeaning tags tag a) (second : TransactionTagMeaning tags tag b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some variant => exact False.elim (first variant second)
  | some variant => cases b with
    | none => exact False.elim (second variant first)
    | some other => exact congrArg some (transactionVariantTag_injective tags distinct variant other (first.symm.trans second))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionVariantTag_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionTagMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionTagMeaning_unique
