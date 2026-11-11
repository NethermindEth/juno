import ProofCheck.ClassMetadata

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
/-- The two application-level tagged class alternatives. -/
inductive ClassVariant where
  | deprecated | sierra
  deriving DecidableEq

def classVariantTag (tags : CBORClassTags) : ClassVariant → w64
  | .deprecated => tags.DeprecatedCairoClass'
  | .sierra => tags.SierraClass'

/-- The direct API's documented caller requirement; registration validates it
    outside this decoder proof boundary. -/
def ClassTagsDistinct (tags : CBORClassTags) : Prop :=
  ∀ a b : ClassVariant, a ≠ b → classVariantTag tags a ≠ classVariantTag tags b

def ClassTagMeaning (tags : CBORClassTags) (tag : w64) : Option ClassVariant → Prop
  | some variant => tag = classVariantTag tags variant
  | none => ∀ variant, tag ≠ classVariantTag tags variant

/-- The abstract profile requirement is exactly the two-field inequality
    checked by the registration adapter outside this proof boundary. -/
theorem classTagsDistinct_iff (tags : CBORClassTags) :
    ClassTagsDistinct tags ↔ tags.DeprecatedCairoClass' ≠ tags.SierraClass' := by
  constructor
  · intro distinct
    exact distinct .deprecated .sierra (by decide)
  · intro different a b ne
    cases a <;> cases b
    · exact False.elim (ne rfl)
    · exact different
    · exact Ne.symm different
    · exact False.elim (ne rfl)

theorem classVariantTag_injective (tags : CBORClassTags) (distinct : ClassTagsDistinct tags)
    (a b : ClassVariant) (same : classVariantTag tags a = classVariantTag tags b) : a = b := by
  by_cases eq : a = b
  · exact eq
  · exact False.elim (distinct a b eq same)

theorem classTagMeaning_total (tags : CBORClassTags) (tag : w64) :
    ∃ choice, ClassTagMeaning tags tag choice := by
  classical
  by_cases known : ∃ variant, tag = classVariantTag tags variant
  · obtain ⟨variant, found⟩ := known
    exact ⟨some variant, found⟩
  · exact ⟨none, fun variant same => known ⟨variant, same⟩⟩

theorem classTagMeaning_unique (tags : CBORClassTags) (tag : w64)
    (distinct : ClassTagsDistinct tags) (a b : Option ClassVariant)
    (first : ClassTagMeaning tags tag a) (second : ClassTagMeaning tags tag b) : a = b := by
  cases a with
  | none => cases b with
    | none => rfl
    | some variant => exact False.elim (first variant second)
  | some variant => cases b with
    | none => exact False.elim (second variant first)
    | some other => exact congrArg some (classVariantTag_injective tags distinct variant other (first.symm.trans second))

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.classVariantTag_injective
#print axioms Perennial.github_com.NethermindEth.juno.core.classTagMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.classTagMeaning_unique

#print axioms Perennial.github_com.NethermindEth.juno.core.classTagsDistinct_iff
