-- Expected failures on perennial.rev: comparability is modeled, but array-value
-- equality has no step rule. These diagnostics are outside the audited target.
import ProofCheck.GeneratedProof.github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps
import Perennial.Golang.Theory.Map

noncomputable section
namespace Perennial
open github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext] [GoLocalContext]
variable [sem : go.Semantics]
variable [package_sem : github_com.NethermindEth.juno.verification.fastcbor_proof.feasibility.maps.Assumptions]

example (left right : GoArray w64 4) :
    ⟦GoOp GoEquals (go.ArrayType 4 go.uint64), (#left, #right)⟧ ⤳[under]
      #(decide (left = right)) := by
  infer_instance

example (left right : GoArray w64 4) :
    ⟦GoOp GoEquals felt.Felt.ty, (#left, #right)⟧ ⤳
      #(decide (left = right)) := by
  infer_instance

example (left right : GoArray w64 4) :
    ⟦GoOp GoEquals felt.SierraClassHash.ty, (#left, #right)⟧ ⤳
      #(decide (left = right)) := by
  infer_instance

end Perennial
