import ProofCheck.FeltMapKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Entries are applied in wire order. Each later entry replaces the complete
    prior by-value felt at that key; entries are never merged across duplicate keys. -/
def migratedClassesApply (old : GMap FeltMapKey FeltMapKey) (entries : List (FeltMapKey × FeltMapKey)) :
    GMap FeltMapKey FeltMapKey :=
  entries.foldl (fun out entry => <[entry.1 := entry.2]> out) old

def migratedClassesOfEntries (entries : List (FeltMapKey × FeltMapKey)) : GMap FeltMapKey FeltMapKey :=
  migratedClassesApply ∅ entries

theorem migratedClassesApply_append (old : GMap FeltMapKey FeltMapKey) (left right : List (FeltMapKey × FeltMapKey)) :
    migratedClassesApply old (left ++ right) = migratedClassesApply (migratedClassesApply old left) right := by
  simp only [migratedClassesApply, List.foldl_append]

theorem migratedClassesOfEntries_append_one (entries : List (FeltMapKey × FeltMapKey))
    (key : FeltMapKey) (value : FeltMapKey) :
    migratedClassesOfEntries (entries ++ [(key, value)]) = <[key := value]> (migratedClassesOfEntries entries) := by
  simp only [migratedClassesOfEntries, migratedClassesApply, List.foldl_append, List.foldl_cons, List.foldl_nil]

theorem migratedClassesOfEntries_last (entries : List (FeltMapKey × FeltMapKey))
    (key : FeltMapKey) (value : FeltMapKey) :
    migratedClassesOfEntries (entries ++ [(key, value)]) !! key = some value := by
  rw [migratedClassesOfEntries_append_one]
  simp

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesApply_append
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesOfEntries_append_one
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesOfEntries_last
