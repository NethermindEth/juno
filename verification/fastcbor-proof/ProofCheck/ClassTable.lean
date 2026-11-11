import ProofCheck.ClassMetadata
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

/-- Concrete search/cache preconditions for every class-package table. These
    certificates are separate from the payload specification and initialization. -/
theorem classFieldNames_ascii (schema : ClassSchema) :
    ∀ name ∈ classNames schema, ASCIIBytes name := by
  cases schema <;>
    simp only [classNames, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals unfold ASCIIBytes; decide

theorem classFieldHeads_ordered (schema : ClassSchema) :
    FieldHeadsOrdered (classKeys schema) := by
  cases schema
  case deprecatedEntryPoint =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([102, 104] : List Int).Pairwise (· ≤ ·)
    decide
  case deprecatedCairoClass =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([99, 103, 105, 106, 108] : List Int).Pairwise (· ≤ ·)
    decide
  case sierraEntryPoint =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([101, 104] : List Int).Pairwise (· ≤ ·)
    decide
  case sierraEntryPointsByType =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([104, 105, 107] : List Int).Pairwise (· ≤ ·)
    decide
  case casmEntryPoint =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([102, 104, 104] : List Int).Pairwise (· ≤ ·)
    decide
  case segmentLengths =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([102, 104] : List Int).Pairwise (· ≤ ·)
    decide
  case casmClass =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([101, 101, 104, 104, 105, 107, 109, 111, 118] : List Int).Pairwise (· ≤ ·)
    decide
  case sierraClass =>
    apply fieldHeadsOrdered_of_pairwise
    apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
    change ([99, 103, 103, 104, 107, 107, 111] : List Int).Pairwise (· ≤ ·)
    decide

theorem classFieldEncodings_nonempty (schema : ClassSchema) :
    ∀ key ∈ classKeys schema, key.Encoded' ≠ [] := by
  cases schema <;>
    simp only [classKeys, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals decide

theorem classFieldEncodings_apart (schema : ClassSchema) :
    FieldEncodingsApart (classKeys schema) := by
  cases schema <;>
    simp only [FieldEncodingsApart, classKeys, List.pairwise_cons, List.Pairwise.nil,
      List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
      false_implies, implies_true, _root_.and_true, forall_const]
  all_goals
    repeat' apply And.intro
    all_goals decide

theorem classCachedShortTable (schema : ClassSchema) :
    CachedShortTable (classKeys schema) := by
  cases schema <;>
    simp only [CachedShortTable, classKeys, List.mem_cons,
      List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals first | rfl | decide

theorem classInlineFieldTable (schema : ClassSchema) :
    InlineFieldTable (classKeys schema) (classNames schema) := by
  cases schema <;>
    simp only [InlineFieldTable, classKeys, List.mem_cons, List.not_mem_nil,
      _root_.or_false, forall_eq_or_imp, forall_eq]
  all_goals
    repeat' apply And.intro
    all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.classFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.classFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.classFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.classCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.classInlineFieldTable
