import ProofCheck.StateDiffPairs
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A direct StateDiff record preserves old contents on null/undefined. Successful
    map decoding retains the actual suffix for contextual nested-array guards. -/
inductive StateDiffRecordValid (d : Nat) (old : StateDiffValue) : List w8 → List w8 → StateDiffValue → Prop where
  | nullish {head : w8} (suffix : List w8) (hn : head = W8 246 ∨ head = W8 247) :
      StateDiffRecordValid d old [head] suffix old
  | map {head payload body suffix argument final} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32)
      (valid : StateDiffPairsValid (d + 1) count [] old body suffix final) :
      StateDiffRecordValid d old ((head :: payload) ++ body) suffix final

/-- Opening errors consume nothing and preserve depth. Later failures retain
    the partial record and the outer map and failed child levels; there is no record rollback. -/
inductive StateDiffRecordRejected (d : Nat) (old : StateDiffValue) : List w8 → StateDiffValue →
    Nat → Nat → Nat → StateDiffFailureReason → Prop where
  | opening {wire reason} (failure : StructOpenFailure d wire reason) :
      StateDiffRecordRejected d old wire old 0 0 0 (.opening reason)
  | map {head payload body argument final consumed offset extraDepth reason} {count : Nat}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (size : uint.Z argument = (count : Int)) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : StateDiffPairsRejected (d + 1) count [] old body final consumed offset extraDepth reason) :
      StateDiffRecordRejected d old ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) (1 + extraDepth) reason

/-- All inputs are classified, including arbitrary field orders, unknown keys,
    duplicate known keys and failures after any successful prefix. -/
theorem stateDiffRecord_classify (d : Nat) (old : StateDiffValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ StateDiffRecordValid d old item rest final) ∨
    (∃ final consumed offset extraDepth reason, StateDiffRecordRejected d old wire final consumed offset extraDepth reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, old, split, .nullish tail nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have size : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases stateDiffPairs_classify (d + 1) count [] old body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major size hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, 1 + extraDepth, reason,
        .map enc major size hd countFit failure⟩
  · exact Or.inr ⟨old, 0, 0, 0, _, .opening failure⟩

theorem stateDiffRecord_nonempty (d : Nat) (old final : StateDiffValue) (item rest : List w8)
    (valid : StateDiffRecordValid d old item rest final) : 0 < item.length := by
  cases valid <;> simp

theorem stateDiffRecordRejected_bounds (d consumed offset : Nat) (old final : StateDiffValue) (wire : List w8)
    (extraDepth : Nat) (reason : StateDiffFailureReason)
    (failure : StateDiffRecordRejected d old wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  cases failure with
  | opening => omega
  | map enc major size depth fit failure =>
    have hb := stateDiffPairsRejected_bounds _ _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem stateDiffRecordRejected_depth (d consumed offset extraDepth : Nat) (old final : StateDiffValue) (wire : List w8)
    (reason : StateDiffFailureReason) (depth : d ≤ 32)
    (failure : StateDiffRecordRejected d old wire final consumed offset extraDepth reason) : d + extraDepth ≤ 32 := by
  cases failure with
  | opening => omega
  | map enc major size inside fit failure =>
    have hb := stateDiffPairsRejected_depth _ _ _ _ _ _ _ _ _ _ (by omega) failure
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffRecord_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffRecord_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffRecordRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.stateDiffRecordRejected_depth
