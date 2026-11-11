import ProofCheck.EventValues
import ProofCheck.FeltValuesTyped

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Each Event owns its optional sender and both nullable felt-value slices.
    The struct cell is separate to support shared event-array allocation. -/
def EventChildren (h : Event) (value : EventValue) : IProp GF := iprop(
  FeltResult h.From' value.sender ∗ FeltValuesResult h.Keys' value.keys ∗ FeltValuesResult h.Data' value.data)

def EventOwned (dst : Loc) (value : EventValue) : IProp GF := iprop(
  ∃ h : Event, dst ↦ h ∗ EventChildren h value)

def eventValuesFieldName (keys : Bool) : GoString := if keys then go!"Keys" else go!"Data"
def eventValuesField (keys : Bool) (h : Event) : GoSlice := if keys then h.Keys' else h.Data'
def eventSetValues (keys : Bool) (h : Event) (out : GoSlice) : Event :=
  if keys then {h with Keys' := out} else {h with Data' := out}
def eventValueSetValues (keys : Bool) (old : EventValue) (value : Option (List (GoArray w64 4))) : EventValue :=
  if keys then {old with keys := value} else {old with data := value}

theorem eventSetValues_original (keys : Bool) (h : Event) :
    eventSetValues keys h (eventValuesField keys h) = h := by cases keys <;> rfl

/-- Borrow the actual From pointer cell and retain both slice headers. -/
theorem event_sender_acc (dst : Loc) (h : Event) :
    dst ↦ h ⊢ (structFieldRef Event go!"From" dst ↦ h.From' ∗
      (∀ value : Loc, structFieldRef Event go!"From" dst ↦ value -∗ dst ↦ {h with From' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hkeys, Hdata, -⟩
  iframe Hsender
  iintro %value Hsender
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hkeys Hdata

/-- Borrow either actual slice header and reconstruct the full Event after
    arbitrary replacement, retaining the other header and sender pointer. -/
theorem event_values_acc (dst : Loc) (h : Event) (keys : Bool) :
    dst ↦ h ⊢ (iprop(
      structFieldRef Event (eventValuesFieldName keys) dst ↦ eventValuesField keys h ∗
      (∀ out : GoSlice, structFieldRef Event (eventValuesFieldName keys) dst ↦ out -∗
        dst ↦ eventSetValues keys h out)) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hkeys, Hdata, -⟩
  cases keys <;> isimp only [eventValuesFieldName, eventValuesField, eventSetValues, Bool.false_eq_true, ite_false, ite_true]
  · iframe Hdata
    iintro %out Hdata
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe Hsender Hkeys Hdata
  · iframe Hkeys
    iintro %out Hkeys
    iapply typedPointsto_combine _ _ _ Hnn
    isimp only [TypedPointsto.typedPointstoDef]
    iframe Hsender Hkeys Hdata

theorem eventChildren_sender (h : Event) (old : EventValue) (p : Loc) (value : Option (GoArray w64 4)) :
    EventChildren h old ∗ FeltResult p value ⊢
      (EventChildren {h with From' := p} {old with sender := value} : IProp GF) := by
  unfold EventChildren
  iintro ⟨⟨_, Hkeys, Hdata⟩, Hsender⟩
  iframe Hsender Hkeys Hdata

theorem eventChildren_values (h : Event) (old : EventValue) (keys : Bool)
    (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    EventChildren h old ∗ FeltValuesResult out value ⊢
      (EventChildren (eventSetValues keys h out) (eventValueSetValues keys old value) : IProp GF) := by
  unfold EventChildren
  iintro ⟨⟨Hsender, Hkeys, Hdata⟩, Hvalue⟩
  cases keys <;> isimp only [eventSetValues, eventValueSetValues, Bool.false_eq_true, ite_false, ite_true]
  · iframe Hsender Hkeys Hvalue
  · iframe Hsender Hvalue Hdata

/-- A freshly allocated zero Event owns no child allocations. -/
theorem eventChildren_zero : ⊢ (EventChildren (zero_val Event) eventZero : IProp GF) := by
  isimp only [EventChildren, eventZero, FeltResult, FeltValuesResult]
  ipureintro
  exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.eventSetValues_original
#print axioms Perennial.github_com.NethermindEth.juno.core.event_sender_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.event_values_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.eventChildren_sender
#print axioms Perennial.github_com.NethermindEth.juno.core.eventChildren_values
#print axioms Perennial.github_com.NethermindEth.juno.core.eventChildren_zero
