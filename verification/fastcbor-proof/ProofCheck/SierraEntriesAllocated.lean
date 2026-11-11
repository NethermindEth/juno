import ProofCheck.SierraEntriesRecordComplete

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

theorem sierraEntriesFailureOwned_allocated (result : GoInterface) (pos : w64) (reason : SierraEntriesFailureReason) :
    SierraEntriesFailureOwned (GF := GF) result pos reason ⊢
      ⌜∃ err : Loc, result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ := by
  unfold SierraEntriesFailureOwned
  iintro ⟨%kinds, _, Herr⟩
  cases reason with
  | opening | key =>
    isimp only [SierraEntriesFailureError] at Herr
    icases Herr with ⟨%err, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | duplicate =>
    isimp only [SierraEntriesFailureError] at Herr
    icases Herr with ⟨%err, %args, %Heq, _⟩
    ipureintro; exact ⟨err, Heq⟩
  | value reason => cases reason with
    | field reason =>
      cases reason
      isimp only [SierraEntriesFailureError, SierraEntriesFieldError] at Herr
      icases Herr with ⟨%err, %Heq, _⟩
      ipureintro; exact ⟨err, Heq⟩
    | ignored =>
      isimp only [SierraEntriesFailureError] at Herr
      icases Herr with ⟨%err, %Heq, _⟩
      ipureintro; exact ⟨err, Heq⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.sierraEntriesFailureOwned_allocated
