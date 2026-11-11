import ProofCheck.ClassDispatchUnique

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Application class values distinguish a nil interface from an allocated
    zero record of any concrete variant. -/
inductive ClassValue where
  | nullish
  | deprecated (value : DeprecatedClassValue)
  | sierra (value : SierraClassValue)

/-- Failed outcomes retain the exact independent dispatch state and its absolute
    increments from the initial cursor. No failed record is published. -/
inductive ClassResult where
  | success (value : ClassValue) (rest : List w8)
  | failure (detail : ClassDispatchResult) (consumed offset extraDepth : Nat)

def classResult : ClassDispatchResult → ClassResult
  | .nullish rest => .success .nullish rest
  | .tagFailure reason => .failure (.tagFailure reason) 0 0 0
  | .unknown tag consumed rest => .failure (.unknown tag consumed rest) consumed consumed 0
  | .payload tagConsumed (.deprecated (.success value rest)) => .success (.deprecated value) rest
  | .payload tagConsumed (.deprecated (.failure retained consumed offset extraDepth reason)) =>
      .failure (.payload tagConsumed (.deprecated (.failure retained consumed offset extraDepth reason)))
        (tagConsumed + consumed) (tagConsumed + offset) extraDepth
  | .payload tagConsumed (.sierra (.success value rest)) => .success (.sierra value) rest
  | .payload tagConsumed (.sierra (.failure retained consumed offset extraDepth reason)) =>
      .failure (.payload tagConsumed (.sierra (.failure retained consumed offset extraDepth reason)))
        (tagConsumed + consumed) (tagConsumed + offset) extraDepth

def ClassSpec (d : Nat) (tags : CBORClassTags) (wire : List w8) (result : ClassResult) : Prop :=
  ∃ raw, ClassDispatchSpec d tags wire raw ∧ classResult raw = result

theorem classSpec_unique (d : Nat) (tags : CBORClassTags) (wire : List w8)
    (distinct : ClassTagsDistinct tags) (a b : ClassResult)
    (first : ClassSpec d tags wire a) (second : ClassSpec d tags wire b) : a = b := by
  obtain ⟨raw, spec, rfl⟩ := first
  obtain ⟨raw2, spec2, rfl⟩ := second
  obtain rfl := classDispatchSpec_unique d tags wire distinct raw raw2 spec spec2
  rfl

theorem classSpec_total_unique (d : Nat) (tags : CBORClassTags) (wire : List w8)
    (depth : d ≤ 32) (distinct : ClassTagsDistinct tags) :
    ∃ result, ClassSpec d tags wire result ∧ ∀ other, ClassSpec d tags wire other → other = result := by
  obtain ⟨raw, spec⟩ := classDispatch_classify d tags wire depth
  refine ⟨classResult raw, ⟨raw, spec, rfl⟩, ?_⟩
  intro other otherSpec
  exact classSpec_unique d tags wire distinct other _ otherSpec ⟨raw, spec, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.classSpec_total_unique
