import Perennial.Golang.Theory.Map
import ProofCheck.StorageDiffsValues
import ProofCheck.FeltMapOwnership

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

abbrev StorageDiffsValueOwned (stored : Loc) (value : StorageDiffsValue) : IProp GF := FeltMapResult stored value

/-- Map slots own their inner maps and their felt children exactly once. Replacing an entry drops
    the previous inner-map ownership, as permitted by the model's garbage collection. -/
def StorageDiffsChildren (stored : GMap FeltMapKey Loc) (values : GMap FeltMapKey StorageDiffsValue) : IProp GF :=
  iprop([∗map] key ↦ storedValue;value ∈ stored;values, StorageDiffsValueOwned storedValue value)

def StorageDiffsOwned (out : Loc) (values : GMap FeltMapKey StorageDiffsValue) : IProp GF := iprop(
  ∃ stored : GMap FeltMapKey Loc, out ↦$ stored ∗ StorageDiffsChildren stored values)

/-- None represents a nil map; some empty represents allocated empty storage. -/
def StorageDiffsResult (out : Loc) (values : Option (GMap FeltMapKey StorageDiffsValue)) : IProp GF :=
  match values with
  | none => iprop(⌜out = map.nil⌝)
  | some values => StorageDiffsOwned out values

theorem storageDiffsChildren_empty : ⊢ (StorageDiffsChildren ∅ ∅ : IProp GF) := by
  unfold StorageDiffsChildren
  iapply (BigSepM2.bigSepM2_empty _).2
  iempintro

/-- The same ownership rule covers fresh and duplicate keys, with no distinct-key
    premise. The new map pointer replaces the old slot and its nullable inner map. -/
theorem storageDiffsChildren_insert (stored : GMap FeltMapKey Loc) (values : GMap FeltMapKey StorageDiffsValue)
    (key : FeltMapKey) (newStored : Loc) (value : StorageDiffsValue) :
    StorageDiffsChildren stored values ∗ StorageDiffsValueOwned newStored value ⊢
      (StorageDiffsChildren (<[key := newStored]> stored) (<[key := value]> values) : IProp GF) := by
  unfold StorageDiffsChildren
  iintro ⟨Hchildren, Hvalue⟩
  iapply (BigSepM2.bigSepM2_insert_elim
    (Φ := fun _ storedValue value => StorageDiffsValueOwned storedValue value)
    (m1 := stored) (m2 := values) (i := key) (x1 := newStored) (x2 := value)) $$ Hvalue Hchildren

theorem storageDiffsOwned_nonnull (out : Loc) (values : GMap FeltMapKey StorageDiffsValue) :
    StorageDiffsOwned out values ⊢ (⌜out ≠ map.nil⌝ : IProp GF) := by
  unfold StorageDiffsOwned
  iintro ⟨%stored, Hmap, _⟩
  rw [ownMap_unseal]
  unfold ownMapDef
  icases Hmap with ⟨%mv, %entries, Hheap, _⟩
  isimp only [map.nil]
  iapply heapPointsto_non_null $$ Hheap

/-- An allocated empty outer map owns no inner maps. -/
theorem storageDiffsOwned_empty (out : Loc) :
    (out ↦$ (∅ : GMap FeltMapKey Loc)) ⊢ (StorageDiffsOwned out ∅ : IProp GF) := by
  iintro Hmap
  unfold StorageDiffsOwned
  iexists (∅ : GMap FeltMapKey Loc)
  iframe Hmap
  iapply storageDiffsChildren_empty

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsChildren_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsChildren_insert
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsOwned_nonnull
#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffsOwned_empty
