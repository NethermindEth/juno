import ProofCheck.L1MessageValues
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

/-- The sender address is inline. The nonce, selector, recipient and payload
    own their separate child allocations; the full struct is retained separately. -/
def L1MessageChildren (h : L1ToL2Message) (value : L1MessageValue) : IProp GF := iprop(
  ⌜h.From' = value.sender⌝ ∗ FeltResult h.Nonce' value.nonce ∗ FeltValuesResult h.Payload' value.payload ∗
  FeltResult h.Selector' value.selector ∗ FeltResult h.To' value.recipient)

def L1MessageOwned (dst : Loc) (value : L1MessageValue) : IProp GF := iprop(
  ∃ h : L1ToL2Message, dst ↦ h ∗ L1MessageChildren h value)

/-- Borrow the actual From cell and retain every other field. -/
theorem l1Message_sender_acc (dst : Loc) (h : L1ToL2Message) :
    dst ↦ h ⊢ (structFieldRef L1ToL2Message go!"From" dst ↦ h.From' ∗
      (∀ value : GoArray w8 20, structFieldRef L1ToL2Message go!"From" dst ↦ value -∗ dst ↦ {h with From' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hnonce, Hpayload, Hselector, Hrecipient, -⟩
  iframe Hsender
  iintro %value Hsender
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hnonce Hpayload Hselector Hrecipient

/-- Borrow the actual Nonce cell and retain every other field. -/
theorem l1Message_nonce_acc (dst : Loc) (h : L1ToL2Message) :
    dst ↦ h ⊢ (structFieldRef L1ToL2Message go!"Nonce" dst ↦ h.Nonce' ∗
      (∀ value : Loc, structFieldRef L1ToL2Message go!"Nonce" dst ↦ value -∗ dst ↦ {h with Nonce' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hnonce, Hpayload, Hselector, Hrecipient, -⟩
  iframe Hnonce
  iintro %value Hnonce
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hnonce Hpayload Hselector Hrecipient

/-- Borrow the actual Payload cell and retain every other field. -/
theorem l1Message_payload_acc (dst : Loc) (h : L1ToL2Message) :
    dst ↦ h ⊢ (structFieldRef L1ToL2Message go!"Payload" dst ↦ h.Payload' ∗
      (∀ value : GoSlice, structFieldRef L1ToL2Message go!"Payload" dst ↦ value -∗ dst ↦ {h with Payload' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hnonce, Hpayload, Hselector, Hrecipient, -⟩
  iframe Hpayload
  iintro %value Hpayload
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hnonce Hpayload Hselector Hrecipient

/-- Borrow the actual Selector cell and retain every other field. -/
theorem l1Message_selector_acc (dst : Loc) (h : L1ToL2Message) :
    dst ↦ h ⊢ (structFieldRef L1ToL2Message go!"Selector" dst ↦ h.Selector' ∗
      (∀ value : Loc, structFieldRef L1ToL2Message go!"Selector" dst ↦ value -∗ dst ↦ {h with Selector' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hnonce, Hpayload, Hselector, Hrecipient, -⟩
  iframe Hselector
  iintro %value Hselector
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hnonce Hpayload Hselector Hrecipient

/-- Borrow the actual To cell and retain every other field. -/
theorem l1Message_recipient_acc (dst : Loc) (h : L1ToL2Message) :
    dst ↦ h ⊢ (structFieldRef L1ToL2Message go!"To" dst ↦ h.To' ∗
      (∀ value : Loc, structFieldRef L1ToL2Message go!"To" dst ↦ value -∗ dst ↦ {h with To' := value}) : IProp GF) := by
  iintro Hh
  icases typedPointsto_not_null_dup dst h (DFrac.own 1) $$ Hh with ⟨Hh, %Hnn⟩
  icases typedPointsto_split dst h (DFrac.own 1) $$ Hh with Hh
  isimp only [TypedPointsto.typedPointstoDef] at Hh
  icases Hh with ⟨Hsender, Hnonce, Hpayload, Hselector, Hrecipient, -⟩
  iframe Hrecipient
  iintro %value Hrecipient
  iapply typedPointsto_combine _ _ _ Hnn
  isimp only [TypedPointsto.typedPointstoDef]
  iframe Hsender Hnonce Hpayload Hselector Hrecipient

theorem l1MessageChildren_sender (h : L1ToL2Message) (old : L1MessageValue) (value : GoArray w8 20) :
    L1MessageChildren h old ⊢
      (L1MessageChildren {h with From' := value} {old with sender := value} : IProp GF) := by
  unfold L1MessageChildren
  iintro ⟨_, Hnonce, Hpayload, Hselector, Hrecipient⟩
  iframe Hnonce Hpayload Hselector Hrecipient
  ipureintro; rfl

theorem l1MessageChildren_nonce (h : L1ToL2Message) (old : L1MessageValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1MessageChildren h old ∗ FeltResult out value ⊢
      (L1MessageChildren {h with Nonce' := out} {old with nonce := value} : IProp GF) := by
  unfold L1MessageChildren
  iintro ⟨⟨%Hsender, Hnonce, Hpayload, Hselector, Hrecipient⟩, Hvalue⟩
  iframe Hvalue Hpayload Hselector Hrecipient
  ipureintro; exact Hsender

theorem l1MessageChildren_payload (h : L1ToL2Message) (old : L1MessageValue) (out : GoSlice) (value : Option (List (GoArray w64 4))) :
    L1MessageChildren h old ∗ FeltValuesResult out value ⊢
      (L1MessageChildren {h with Payload' := out} {old with payload := value} : IProp GF) := by
  unfold L1MessageChildren
  iintro ⟨⟨%Hsender, Hnonce, Hpayload, Hselector, Hrecipient⟩, Hvalue⟩
  iframe Hvalue Hnonce Hselector Hrecipient
  ipureintro; exact Hsender

theorem l1MessageChildren_selector (h : L1ToL2Message) (old : L1MessageValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1MessageChildren h old ∗ FeltResult out value ⊢
      (L1MessageChildren {h with Selector' := out} {old with selector := value} : IProp GF) := by
  unfold L1MessageChildren
  iintro ⟨⟨%Hsender, Hnonce, Hpayload, Hselector, Hrecipient⟩, Hvalue⟩
  iframe Hvalue Hnonce Hpayload Hrecipient
  ipureintro; exact Hsender

theorem l1MessageChildren_recipient (h : L1ToL2Message) (old : L1MessageValue) (out : Loc) (value : Option (GoArray w64 4)) :
    L1MessageChildren h old ∗ FeltResult out value ⊢
      (L1MessageChildren {h with To' := out} {old with recipient := value} : IProp GF) := by
  unfold L1MessageChildren
  iintro ⟨⟨%Hsender, Hnonce, Hpayload, Hselector, Hrecipient⟩, Hvalue⟩
  iframe Hvalue Hnonce Hpayload Hselector
  ipureintro; exact Hsender

theorem l1MessageChildren_zero : ⊢ (L1MessageChildren (zero_val L1ToL2Message) l1MessageZero : IProp GF) := by
  isimp only [L1MessageChildren, l1MessageZero, FeltResult, FeltValuesResult]
  ipureintro
  exact ⟨rfl, rfl, rfl, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Message_sender_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Message_nonce_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Message_payload_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Message_selector_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1Message_recipient_acc
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_sender
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_nonce
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_payload
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_selector
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_recipient
#print axioms Perennial.github_com.NethermindEth.juno.core.l1MessageChildren_zero
