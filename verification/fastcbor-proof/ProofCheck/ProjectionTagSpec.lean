import ProofCheck.ExpectedDeterminism
import ProofCheck.ExpectedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The tag prelude inspects only the first major type, not the content item. -/
def projectionHasTag (wire : List w8) : Prop :=
  ∃ head tail, wire = head :: tail ∧ head &&& W8 224 = W8 192

def projectionTagRejected (tag : w64) (rest : List w8) : Prop :=
  uint.Z tag ≤ 3 ∨ projectionHasTag rest

inductive ProjectionTagStatus where
  | success
  | malformed (reason : ExpectedFailureReason)
  | unsupported

/-- Independent projection tag policy. Untagged input, including EOF, is left
    for the record reader. Exactly one nonreserved tag may be consumed; an absent
    content item is likewise left for that reader to diagnose. Invalid tag heads
    roll back, whereas reserved tags and tag chains consume the first tag head. -/
inductive ProjectionTagSpec : List w8 → Nat → List w8 → ProjectionTagStatus → Prop where
  | untagged {wire} (plain : ¬ projectionHasTag wire) : ProjectionTagSpec wire 0 wire .success
  | allowed {head payload rest tag} (enc : HeadArgument head payload tag)
      (major : head &&& W8 224 = W8 192) (allowed : ¬ projectionTagRejected tag rest) :
      ProjectionTagSpec ((head :: payload) ++ rest) (head :: payload).length rest .success
  | malformed {wire reason} (tagged : projectionHasTag wire) (failure : ExpectedFailure (W8 192) wire reason) :
      ProjectionTagSpec wire 0 wire (.malformed reason)
  | unsupported {head payload rest tag} (enc : HeadArgument head payload tag)
      (major : head &&& W8 224 = W8 192) (rejected : projectionTagRejected tag rest) :
      ProjectionTagSpec ((head :: payload) ++ rest) (head :: payload).length rest .unsupported

theorem projectionTag_classify (wire : List w8) :
    ∃ consumed rest status, ProjectionTagSpec wire consumed rest status := by
  classical
  by_cases tagged : projectionHasTag wire
  · rcases expected_classify (W8 192) wire with ⟨head, payload, rest, tag, split, enc, major⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases rejected : projectionTagRejected tag rest
      · exact ⟨_, _, _, .unsupported enc major rejected⟩
      · exact ⟨_, _, _, .allowed enc major rejected⟩
    · exact ⟨0, wire, _, .malformed tagged failure⟩
  · exact ⟨0, wire, _, .untagged tagged⟩

theorem projectionTag_suffix (wire rest : List w8) (consumed : Nat) (status : ProjectionTagStatus)
    (spec : ProjectionTagSpec wire consumed rest status) :
    ∃ item, wire = item ++ rest ∧ item.length = consumed := by
  cases spec with
  | untagged => exact ⟨[], rfl, rfl⟩
  | allowed => exact ⟨_, rfl, rfl⟩
  | malformed => exact ⟨[], rfl, rfl⟩
  | unsupported => exact ⟨_, rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionTag_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.projectionTag_suffix
