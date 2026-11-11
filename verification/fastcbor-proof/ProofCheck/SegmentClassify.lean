import ProofCheck.SegmentSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Coverage properties used only to structure the finite-depth induction. -/
def SegmentRecordClassified (d : Nat) : Prop := ∀ (old : SegmentValue) (wire : List w8),
    (∃ item rest final, wire = item ++ rest ∧ SegmentRecordValid d old item rest final) ∨
    (∃ final consumed offset extraDepth reason, SegmentRecordRejected d old wire final consumed offset extraDepth reason)

def SegmentArrayClassified (d : Nat) : Prop := ∀ wire : List w8, ∃ outcome, SegmentArraySpec d wire outcome

theorem segmentArray_success_suffix (d : Nat) (wire rest : List w8) (values : Option (List SegmentValue))
    (spec : SegmentArraySpec d wire (.success values rest)) :
    ∃ item, wire = item ++ rest ∧ 0 < item.length := by
  cases spec with
  | nullish head tail hn => exact ⟨[head], rfl, by simp⟩
  | array enc major size fit depth valid => exact ⟨_, rfl, by simp⟩

/-- Field classification delegates Children to the same-depth array relation.
    The final simultaneous induction below supplies that coverage property. -/
theorem segmentField_classify_of_array (d : Nat) (arrays : SegmentArrayClassified d)
    (field : SegmentField) (old : SegmentValue) (wire : List w8) :
    (∃ item rest final, wire = item ++ rest ∧ SegmentFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, SegmentFieldRejected d field wire consumed offset extraDepth reason) := by
  cases field with
  | children =>
    obtain ⟨outcome, spec⟩ := arrays wire
    cases outcome with
    | success value rest =>
      obtain ⟨item, split, positive⟩ := segmentArray_success_suffix d wire rest value spec
      rw [split] at spec
      exact Or.inl ⟨item, rest, _, split, .children spec⟩
    | failure consumed offset extraDepth reason => exact Or.inr ⟨consumed, offset, extraDepth, _, .children spec⟩
  | length =>
    rcases scalar_classify old.length wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact Or.inl ⟨item, rest, _, split, .length valid⟩
    · exact Or.inr ⟨0, 0, 0, _, .length failure⟩

