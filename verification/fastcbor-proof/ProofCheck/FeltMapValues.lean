import ProofCheck.FeltMapKeySpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Entries are applied in wire order. Each later entry replaces the complete
    prior pointer at that key; entries are never merged across duplicate keys. -/
def feltMapApply (old : GMap FeltMapKey FeltMapValue) (entries : List (FeltMapKey × FeltMapValue)) :
    GMap FeltMapKey FeltMapValue :=
  entries.foldl (fun out entry => <[entry.1 := entry.2]> out) old

def feltMapOfEntries (entries : List (FeltMapKey × FeltMapValue)) : GMap FeltMapKey FeltMapValue :=
  feltMapApply ∅ entries

theorem feltMapApply_append (old : GMap FeltMapKey FeltMapValue) (left right : List (FeltMapKey × FeltMapValue)) :
    feltMapApply old (left ++ right) = feltMapApply (feltMapApply old left) right := by
  simp only [feltMapApply, List.foldl_append]

theorem feltMapOfEntries_append_one (entries : List (FeltMapKey × FeltMapValue))
    (key : FeltMapKey) (value : FeltMapValue) :
    feltMapOfEntries (entries ++ [(key, value)]) = <[key := value]> (feltMapOfEntries entries) := by
  simp only [feltMapOfEntries, feltMapApply, List.foldl_append, List.foldl_cons, List.foldl_nil]

theorem feltMapOfEntries_last (entries : List (FeltMapKey × FeltMapValue))
    (key : FeltMapKey) (value : FeltMapValue) :
    feltMapOfEntries (entries ++ [(key, value)]) !! key = some value := by
  rw [feltMapOfEntries_append_one]
  simp

/-- A null/undefined later value replaces a previously populated entry
    with a present nil pointer, distinct from an absent map key. -/
theorem feltMapOfEntries_last_null (entries : List (FeltMapKey × FeltMapValue)) (key : FeltMapKey) :
    feltMapOfEntries (entries ++ [(key, none)]) !! key = some none :=
  feltMapOfEntries_last entries key none

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapApply_append
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapOfEntries_append_one
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapOfEntries_last
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapOfEntries_last_null
