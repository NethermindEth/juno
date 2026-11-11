import ProofCheck.HeaderProjectionRecord

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Unused abstract slots are absent/zero. This gives a faithful shared value
    representation for the five different concrete Go projection layouts. -/
def HeaderProjectionCanonical : HeaderProjectionKind → HeaderProjectionValue → Prop
  | .hash, v => v.root = none ∧ v.count = W64 0 ∧ v.timestamp = none
  | .root, v => v.hash = none ∧ v.count = W64 0 ∧ v.timestamp = none
  | .count, v => v.hash = none ∧ v.root = none ∧ v.timestamp = none
  | .timestamp, v => v.hash = none ∧ v.root = none ∧ v.count = W64 0
  | .hashAndRoot, v => v.count = W64 0 ∧ v.timestamp = none

theorem headerProjectionZero_canonical (kind : HeaderProjectionKind) :
    HeaderProjectionCanonical kind headerProjectionZero := by
  cases kind <;> exact ⟨rfl, by first | exact ⟨rfl, rfl⟩ | rfl⟩

theorem headerProjectionField_canonical (d : Nat) (kind : HeaderProjectionKind)
    (field : HeaderProjectionField) (old final : HeaderProjectionValue) (wire : List w8)
    (member : field ∈ headerProjectionFields kind) (initial : HeaderProjectionCanonical kind old)
    (valid : HeaderProjectionFieldValid d field old wire final) : HeaderProjectionCanonical kind final := by
  cases valid <;> cases kind <;> simp_all [headerProjectionFields, HeaderProjectionCanonical]

theorem headerProjectionItem_canonical (d : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (choice : Option Nat) (old final : HeaderProjectionValue) (wire : List w8)
    (initial : HeaderProjectionCanonical kind old)
    (valid : HeaderProjectionItemValid d kind seen choice old wire final) : HeaderProjectionCanonical kind final := by
  cases valid with
  | ignored => exact initial
  | selected fresh lookup valid =>
    exact headerProjectionField_canonical _ _ _ _ _ _ (List.mem_of_getElem? lookup) initial valid

theorem headerProjectionPairs_canonical (d n : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (old final : HeaderProjectionValue) (wire : List w8) (initial : HeaderProjectionCanonical kind old)
    (valid : HeaderProjectionPairsValid d kind n seen old wire final) : HeaderProjectionCanonical kind final := by
  induction valid with
  | nil => exact initial
  | step keyValid valueValid rest ih =>
    exact ih (headerProjectionItem_canonical _ _ _ _ _ _ _ initial valueValid)

theorem headerProjectionPairsRejected_canonical (d n consumed offset : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (old final : HeaderProjectionValue) (wire : List w8) (reason : HeaderProjectionFailure)
    (initial : HeaderProjectionCanonical kind old)
    (failure : HeaderProjectionPairsRejected d kind n seen old wire final consumed offset reason) :
    HeaderProjectionCanonical kind final := by
  induction failure with
  | key | value => exact initial
  | next keyValid valueValid rest ih =>
    exact ih (headerProjectionItem_canonical _ _ _ _ _ _ _ initial valueValid)

theorem headerProjectionBody_canonical (d : Nat) (kind : HeaderProjectionKind)
    (old final : HeaderProjectionValue) (wire : List w8) (initial : HeaderProjectionCanonical kind old)
    (valid : HeaderProjectionBodyValid d kind old wire final) : HeaderProjectionCanonical kind final := by
  cases valid with
  | nullish => exact initial
  | map enc major length depth valid => exact headerProjectionPairs_canonical _ _ _ _ _ _ _ initial valid

theorem headerProjectionBodyRejected_canonical (d consumed offset : Nat) (kind : HeaderProjectionKind)
    (old final : HeaderProjectionValue) (wire : List w8) (entered : Bool) (reason : HeaderProjectionRecordFailure)
    (initial : HeaderProjectionCanonical kind old)
    (failure : HeaderProjectionBodyRejected d kind old wire final consumed offset entered reason) :
    HeaderProjectionCanonical kind final := by
  cases failure with
  | opening => exact initial
  | map enc major length depth fit failure =>
    exact headerProjectionPairsRejected_canonical _ _ _ _ _ _ _ _ _ _ initial failure

def headerProjectionResultValue : HeaderProjectionRecordResult → HeaderProjectionValue
  | .success final _ => final
  | .failure retained _ _ _ _ => retained

/-- Successful and rejected complete records retain a representable value for
    their particular Go layout, including partially published earlier fields. -/
theorem headerProjectionRecord_canonical (d : Nat) (kind : HeaderProjectionKind)
    (old : HeaderProjectionValue) (wire : List w8) (result : HeaderProjectionRecordResult)
    (initial : HeaderProjectionCanonical kind old) (spec : HeaderProjectionRecordSpec d kind old wire result) :
    HeaderProjectionCanonical kind (headerProjectionResultValue result) := by
  cases spec with
  | success tag split valid => exact headerProjectionBody_canonical _ _ _ _ _ initial valid
  | malformed | unsupported => exact initial
  | rejected tag failure => exact headerProjectionBodyRejected_canonical _ _ _ _ _ _ _ _ _ initial failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionZero_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionField_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItem_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairs_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairsRejected_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionBody_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionBodyRejected_canonical
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionRecord_canonical
