import ProofCheck.StateMetadata
import ProofCheck.FieldRead
import ProofCheck.FindWords

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Concrete search/cache preconditions for every state-package table. These
    certificates are separate from the payload specification and initialization. -/
theorem stateFieldNames_ascii (schema : StateSchema) :
    ∀ name ∈ stateNames schema, ASCIIBytes name := by
  cases schema <;>
    simp only [stateNames, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals unfold ASCIIBytes; decide

theorem stateFieldHeads_ordered (schema : StateSchema) :
    FieldHeadsOrdered (stateKeys schema) := by
  cases schema
  case stateDiff =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([102, 108, 111, 111, 113, 113, 113] : List Int).Pairwise (· ≤ ·)
    decide
  case stateUpdate =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([103, 103, 105, 105] : List Int).Pairwise (· ≤ ·)
    decide

theorem stateFieldEncodings_nonempty (schema : StateSchema) :
    ∀ key ∈ stateKeys schema, key.Encoded' ≠ [] := by
  cases schema <;>
    simp only [stateKeys, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals decide

theorem stateFieldEncodings_apart (schema : StateSchema) :
    FieldEncodingsApart (stateKeys schema) := by
  cases schema <;>
    simp only [FieldEncodingsApart, stateKeys, List.pairwise_cons, List.Pairwise.nil,
      List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
      false_implies, implies_true, _root_.and_true, forall_const]
  all_goals
    repeat' apply And.intro
    all_goals decide

theorem stateCachedShortTable (schema : StateSchema) :
    CachedShortTable (stateKeys schema) := by
  cases schema <;>
    simp only [CachedShortTable, stateKeys, List.mem_cons,
      List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals first | rfl | decide

theorem stateInlineFieldTable (schema : StateSchema) :
    InlineFieldTable (stateKeys schema) (stateNames schema) := by
  cases schema <;>
    simp only [InlineFieldTable, stateKeys, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.stateFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.stateFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.stateCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.stateInlineFieldTable
