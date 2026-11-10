import ProofCheck.SkipFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- If both parses fail at the next item, deterministic successful prefixes
    must end at the same byte boundary. Otherwise the longer prefix supplies
    a successful next item at the shorter prefix's failure boundary. -/
theorem wireFailure_prefix_unique {Item : Type} (bytes : Item → List w8)
    (left right : List Item) (tailA tailB : List w8)
    (unique : ∀ a ∈ left, ∀ b ∈ right, ∀ restA restB,
      bytes a ++ restA = bytes b ++ restB → bytes a = bytes b ∧ restA = restB)
    (rejectA : ∀ item ∈ right, ∀ suffix, tailA = bytes item ++ suffix → False)
    (rejectB : ∀ item ∈ left, ∀ suffix, tailB = bytes item ++ suffix → False)
    (same : left.flatMap bytes ++ tailA = right.flatMap bytes ++ tailB) :
    left.flatMap bytes = right.flatMap bytes ∧ tailA = tailB := by
  by_cases length : left.length = right.length
  · have result := wireSequence_prefix_unique bytes (fun _ => ()) left right tailA tailB length
      (fun a ha b hb restA restB split => by
        obtain ⟨wire, rest⟩ := unique a ha b hb restA restB split
        exact ⟨wire, rfl, rest⟩) same
    exact ⟨result.1, result.2.2⟩
  · by_cases shorter : left.length < right.length
    · obtain ⟨item, suffix, member, split⟩ := wireSequence_after_prefix bytes left right tailA tailB shorter
        (fun a ha b hb restA restB split => (unique a ha b hb restA restB split).2) same
      exact False.elim (rejectA item member suffix split)
    · have shorter : right.length < left.length := by omega
      obtain ⟨item, suffix, member, split⟩ := wireSequence_after_prefix bytes right left tailB tailA shorter
        (fun b hb a ha restB restA split => (unique a ha b hb restA restB split.symm).2.symm) same.symm
      exact False.elim (rejectB item member suffix split)

/-- Two failures after unsigned limbs have exactly the same successful prefix
    and unread failing input, regardless of each limb's encoded width. -/
theorem unsignedFailure_prefix_unique (left right : List UnsignedWire) (tailA tailB : List w8)
    (a b : ExpectedFailureReason) (first : ExpectedFailure (W8 0) tailA a)
    (second : ExpectedFailure (W8 0) tailB b)
    (same : unsignedBytes left ++ tailA = unsignedBytes right ++ tailB) :
    unsignedBytes left = unsignedBytes right ∧ tailA = tailB := by
  apply wireFailure_prefix_unique UnsignedWire.bytes left right tailA tailB
  · intro x _ y _ restX restY split
    have result := unsignedWire_prefix_unique x y restX restY split
    exact ⟨result.1, result.2.2⟩
  · intro item _ suffix split
    exact expectedFailure_excludes_encoded (W8 0) item.head item.payload suffix tailA item.value a
      item.argument item.major split first
  · intro item _ suffix split
    exact expectedFailure_excludes_encoded (W8 0) item.head item.payload suffix tailB item.value b
      item.argument item.major split second
  · exact same

/-- Two recursive Skip failures after valid siblings have the same prefix bytes
    and failing suffix; diagnostic equality is supplied by skipFailure_unique. -/
theorem skipFailure_prefix_unique (d : Nat) (left right : List (List w8)) (tailA tailB : List w8)
    (offsetA offsetB : Nat) (a b : SkipFailureReason)
    (leftValid : ∀ item ∈ left, SkipValid d item) (rightValid : ∀ item ∈ right, SkipValid d item)
    (first : SkipFailure d tailA offsetA a) (second : SkipFailure d tailB offsetB b)
    (same : left.flatten ++ tailA = right.flatten ++ tailB) :
    left.flatten = right.flatten ∧ tailA = tailB := by
  have result := wireFailure_prefix_unique (id : List w8 → List w8) left right tailA tailB
    (fun x hx y hy restX restY split => skipValid_prefix_unique d x y restX restY
      (leftValid x hx) (rightValid y hy) split) (by
        intro item member suffix split
        subst tailA
        exact skipValid_not_failure d offsetA item suffix a (rightValid item member) first) (by
        intro item member suffix split
        subst tailB
        exact skipValid_not_failure d offsetB item suffix b (leftValid item member) second)
    (by simpa only [List.flatMap_id] using same)
  simpa only [List.flatMap_id] using result

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wireFailure_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedFailure_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_prefix_unique
