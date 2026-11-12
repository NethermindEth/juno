import ProofCheck.Utf8Loop
import ProofCheck.Utf8Contract
import ProofCheck.Utf8WordImpl

set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Supplying the actual generic word-helper proof discharges the complete
    validator contract used by the Header/API chain. -/
theorem utf8ValidatorContract_from_word (wordContract : Utf8WordWindowContract (GF := GF)) :
    Utf8ValidatorContract (GF := GF) := by
  intro data xs dq
  exact wp_utf8_valid_from_word wordContract data xs dq

/-- The actual Go validator implements the independent UTF-8 relation on every
    owned byte input, with no helper correctness premise. -/
theorem utf8ValidatorContract_proved : Utf8ValidatorContract (GF := GF) :=
  utf8ValidatorContract_from_word utf8WordWindowContract_proved

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8ValidatorContract_from_word

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8ValidatorContract_proved
