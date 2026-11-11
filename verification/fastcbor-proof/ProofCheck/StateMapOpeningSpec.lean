import ProofCheck.StateMapDiagnostics
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A specified failure that enters no map is exactly an opening rejection:
    no bytes are consumed, the error offset is zero, and the shared map-opening
    relation supplies the diagnostic. No entry failure can satisfy this shape. -/
theorem feltMap_zero_depth_failure_open (d consumed offset : Nat) (wire : List w8)
    (reason : FeltMapFailureReason) (depth : d ≤ 32)
    (failure : FeltMapSpec d wire (.failure consumed offset 0 reason)) :
    consumed = 0 ∧ offset = 0 ∧ StructOpenFailure d wire (feltMapDiagnostic reason) := by
  generalize hzero : (0 : Nat) = entered at failure
  cases failure with
  | outer hn hu failure => exact ⟨rfl, rfl, .outer hn hu failure⟩
  | count enc major size short => exact ⟨rfl, rfl, .count enc major (by omega)⟩
  | depth enc major size fit deep =>
    have eq : d = 32 := by omega
    subst d
    exact ⟨rfl, rfl, .depth enc major (by omega)⟩
  | element enc major size fit depth failure => omega

/-- A specified failure that enters no map is exactly an opening rejection:
    no bytes are consumed, the error offset is zero, and the shared map-opening
    relation supplies the diagnostic. No entry failure can satisfy this shape. -/
theorem storageDiffs_zero_depth_failure_open (d consumed offset : Nat) (wire : List w8)
    (reason : StorageDiffsFailureReason) (depth : d ≤ 32)
    (failure : StorageDiffsSpec d wire (.failure consumed offset 0 reason)) :
    consumed = 0 ∧ offset = 0 ∧ StructOpenFailure d wire (storageDiffsDiagnostic reason) := by
  generalize hzero : (0 : Nat) = entered at failure
  cases failure with
  | outer hn hu failure => exact ⟨rfl, rfl, .outer hn hu failure⟩
  | count enc major size short => exact ⟨rfl, rfl, .count enc major (by omega)⟩
  | depth enc major size fit deep =>
    have eq : d = 32 := by omega
    subst d
    exact ⟨rfl, rfl, .depth enc major (by omega)⟩
  | element enc major size fit depth failure => omega

/-- A specified failure that enters no map is exactly an opening rejection:
    no bytes are consumed, the error offset is zero, and the shared map-opening
    relation supplies the diagnostic. No entry failure can satisfy this shape. -/
theorem migratedClasses_zero_depth_failure_open (d consumed offset : Nat) (wire : List w8)
    (reason : MigratedClassesFailureReason) (depth : d ≤ 32)
    (failure : MigratedClassesSpec d wire (.failure consumed offset 0 reason)) :
    consumed = 0 ∧ offset = 0 ∧ StructOpenFailure d wire (migratedClassesDiagnostic reason) := by
  generalize hzero : (0 : Nat) = entered at failure
  cases failure with
  | outer hn hu failure => exact ⟨rfl, rfl, .outer hn hu failure⟩
  | count enc major size short => exact ⟨rfl, rfl, .count enc major (by omega)⟩
  | depth enc major size fit deep =>
    have eq : d = 32 := by omega
    subst d
    exact ⟨rfl, rfl, .depth enc major (by omega)⟩
  | element enc major size fit depth failure => omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltMap_zero_depth_failure_open

#print axioms Perennial.github_com.NethermindEth.juno.core.storageDiffs_zero_depth_failure_open

#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClasses_zero_depth_failure_open
