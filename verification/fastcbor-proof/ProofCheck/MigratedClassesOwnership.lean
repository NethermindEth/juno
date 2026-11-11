import Perennial.Golang.Theory.Map
import ProofCheck.MigratedClassesValues
import ProofCheck.FeltValid

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Both keys and values are inline four-word felts. There are no heap-owned
    felt children; this predicate does not assume any Go key equality rule. -/
abbrev MigratedClassesOwned (out : Loc) (values : GMap FeltMapKey FeltMapKey) : IProp GF :=
  iprop(out ↦$ values)

/-- Nil and allocated-empty maps remain distinct. -/
def MigratedClassesResult (out : Loc) (values : Option (GMap FeltMapKey FeltMapKey)) : IProp GF :=
  match values with
  | none => iprop(⌜out = map.nil⌝)
  | some values => MigratedClassesOwned out values

theorem migratedClassesOwned_nonnull (out : Loc) (values : GMap FeltMapKey FeltMapKey) :
    MigratedClassesOwned out values ⊢ (⌜out ≠ map.nil⌝ : IProp GF) := by
  iintro Hmap
  iapply ownMap_not_nil $$ Hmap

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.migratedClassesOwned_nonnull
