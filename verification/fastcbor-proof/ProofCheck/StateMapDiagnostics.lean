import ProofCheck.FeltMapSpec
import ProofCheck.StorageDiffsSpec
import ProofCheck.MigratedClassesSpec
import ProofCheck.FeltComplete

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

/-- Map diagnostics retain key/value distinctions in the independent reason;
    the owned error itself is the existing cursor diagnostic at its exact offset. -/
def feltMapDiagnostic : FeltMapFailureReason → FeltFailureReason
  | .expected reason => .expected (W8 160) reason
  | .count => .skipped .count
  | .depth => .depth
  | .element (.key reason) => reason
  | .element (.value reason) => reason

def migratedClassesDiagnostic : MigratedClassesFailureReason → FeltFailureReason
  | .expected reason => .expected (W8 160) reason
  | .count => .skipped .count
  | .depth => .depth
  | .element (.key reason) => reason
  | .element (.value reason) => reason

def storageDiffsDiagnostic : StorageDiffsFailureReason → FeltFailureReason
  | .expected reason => .expected (W8 160) reason
  | .count => .skipped .count
  | .depth => .depth
  | .element (.key reason) => reason
  | .element (.value reason) => feltMapDiagnostic reason

def FeltMapFailureOwned (err : Loc) (pos : w64) (reason : FeltMapFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
    FeltFailureError err pos (feltMapDiagnostic reason) (feltFailureKind (feltMapDiagnostic reason) kinds))

def MigratedClassesFailureOwned (err : Loc) (pos : w64) (reason : MigratedClassesFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
    FeltFailureError err pos (migratedClassesDiagnostic reason) (feltFailureKind (migratedClassesDiagnostic reason) kinds))

def StorageDiffsFailureOwned (err : Loc) (pos : w64) (reason : StorageDiffsFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗
    FeltFailureError err pos (storageDiffsDiagnostic reason) (feltFailureKind (storageDiffsDiagnostic reason) kinds))

end github_com.NethermindEth.juno.core
end Perennial
