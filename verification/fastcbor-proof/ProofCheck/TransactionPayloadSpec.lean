import ProofCheck.TransactionTagProfile
import ProofCheck.DeclareRecordUnique
import ProofCheck.DeployRecordUnique
import ProofCheck.InvokeRecordUnique
import ProofCheck.L1HandlerRecordUnique
import ProofCheck.DeployAccountRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Each tagged payload retains its native independent success or complete
    partial-failure outcome, including the exact record and depth state. -/
inductive TransactionPayloadResult where
  | declare (result : DeclareRecordResult)
  | deploy (result : DeployRecordResult)
  | invoke (result : InvokeRecordResult)
  | l1Handler (result : L1HandlerRecordResult)
  | deployAccount (result : DeployAccountRecordResult)

def transactionPayloadVariant : TransactionPayloadResult → TransactionVariant
  | .declare _ => .declare
  | .deploy _ => .deploy
  | .invoke _ => .invoke
  | .l1Handler _ => .l1Handler
  | .deployAccount _ => .deployAccount

def TransactionPayloadSpec (d : Nat) (wire : List w8) : TransactionPayloadResult → Prop
  | .declare result => DeclareRecordSpec d declareZero wire result
  | .deploy result => DeployRecordSpec d deployZero wire result
  | .invoke result => InvokeRecordSpec d invokeZero wire result
  | .l1Handler result => L1HandlerRecordSpec d l1HandlerZero wire result
  | .deployAccount result => DeployAccountRecordSpec d deployAccountZero wire result

theorem transactionPayload_classify (d : Nat) (wire : List w8) (variant : TransactionVariant) (depth : d ≤ 32) :
    ∃ result, transactionPayloadVariant result = variant ∧ TransactionPayloadSpec d wire result := by
  cases variant with
  | declare =>
    obtain ⟨result, spec, _⟩ := declareRecordSpec_total_unique d declareZero wire depth
    exact ⟨.declare result, rfl, spec⟩
  | deploy =>
    obtain ⟨result, spec, _⟩ := deployRecordSpec_total_unique d deployZero wire depth
    exact ⟨.deploy result, rfl, spec⟩
  | invoke =>
    obtain ⟨result, spec, _⟩ := invokeRecordSpec_total_unique d invokeZero wire depth
    exact ⟨.invoke result, rfl, spec⟩
  | l1Handler =>
    obtain ⟨result, spec, _⟩ := l1HandlerRecordSpec_total_unique d l1HandlerZero wire depth
    exact ⟨.l1Handler result, rfl, spec⟩
  | deployAccount =>
    obtain ⟨result, spec, _⟩ := deployAccountRecordSpec_total_unique d deployAccountZero wire depth
    exact ⟨.deployAccount result, rfl, spec⟩

theorem transactionPayloadSpec_unique (d : Nat) (wire : List w8) (a b : TransactionPayloadResult)
    (variant : transactionPayloadVariant a = transactionPayloadVariant b)
    (first : TransactionPayloadSpec d wire a) (second : TransactionPayloadSpec d wire b) : a = b := by
  cases a <;> cases b <;> simp only [transactionPayloadVariant] at variant <;> try contradiction
  · exact congrArg TransactionPayloadResult.declare (declareRecordSpec_unique d declareZero wire _ _ first second)
  · exact congrArg TransactionPayloadResult.deploy (deployRecordSpec_unique d deployZero wire _ _ first second)
  · exact congrArg TransactionPayloadResult.invoke (invokeRecordSpec_unique d invokeZero wire _ _ first second)
  · exact congrArg TransactionPayloadResult.l1Handler (l1HandlerRecordSpec_unique d l1HandlerZero wire _ _ first second)
  · exact congrArg TransactionPayloadResult.deployAccount (deployAccountRecordSpec_unique d deployAccountZero wire _ _ first second)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionPayload_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.transactionPayloadSpec_unique
