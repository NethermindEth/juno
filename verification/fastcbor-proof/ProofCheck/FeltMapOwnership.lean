import Perennial.Golang.Theory.Map
import ProofCheck.FeltMapValues
import ProofCheck.FeltValid

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

abbrev FeltMapValueOwned (stored : Loc) (value : FeltMapValue) : IProp GF := FeltResult stored value

/-- Map slots own their felt children exactly once. Replacing an entry drops
    the previous felt ownership, as permitted by the model's garbage collection. -/
def FeltMapChildren (stored : GMap FeltMapKey Loc) (values : GMap FeltMapKey FeltMapValue) : IProp GF :=
  iprop([∗map] key ↦ storedValue;value ∈ stored;values, FeltMapValueOwned storedValue value)

def FeltMapOwned (out : Loc) (values : GMap FeltMapKey FeltMapValue) : IProp GF := iprop(
  ∃ stored : GMap FeltMapKey Loc, out ↦$ stored ∗ FeltMapChildren stored values)

/-- None represents a nil map; some empty represents allocated empty storage. -/
def FeltMapResult (out : Loc) (values : Option (GMap FeltMapKey FeltMapValue)) : IProp GF :=
  match values with
  | none => iprop(⌜out = map.nil⌝)
  | some values => FeltMapOwned out values

theorem feltMapChildren_empty : ⊢ (FeltMapChildren ∅ ∅ : IProp GF) := by
  unfold FeltMapChildren
  iapply (BigSepM2.bigSepM2_empty _).2
  iempintro

/-- The same ownership rule covers fresh and duplicate keys, with no distinct-key
    premise. The new pointer replaces the old slot and its nullable felt. -/
theorem feltMapChildren_insert (stored : GMap FeltMapKey Loc) (values : GMap FeltMapKey FeltMapValue)
    (key : FeltMapKey) (newStored : Loc) (value : FeltMapValue) :
    FeltMapChildren stored values ∗ FeltMapValueOwned newStored value ⊢
      (FeltMapChildren (<[key := newStored]> stored) (<[key := value]> values) : IProp GF) := by
  unfold FeltMapChildren
  iintro ⟨Hchildren, Hvalue⟩
  iapply (BigSepM2.bigSepM2_insert_elim
    (Φ := fun _ storedValue value => FeltMapValueOwned storedValue value)
    (m1 := stored) (m2 := values) (i := key) (x1 := newStored) (x2 := value)) $$ Hvalue Hchildren

theorem feltMapOwned_nonnull (out : Loc) (values : GMap FeltMapKey FeltMapValue) :
    FeltMapOwned out values ⊢ (⌜out ≠ map.nil⌝ : IProp GF) := by
  unfold FeltMapOwned
  iintro ⟨%stored, Hmap, _⟩
  rw [ownMap_unseal]
  unfold ownMapDef
  icases Hmap with ⟨%mv, %entries, Hheap, _⟩
  isimp only [map.nil]
  iapply heapPointsto_non_null $$ Hheap

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapChildren_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapChildren_insert
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapOwned_nonnull
