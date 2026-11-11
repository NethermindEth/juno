import ProofCheck.FeltMapKeySpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A Lean array wrapper alone does not enforce its Go type's four-word length. -/
def FeltWordsWellFormed (value : FeltMapKey) : Prop := value.arr.length = 4

def NullableFeltWellFormed : FeltMapValue → Prop
  | none => True
  | some value => FeltWordsWellFormed value

/-- Representation validity, independent of the missing Go array-equality rule. -/
def FeltKeyedMapWellFormed {V : Type} (valueValid : V → Prop)
    (values : GMap FeltMapKey V) : Prop :=
  ∀ key value, values !! key = some value → FeltWordsWellFormed key ∧ valueValid value

def FeltKeyedEntriesWellFormed {V : Type} (valueValid : V → Prop)
    (entries : List (FeltMapKey × V)) : Prop :=
  ∀ entry ∈ entries, FeltWordsWellFormed entry.1 ∧ valueValid entry.2

theorem paddedLimbs_wellFormed (items : List UnsignedWire) (bound : items.length ≤ 4) :
    FeltWordsWellFormed (paddedLimbs items) := by
  unfold FeltWordsWellFormed paddedLimbs
  simp only [List.length_append, List.length_map, List.length_replicate]
  omega

theorem feltValid_wellFormed (d : Nat) (wire : List w8) (value : FeltMapValue)
    (valid : FeltValid d wire value) : NullableFeltWellFormed value := by
  cases valid with
  | nullish => trivial
  | small items enc major count small bound => exact paddedLimbs_wellFormed items small
  | extended items extras enc major count four bound valid =>
    exact paddedLimbs_wellFormed items (by omega)

theorem feltMapKeyValid_wellFormed (d : Nat) (wire : List w8) (key : FeltMapKey)
    (valid : FeltMapKeyValid d wire key) : FeltWordsWellFormed key :=
  feltValid_wellFormed d wire (some key) valid

theorem feltKeyedMapWellFormed_empty {V : Type} (valueValid : V → Prop) :
    FeltKeyedMapWellFormed valueValid ∅ := by
  intro key value found
  simp at found

/-- Works for fresh and duplicate keys, including replacement by a nullable value. -/
theorem feltKeyedMapWellFormed_insert {V : Type} (valueValid : V → Prop)
    (old : GMap FeltMapKey V) (key : FeltMapKey) (value : V)
    (before : FeltKeyedMapWellFormed valueValid old)
    (keyValid : FeltWordsWellFormed key) (valid : valueValid value) :
    FeltKeyedMapWellFormed valueValid (<[key := value]> old) := by
  intro query result found
  by_cases same : query = key
  · subst query
    have resultEq : value = result := by simpa using found
    subst result
    exact ⟨keyValid, valid⟩
  · exact before query result (by simpa [same, Ne.symm same] using found)

theorem feltKeyedMapWellFormed_fold {V : Type} (valueValid : V → Prop)
    (old : GMap FeltMapKey V) (entries : List (FeltMapKey × V))
    (before : FeltKeyedMapWellFormed valueValid old)
    (valid : FeltKeyedEntriesWellFormed valueValid entries) :
    FeltKeyedMapWellFormed valueValid
      (entries.foldl (fun out entry => <[entry.1 := entry.2]> out) old) := by
  induction entries generalizing old with
  | nil => exact before
  | cons entry entries ih =>
    simp only [List.foldl_cons]
    have first := valid entry (by simp)
    apply ih
    · exact feltKeyedMapWellFormed_insert valueValid old entry.1 entry.2 before first.1 first.2
    · intro next member
      exact valid next (by simp [member])

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics]

/-- The actual four-word points-to supplies this fact for freshly read keys. -/
theorem feltKeyOwned_wellFormed (ptr : Loc) (key : FeltMapKey) (dq : DFrac) :
    ptr ↦{dq} key ⊢ (⌜FeltWordsWellFormed key⌝ : IProp GF) := by
  iintro Hkey
  ihave %length := array_len ptr dq 4 key.arr $$ Hkey
  ipureintro
  unfold FeltWordsWellFormed
  omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.paddedLimbs_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMapKeyValid_wellFormed
#print axioms Perennial.github_com.NethermindEth.juno.core.feltKeyedMapWellFormed_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.feltKeyedMapWellFormed_insert
#print axioms Perennial.github_com.NethermindEth.juno.core.feltKeyedMapWellFormed_fold
#print axioms Perennial.github_com.NethermindEth.juno.core.feltKeyOwned_wellFormed
