import ProofCheck.GasPriceKeys

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def gasPriceValueSet (wei : Bool) (old : GasPriceValue) (value : Option (GoArray w64 4)) : GasPriceValue :=
  if wei then {old with wei := value} else {old with fri := value}

/-- Replacing one amount preserves ownership of the other amount. -/
theorem gasPriceChildren_set (g : GasPrice) (old : GasPriceValue) (wei : Bool)
    (p : Loc) (value : Option (GoArray w64 4)) :
    GasPriceChildren g old ∗ FeltResult p value ⊢
      (GasPriceChildren (gasPriceSet wei g p) (gasPriceValueSet wei old value) : IProp GF) := by
  unfold GasPriceChildren
  iintro ⟨⟨Hwei, Hfri⟩, Hvalue⟩
  cases wei <;> isimp only [gasPriceSet, gasPriceValueSet, Bool.false_eq_true, ite_false, ite_true]
  · iframe Hwei Hvalue
  · iframe Hvalue Hfri

/-- A successful map entry for the currently proved key families. Values are
    fully general felt/Skip encodings; unknown keys may repeat. -/
inductive GasPriceEntry (depth : Nat) where
  | amount (wei : Bool) (key : GasKeyWire) (choice : key.choice = some wei) (value : FeltWire depth)
  | ignored (key : GasKeyWire) (choice : key.choice = none) (value : List w8) (valid : SkipValid depth value)

def GasPriceEntry.key {d : Nat} : GasPriceEntry d → GasKeyWire
  | .amount _ key _ _ => key
  | .ignored key _ _ _ => key

def GasPriceEntry.valueBytes {d : Nat} : GasPriceEntry d → List w8
  | .amount _ _ _ value => value.bytes
  | .ignored _ _ value _ => value

def GasPriceEntry.bytes {d : Nat} (entry : GasPriceEntry d) : List w8 := entry.key.bytes ++ entry.valueBytes

def GasPriceEntry.apply {d : Nat} (old : GasPriceValue) : GasPriceEntry d → GasPriceValue
  | .amount wei _ _ value => gasPriceValueSet wei old value.value
  | .ignored _ _ _ _ => old

def GasEntriesFresh {d : Nat} (seen : w64) : List (GasPriceEntry d) → Prop
  | [] => True
  | entry :: rest => GasKeyFresh seen entry.key.choice ∧ GasEntriesFresh (gasKeySeen seen entry.key.choice) rest

theorem gasPriceEntry_length (d : Nat) (entry : GasPriceEntry d) : 2 ≤ entry.bytes.length := by
  have hk := gasKey_nonempty entry.key
  cases entry with
  | amount wei key choice value =>
    have hv := feltWire_nonempty d value
    simp only [GasPriceEntry.bytes, GasPriceEntry.key, GasPriceEntry.valueBytes, List.length_append] at *
    omega
  | ignored key choice value valid =>
    have hv := skipValid_nonempty d value valid
    simp only [GasPriceEntry.bytes, GasPriceEntry.key, GasPriceEntry.valueBytes, List.length_append] at *
    omega

theorem gasPriceEntries_count_bound (d : Nat) (entries : List (GasPriceEntry d)) :
    2 * entries.length ≤ (entries.flatMap GasPriceEntry.bytes).length := by
  induction entries with
  | nil => simp
  | cons entry entries ih =>
    have := gasPriceEntry_length d entry
    simp only [List.length_cons, List.flatMap_cons, List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceChildren_set
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceEntry_length
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPriceEntries_count_bound
