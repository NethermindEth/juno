import ProofCheck.L2MessageValues
import ProofCheck.FeltValuesTyped
import ProofCheck.AddressComplete

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

/-- The address is inline in the struct; only the sender and payload have child
    allocations. Keeping the struct separate supports shared message arrays. -/
def L2MessageChildren (h : L2ToL1Message) (value : L2MessageValue) : IProp GF := iprop(
  FeltResult h.From' value.sender ∗ FeltValuesResult h.Payload' value.payload ∗ ⌜h.To' = value.recipient⌝)

def L2MessageOwned (dst : Loc) (value : L2MessageValue) : IProp GF := iprop(
  ∃ h : L2ToL1Message, dst ↦ h ∗ L2MessageChildren h value)

/-- Borrow the sender pointer cell, retaining the payload and inline address. -/
theorem l2Message_sender_acc (dst : Loc) (h : L2ToL1Message) :
    dst ↦ h ⊢ (structFieldRef L2ToL1Message go!"From" dst ↦ h.From' ∗
      (∀ value : Loc, structFieldRef L2ToL1Message go!"From" dst ↦ value -∗ dst ↦ {h with From' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hpayload, Hrecipient, -⟩
  iframe Hsender
  iintro %value Hsender
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hpayload Hrecipient

/-- Borrow the payload header and restore the full struct after replacement. -/
theorem l2Message_payload_acc (dst : Loc) (h : L2ToL1Message) :
    dst ↦ h ⊢ (structFieldRef L2ToL1Message go!"Payload" dst ↦ h.Payload' ∗
      (∀ value : GoSlice, structFieldRef L2ToL1Message go!"Payload" dst ↦ value -∗ dst ↦ {h with Payload' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hpayload, Hrecipient, -⟩
  iframe Hpayload
  iintro %value Hpayload
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hpayload Hrecipient

/-- Borrow the entire inline address array, retaining the two child headers. -/
theorem l2Message_recipient_acc (dst : Loc) (h : L2ToL1Message) :
    dst ↦ h ⊢ (structFieldRef L2ToL1Message go!"To" dst ↦ h.To' ∗
      (∀ value : GoArray w8 20, structFieldRef L2ToL1Message go!"To" dst ↦ value -∗ dst ↦ {h with To' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hpayload, Hrecipient, -⟩
  iframe Hrecipient
  iintro %value Hrecipient
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hpayload Hrecipient

theorem l2MessageChildren_sender (h : L2ToL1Message) (old : L2MessageValue) (p : Loc) (value : Option (GoArray w64 4)) :
    L2MessageChildren h old ∗ FeltResult p value ⊢
      (L2MessageChildren {h with From' := p} {old with sender := value} : IProp GF) := by
  unfold L2MessageChildren
  iintro ⟨⟨_, Hpayload, %Hrecipient⟩, Hsender⟩
  iframe Hsender Hpayload
  ipureintro; exact Hrecipient

theorem l2MessageChildren_payload (h : L2ToL1Message) (old : L2MessageValue)
    (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    L2MessageChildren h old ∗ FeltValuesResult out value ⊢
      (L2MessageChildren {h with Payload' := out} {old with payload := value} : IProp GF) := by
  unfold L2MessageChildren
  iintro ⟨⟨Hsender, _, %Hrecipient⟩, Hvalue⟩
  iframe Hsender Hvalue
  ipureintro; exact Hrecipient

theorem l2MessageChildren_recipient (h : L2ToL1Message) (old : L2MessageValue) (value : GoArray w8 20) :
    L2MessageChildren h old ⊢
      (L2MessageChildren {h with To' := value} {old with recipient := value} : IProp GF) := by
  unfold L2MessageChildren
  iintro ⟨Hsender, Hpayload, _⟩
  iframe Hsender Hpayload
  ipureintro; rfl

theorem l2MessageChildren_zero : ⊢ (L2MessageChildren (zero_val L2ToL1Message) l2MessageZero : IProp GF) := by
  isimp only [L2MessageChildren, l2MessageZero, FeltResult, FeltValuesResult]
  ipureintro
  exact ⟨rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l2Message_sender_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l2Message_payload_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l2Message_recipient_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageChildren_sender
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageChildren_payload
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageChildren_recipient
#print axioms Perennial.github_com.NethermindEth.juno.core.l2MessageChildren_zero
