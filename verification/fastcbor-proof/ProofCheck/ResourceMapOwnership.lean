import Perennial.Golang.Theory.Map
import ProofCheck.ResourceMapValues
import ProofCheck.ResourceBoundsOwnership

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

def ResourceMapValueOwned (stored : ResourceBounds) (value : ResourceBoundsValue) : IProp GF := iprop(
  ⌜stored.MaxAmount' = value.amount⌝ ∗ FeltResult stored.MaxPricePerUnit' value.price)

/-- Map slots own their price children exactly once. Replacing an entry drops
    the previous child ownership, as permitted by the model's garbage collection. -/
def ResourceMapChildren (stored : GMap w32 ResourceBounds) (values : GMap w32 ResourceBoundsValue) : IProp GF :=
  iprop([∗map] key ↦ storedValue;value ∈ stored;values, ResourceMapValueOwned storedValue value)

def ResourceMapOwned (out : Loc) (values : GMap w32 ResourceBoundsValue) : IProp GF := iprop(
  ∃ stored : GMap w32 ResourceBounds, out ↦$ stored ∗ ResourceMapChildren stored values)

/-- None represents a nil map; some empty represents allocated empty storage. -/
def ResourceMapResult (out : Loc) (values : Option (GMap w32 ResourceBoundsValue)) : IProp GF :=
  match values with
  | none => iprop(⌜out = map.nil⌝)
  | some values => ResourceMapOwned out values

theorem resourceMapChildren_empty : ⊢ (ResourceMapChildren ∅ ∅ : IProp GF) := by
  unfold ResourceMapChildren
  iapply (BigSepM2.bigSepM2_empty _).2
  iempintro

/-- The same ownership rule covers fresh and duplicate keys, with no distinct-key
    premise. The new whole record replaces the old slot and its nullable child. -/
theorem resourceMapChildren_insert (stored : GMap w32 ResourceBounds) (values : GMap w32 ResourceBoundsValue)
    (key : w32) (newStored : ResourceBounds) (value : ResourceBoundsValue) :
    ResourceMapChildren stored values ∗ ResourceMapValueOwned newStored value ⊢
      (ResourceMapChildren (<[key := newStored]> stored) (<[key := value]> values) : IProp GF) := by
  unfold ResourceMapChildren
  iintro ⟨Hchildren, Hvalue⟩
  iapply (BigSepM2.bigSepM2_insert_elim
    (Φ := fun _ storedValue value => ResourceMapValueOwned storedValue value)
    (m1 := stored) (m2 := values) (i := key) (x1 := newStored) (x2 := value)) $$ Hvalue Hchildren

/-- Move the temporary record's price ownership into its eventual map slot;
    the local record cell remains available for the actual Go load. -/
theorem resourceMapValue_take (dst : Loc) (value : ResourceBoundsValue) :
    ResourceBoundsOwned dst value ⊢ (iprop(∃ stored : ResourceBounds,
      dst ↦ stored ∗ ResourceMapValueOwned stored value) : IProp GF) := by
  unfold ResourceBoundsOwned ResourceMapValueOwned
  iintro ⟨%ptr, Hdst, Hprice⟩
  iexists ResourceBounds.mk value.amount ptr
  iframe Hdst Hprice
  ipureintro; rfl

theorem resourceMapOwned_nonnull (out : Loc) (values : GMap w32 ResourceBoundsValue) :
    ResourceMapOwned out values ⊢ (⌜out ≠ map.nil⌝ : IProp GF) := by
  unfold ResourceMapOwned
  iintro ⟨%stored, Hmap, _⟩
  rw [ownMap_unseal]
  unfold ownMapDef
  icases Hmap with ⟨%mv, %entries, Hheap, _⟩
  isimp only [map.nil]
  iapply heapPointsto_non_null $$ Hheap

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapChildren_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapChildren_insert
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapValue_take
#print axioms Perennial.github_com.NethermindEth.juno.core.resourceMapOwned_nonnull
