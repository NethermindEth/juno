import ProofCheck.ReceiptState
import ProofCheck.AvailabilityKeys
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

abbrev availabilityFieldKeys := receiptKeys .dataAvailability
abbrev availabilityFieldNames := receiptNames .dataAvailability

def availabilityKeyIndex : AvailabilityField → w64
  | .gas => W64 0
  | .dataGas => W64 1

def availabilityKeyCode : Option AvailabilityField → w64
  | none => W64 (-2)
  | some field => availabilityKeyIndex field

def availabilityKeySeen (seen : w64) : Option AvailabilityField → w64
  | none => seen
  | some field => seen ||| (W64 1 <<< availabilityKeyIndex field)

def AvailabilityKeyFresh (seen : w64) : Option AvailabilityField → Prop
  | none => True
  | some field => seen &&& (W64 1 <<< availabilityKeyIndex field) = W64 0

/-- Actual table names agree with the independent DataAvailability schema. -/
theorem availabilityTable_names : availabilityFieldNames = availabilityNames := rfl

theorem availabilityKey_lookup (field : AvailabilityField) :
    availabilityFieldNames[(sint.Z (availabilityKeyIndex field)).toNat]? = some (availabilityFieldName field) := by
  cases field <;> rfl

theorem availabilityFoldedNames_distinct : (availabilityFieldNames.map (List.map fieldFoldByte)).Nodup :=
  receiptNames_distinct .dataAvailability

theorem availabilityFieldNames_ascii : ∀ name ∈ availabilityFieldNames, ASCIIBytes name := by
  simp only [availabilityFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Unknown abstract fields exclude every actual table name. -/
theorem availabilityKeyMeaning_unknown (key : GoString) (meaning : AvailabilityKeyMeaning key none) :
    ∀ name ∈ availabilityFieldNames, key.map fieldFoldByte ≠ name.map fieldFoldByte := by
  simp only [availabilityFieldNames, receiptNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  exact ⟨meaning .gas, meaning .dataGas⟩

theorem availabilityFieldHeads_ordered : FieldHeadsOrdered availabilityFieldKeys := by
  apply fieldHeadsOrdered_of_pairwise
  apply (List.pairwise_map (f := fun k : cursor.StaticField => uint.Z (k.Encoded'.headD (W8 0)))).mp
  change ([101, 105] : List Int).Pairwise (· ≤ ·)
  decide

theorem availabilityFieldEncodings_nonempty : ∀ key ∈ availabilityFieldKeys, key.Encoded' ≠ [] := by
  simp only [availabilityFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  decide

theorem availabilityFieldEncodings_apart : FieldEncodingsApart availabilityFieldKeys := by
  simp only [FieldEncodingsApart, availabilityFieldKeys, receiptKeys, List.pairwise_cons, List.Pairwise.nil,
    List.mem_cons, List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq,
    false_implies, implies_true, _root_.and_true, forall_const]
  repeat' apply And.intro
  all_goals decide

theorem availabilityCachedShortTable : CachedShortTable availabilityFieldKeys := by
  simp only [CachedShortTable, availabilityFieldKeys, receiptKeys, List.mem_cons,
    List.not_mem_nil, _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals first | rfl | decide

theorem availabilityInlineFieldTable : InlineFieldTable availabilityFieldKeys availabilityFieldNames := by
  simp only [InlineFieldTable, availabilityFieldKeys, receiptKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityTable_names
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKey_lookup
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityFoldedNames_distinct
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityFieldNames_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityKeyMeaning_unknown
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityFieldHeads_ordered
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityFieldEncodings_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityFieldEncodings_apart
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityCachedShortTable
#print axioms Perennial.github_com.NethermindEth.juno.core.availabilityInlineFieldTable
