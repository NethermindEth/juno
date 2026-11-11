import ProofCheck.StateMapDiagnostics

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Empty maps retain allocated-empty identity, for every encoded zero-count
    head width and arbitrary remaining input. -/
theorem feltMap_empty_spec (d : Nat) (head : w8) (payload rest : List w8)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    FeltMapSpec d ((head :: payload) ++ rest) (.success (some ∅) rest) := by
  simpa only [List.nil_append, List.append_nil, feltMapOfEntries, feltMapApply, List.foldl_nil] using
    (FeltMapSpec.map (count := 0) enc major (by decide) (by simp) depth
      (FeltMapEntriesValid.nil (d := d + 1) rest))

/-- Empty maps retain allocated-empty identity, for every encoded zero-count
    head width and arbitrary remaining input. -/
theorem storageDiffs_empty_spec (d : Nat) (head : w8) (payload rest : List w8)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    StorageDiffsSpec d ((head :: payload) ++ rest) (.success (some ∅) rest) := by
  simpa only [List.nil_append, List.append_nil, storageDiffsOfEntries, storageDiffsApply, List.foldl_nil] using
    (StorageDiffsSpec.map (count := 0) enc major (by decide) (by simp) depth
      (StorageDiffsEntriesValid.nil (d := d + 1) rest))

/-- Empty maps retain allocated-empty identity, for every encoded zero-count
    head width and arbitrary remaining input. -/
theorem migratedClasses_empty_spec (d : Nat) (head : w8) (payload rest : List w8)
    (enc : HeadArgument head payload (W64 0)) (major : head &&& W8 224 = W8 160)
    (depth : d < 32) :
    MigratedClassesSpec d ((head :: payload) ++ rest) (.success (some ∅) rest) := by
  simpa only [List.nil_append, List.append_nil, migratedClassesOfEntries, migratedClassesApply, List.foldl_nil] using
    (MigratedClassesSpec.map (count := 0) enc major (by decide) (by simp) depth
      (MigratedClassesEntriesValid.nil (d := d + 1) rest))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMap_empty_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_empty_spec
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_empty_spec