theorem segmentValue_classify_of_array (d : Nat) (arrays : SegmentArrayClassified d) (choice : Option SegmentField) (old : SegmentValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SegmentValueValid d choice old item rest final) ∨
    (∃ final consumed offset extraDepth reason, SegmentValueRejected d choice old wire final consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored valid⟩
    · exact Or.inr ⟨old, 0, offset, 0, _, .ignored failure⟩
  | some field =>
    rcases segmentField_classify_of_array d arrays field old wire with ⟨item, rest, final, split, valid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected valid⟩
    · exact Or.inr ⟨old, consumed, offset, extraDepth, _, .selected failure⟩


theorem segmentPairs_classify_of_array (d : Nat) (arrays : SegmentArrayClassified d) (n : Nat) (seen : List SegmentField) (old : SegmentValue) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SegmentPairsValid d n seen old item rest final) ∨
    (∃ final consumed offset extraDepth reason, SegmentPairsRejected d n seen old wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old wire⟩
  | succ n ih =>
    rcases segmentKey_classify wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : SegmentChoiceFresh seen choice
      · rcases segmentValue_classify_of_array d arrays choice old tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨retained, consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (segmentChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step keyValid fresh valueValid validRest⟩
          · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using SegmentPairsRejected.next keyValid fresh valueValid failure
        · exact Or.inr ⟨retained, key.length + consumed, key.length + offset, extraDepth, _, .value keyValid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨old, key.length, key.length, 0, _,
            .duplicate keyValid (by simpa only [SegmentChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨old, 0, 0, 0, _, .key failure⟩


theorem segmentRecord_classify_of_array (d : Nat) (depth : d ≤ 32)
    (next : d < 32 → SegmentArrayClassified (d + 1)) : SegmentRecordClassified d := by
  intro old wire
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish tail nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases segmentPairs_classify_of_array (d + 1) (next hd) count [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major size hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, 1 + extraDepth, reason,
        .map enc major size hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, 0, _, .opening failure⟩


theorem segmentItems_classify_of_record (d : Nat) (records : SegmentRecordClassified d) (n : Nat) (wire : List w8) :
    (∃ item rest values, wire = item ++ rest ∧ SegmentItemsValid d n item rest values) ∨
    (∃ done retained consumed offset childDepth reason,
      SegmentItemsRejected d n wire done retained consumed offset childDepth reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, [], rfl, .nil wire⟩
  | succ n ih =>
    rcases records segmentZero wire with
      ⟨item, tail, value, split, valid⟩ | ⟨retained, consumed, offset, childDepth, reason, failure⟩
    · subst wire
      rcases ih tail with ⟨items, rest, values, split, restValid⟩ |
        ⟨done, retained, consumed, offset, childDepth, reason, failure⟩
      · subst tail
        exact Or.inl ⟨item ++ items, rest, value :: values, (List.append_assoc _ _ _).symm, .step valid restValid⟩
      · exact Or.inr ⟨value :: done, retained, item.length + consumed, item.length + offset,
          childDepth, reason, .next valid failure⟩
    · exact Or.inr ⟨[], retained, consumed, offset, childDepth, reason, .here failure⟩


theorem segmentArray_classify_of_record (d : Nat) (depth : d ≤ 32)
    (next : d < 32 → SegmentRecordClassified (d + 1)) : SegmentArrayClassified d := by
  intro wire
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail => exact ⟨_, .nullish head tail (by simpa using nullish)⟩
  have hn : wire.head? ≠ some (W8 246) := fun eq => nullish (Or.inl eq)
  have hu : wire.head? ≠ some (W8 247) := fun eq => nullish (Or.inr eq)
  rcases expected_classify (W8 128) wire with
    ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    by_cases short : tail.length < count
    · exact ⟨_, .count enc major size short⟩
    have fit : count ≤ tail.length := by omega
    by_cases deep : 32 ≤ d
    · exact ⟨_, .depth enc major size fit deep⟩
    have outer : d < 32 := by omega
    rcases segmentItems_classify_of_record (d + 1) (next outer) count tail with
      ⟨item, rest, values, split, valid⟩ | ⟨done, retained, consumed, offset, childDepth, reason, failure⟩
    · subst tail
      refine ⟨.success (some values) rest, ?_⟩
      simpa only [List.append_assoc] using SegmentArraySpec.array enc major size fit outer valid
    · exact ⟨_, .element enc major size fit outer failure⟩
  · exact ⟨_, .outer hn hu failure⟩


/-- Both recursive relations classify every input. Only the remaining nesting
    budget decreases between the record and array induction obligations. -/
theorem segment_classify_depth (fuel : Nat) : ∀ d : Nat, d ≤ 32 → 32 - d = fuel →
    SegmentRecordClassified d ∧ SegmentArrayClassified d := by
  induction fuel with
  | zero =>
    intro d depth remaining
    have impossible : ¬ d < 32 := by omega
    exact ⟨segmentRecord_classify_of_array d depth (fun inside => False.elim (impossible inside)),
      segmentArray_classify_of_record d depth (fun inside => False.elim (impossible inside))⟩
  | succ fuel ih =>
    intro d depth remaining
    have next : d < 32 → SegmentRecordClassified (d + 1) ∧ SegmentArrayClassified (d + 1) := by
      intro inside
      exact ih (d + 1) (by omega) (by omega)
    exact ⟨segmentRecord_classify_of_array d depth (fun inside => (next inside).2),
      segmentArray_classify_of_record d depth (fun inside => (next inside).1)⟩

theorem segmentRecord_classify (d : Nat) (old : SegmentValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SegmentRecordValid d old item rest final) ∨
    (∃ final consumed offset extraDepth reason, SegmentRecordRejected d old wire final consumed offset extraDepth reason) :=
  (segment_classify_depth (32 - d) d depth rfl).1 old wire

theorem segmentArray_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    ∃ outcome, SegmentArraySpec d wire outcome :=
  (segment_classify_depth (32 - d) d depth rfl).2 wire

theorem segmentField_classify (d : Nat) (field : SegmentField) (old : SegmentValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ SegmentFieldValid d field old item rest final) ∨
    (∃ consumed offset extraDepth reason, SegmentFieldRejected d field wire consumed offset extraDepth reason) :=
  segmentField_classify_of_array d (segment_classify_depth (32 - d) d depth rfl).2 field old wire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_success_suffix
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_classify_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentValue_classify_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentPairs_classify_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_classify_of_array
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentItems_classify_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_classify_of_record
#print axioms Perennial.github_com.NethermindEth.juno.core.segment_classify_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentRecord_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentArray_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.segmentField_classify
