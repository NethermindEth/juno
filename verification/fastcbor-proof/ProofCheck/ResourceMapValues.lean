import ProofCheck.ResourceBoundsValues

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Entries are applied in wire order. Each later entry replaces the complete
    prior record at that key; fields are never merged across duplicate entries. -/
def resourceMapApply (old : GMap w32 ResourceBoundsValue) (entries : List (w32 × ResourceBoundsValue)) :
    GMap w32 ResourceBoundsValue :=
  entries.foldl (fun out entry => <[entry.1 := entry.2]> out) old

def resourceMapOfEntries (entries : List (w32 × ResourceBoundsValue)) : GMap w32 ResourceBoundsValue :=
  resourceMapApply ∅ entries

theorem resourceMapApply_append (old : GMap w32 ResourceBoundsValue) (left right : List (w32 × ResourceBoundsValue)) :
    resourceMapApply old (left ++ right) = resourceMapApply (resourceMapApply old left) right := by
  simp only [resourceMapApply, List.foldl_append]

theorem resourceMapOfEntries_append_one (entries : List (w32 × ResourceBoundsValue))
    (key : w32) (value : ResourceBoundsValue) :
    resourceMapOfEntries (entries ++ [(key, value)]) = <[key := value]> (resourceMapOfEntries entries) := by
  simp only [resourceMapOfEntries, resourceMapApply, List.foldl_append, List.foldl_cons, List.foldl_nil]

theorem resourceMapOfEntries_last (entries : List (w32 × ResourceBoundsValue))
    (key : w32) (value : ResourceBoundsValue) :
    resourceMapOfEntries (entries ++ [(key, value)]) !! key = some value := by
  rw [resourceMapOfEntries_append_one]
  simp

/-- A null/undefined later value starts from zero and replaces a previously
    populated entry, including clearing its optional price. -/
theorem resourceMapOfEntries_last_null (entries : List (w32 × ResourceBoundsValue)) (key : w32) :
    resourceMapOfEntries (entries ++ [(key, resourceBoundsZero)]) !! key = some resourceBoundsZero :=
  resourceMapOfEntries_last entries key resourceBoundsZero

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapApply_append
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapOfEntries_append_one
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapOfEntries_last
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapOfEntries_last_null
