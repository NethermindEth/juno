import ProofCheck.LimbItemsUnique
import Perennial.Golang.Theory.Map

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Both map counts are bounded by half the same representable input size.
    Therefore the nested map's remaining pair count plus the outer reserve hint
    remains a positive representable signed request. -/
theorem feltReserveWant_bounds (bytes inner outer left extra : Nat)
    (size : bytes < 2 ^ 63) (innerFit : 2 * inner ≤ bytes) (outerFit : 2 * outer ≤ bytes)
    (live : 0 < left ∧ left ≤ inner) (hint : extra ≤ outer) :
    0 < left + extra ∧ left + extra < 2 ^ 63 := by
  omega

/-- The machine addition used by cborReadFeltMapFrom agrees with mathematical
    addition under the count bounds; this is not an assumed overflow guarantee. -/
theorem feltReserveWant_add (bytes inner outer left extra : Nat)
    (size : bytes < 2 ^ 63) (innerFit : 2 * inner ≤ bytes) (outerFit : 2 * outer ≤ bytes)
    (live : 0 < left ∧ left ≤ inner) (hint : extra ≤ outer) :
    sint.Z (W64 left + W64 extra) = (left + extra : Nat) ∧ 0 < sint.Z (W64 left + W64 extra) := by
  have bounds := feltReserveWant_bounds bytes inner outer left extra size innerFit outerFit live hint
  constructor <;> word

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltReserveWant_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.feltReserveWant_add
