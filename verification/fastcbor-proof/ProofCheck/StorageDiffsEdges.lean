import ProofCheck.StorageDiffsUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Null/undefined inner maps are accepted as present nil values. -/
theorem storageDiffsEntry_nullish_inner (d : Nat) (item rest : List w8)
    (key : FeltMapKey) (head : w8) (valid : FeltMapKeyValid d item key)
    (hn : head = W8 246 ∨ head = W8 247) :
    StorageDiffsEntryValid d (item ++ [head]) rest (key, none) := by
  exact .pair valid (.nullish head rest hn)

/-- Empty inner maps are distinct from null inner maps at any accepted width. -/
theorem storageDiffsEntry_empty_inner (d : Nat) (item payload rest : List w8)
    (key : FeltMapKey) (head : w8) (valid : FeltMapKeyValid d item key)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    StorageDiffsEntryValid d (item ++ head :: payload) rest (key, some ∅) := by
  apply StorageDiffsEntryValid.pair valid
  simpa only [List.append_nil, feltMapOfEntries, feltMapApply, List.foldl_nil] using
    (FeltMapSpec.map (count := 0) (item := []) enc major (by rfl)
      (by simp) depth (.nil rest))

/-- Every supported width encoding zero outer pairs produces a nonnil empty map. -/
theorem storageDiffsSpec_empty (d : Nat) (head : w8) (payload rest : List w8)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    StorageDiffsSpec d ((head :: payload) ++ rest) (.success (some ∅) rest) := by
  simpa only [List.append_nil, storageDiffsOfEntries, storageDiffsApply, List.foldl_nil] using
    (StorageDiffsSpec.map (count := 0) (item := []) enc major (by rfl)
      (by simp) depth (.nil rest))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntry_nullish_inner
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsEntry_empty_inner
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsSpec_empty
