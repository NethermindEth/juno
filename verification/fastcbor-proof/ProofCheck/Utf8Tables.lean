import ProofCheck.Utf8Spec
import ProofCheck.Utf8Initialize

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

/-- Every invalid leading byte selects the actual Go table's invalid class.
    The finite 256-byte computation is checked by kernel reduction. -/
theorem utf8FirstTable_bad_lead (b : w8) (bad : Utf8BadLead b) :
    utf8FirstTable.arr[(uint.Z b).toNat]? = some (W8 241) := by
  have h : ∀ b : w8, Utf8BadLead b →
      utf8FirstTable.arr[(uint.Z b).toNat]? = some (W8 241) := by
    unfold Utf8BadLead
    decide
  exact h b bad

variable [GooseExts]

/-- Invalid starters use the zero accept-range slot (index 15). The validator
    reads it before taking its size-switch default and returning false. -/
theorem utf8AcceptTable_invalid :
    utf8AcceptTable.arr[15]? = some (unicode.utf8.acceptRange.mk (W8 0) (W8 0)) := by
  rfl

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8FirstTable_bad_lead
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8AcceptTable_invalid
