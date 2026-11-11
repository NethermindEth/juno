import ProofCheck.ClassTagProfile
import ProofCheck.DeprecatedClassRecordUnique
import ProofCheck.SierraClassRecordUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Each payload preserves its independent record result, including private
    partial state and exact cursor/depth increments on failure. -/
inductive ClassPayloadResult where
  | deprecated (result : DeprecatedClassRecordResult)
  | sierra (result : SierraClassRecordResult)

def classPayloadVariant : ClassPayloadResult → ClassVariant
  | .deprecated _ => .deprecated
  | .sierra _ => .sierra

def ClassPayloadSpec (d : Nat) (wire : List w8) : ClassPayloadResult → Prop
  | .deprecated result => DeprecatedClassRecordSpec d deprecatedClassZero wire result
  | .sierra result => SierraClassRecordSpec d sierraClassZero wire result

theorem classPayload_classify (d : Nat) (wire : List w8) (variant : ClassVariant) (depth : d ≤ 32) :
    ∃ result, classPayloadVariant result = variant ∧ ClassPayloadSpec d wire result := by
  cases variant with
  | deprecated =>
    obtain ⟨result, spec, _⟩ := deprecatedClassRecordSpec_total_unique d deprecatedClassZero wire depth
    exact ⟨.deprecated result, rfl, spec⟩
  | sierra =>
    obtain ⟨result, spec, _⟩ := sierraClassRecordSpec_total_unique d sierraClassZero wire depth
    exact ⟨.sierra result, rfl, spec⟩

theorem classPayloadSpec_unique (d : Nat) (wire : List w8) (a b : ClassPayloadResult)
    (variant : classPayloadVariant a = classPayloadVariant b)
    (first : ClassPayloadSpec d wire a) (second : ClassPayloadSpec d wire b) : a = b := by
  cases a <;> cases b <;> simp only [classPayloadVariant] at variant <;> try contradiction
  · exact congrArg ClassPayloadResult.deprecated (deprecatedClassRecordSpec_unique d deprecatedClassZero wire _ _ first second)
  · exact congrArg ClassPayloadResult.sierra (sierraClassRecordSpec_unique d sierraClassZero wire _ _ first second)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classPayload_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.classPayloadSpec_unique
