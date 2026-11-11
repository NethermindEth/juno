import ProofCheck.FeltMapUnique
import ProofCheck.FeltMapDepth

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

abbrev StorageDiffsValue := Option (GMap FeltMapKey FeltMapValue)

/-- Entries are applied in wire order. Each later entry replaces the complete
    prior inner map at that key; entries are never merged across duplicate keys. -/
def storageDiffsApply (old : GMap FeltMapKey StorageDiffsValue) (entries : List (FeltMapKey × StorageDiffsValue)) :
    GMap FeltMapKey StorageDiffsValue :=
  entries.foldl (fun out entry => <[entry.1 := entry.2]> out) old

def storageDiffsOfEntries (entries : List (FeltMapKey × StorageDiffsValue)) : GMap FeltMapKey StorageDiffsValue :=
  storageDiffsApply ∅ entries

theorem storageDiffsApply_append (old : GMap FeltMapKey StorageDiffsValue) (left right : List (FeltMapKey × StorageDiffsValue)) :
    storageDiffsApply old (left ++ right) = storageDiffsApply (storageDiffsApply old left) right := by
  simp only [storageDiffsApply, List.foldl_append]

theorem storageDiffsOfEntries_append_one (entries : List (FeltMapKey × StorageDiffsValue))
    (key : FeltMapKey) (value : StorageDiffsValue) :
    storageDiffsOfEntries (entries ++ [(key, value)]) = <[key := value]> (storageDiffsOfEntries entries) := by
  simp only [storageDiffsOfEntries, storageDiffsApply, List.foldl_append, List.foldl_cons, List.foldl_nil]

theorem storageDiffsOfEntries_last (entries : List (FeltMapKey × StorageDiffsValue))
    (key : FeltMapKey) (value : StorageDiffsValue) :
    storageDiffsOfEntries (entries ++ [(key, value)]) !! key = some value := by
  rw [storageDiffsOfEntries_append_one]
  simp

/-- A null/undefined later value replaces a previously populated entry
    with a present nil inner map, distinct from an absent map key. -/
theorem storageDiffsOfEntries_last_null (entries : List (FeltMapKey × StorageDiffsValue)) (key : FeltMapKey) :
    storageDiffsOfEntries (entries ++ [(key, none)]) !! key = some none :=
  storageDiffsOfEntries_last entries key none

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsApply_append
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsOfEntries_append_one
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsOfEntries_last
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsOfEntries_last_null
