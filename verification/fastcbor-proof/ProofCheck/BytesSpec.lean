import ProofCheck.ExpectedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A definite CBOR byte string, with arbitrary payload and supported head width.
    No UTF-8 or JSON restriction is imposed on its contents. -/
inductive BytesValid : List w8 → List w8 → Prop where
  | bytes {head payload body argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
      (count : uint.Z argument = body.length) :
      BytesValid ((head :: payload) ++ body) body

inductive BytesFailureReason where
  | expected (reason : ExpectedFailureReason)
  | truncated

/-- Invalid heads and wrong major types precede payload truncation. -/
inductive BytesRejected : List w8 → BytesFailureReason → Prop where
  | expected {wire reason} (failure : ExpectedFailure (W8 64) wire reason) :
      BytesRejected wire (.expected reason)
  | truncated {head payload tail argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
      (short : tail.length < uint.Z argument) : BytesRejected ((head :: payload) ++ tail) .truncated

theorem bytes_classify (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ BytesValid item value) ∨
    (∃ reason, BytesRejected wire reason) := by
  rcases expected_classify (W8 64) wire with
    ⟨head, payload, tail, argument, split, enc, major⟩ | ⟨reason, failure⟩
  · subst wire
    by_cases short : tail.length < uint.Z argument
    · exact Or.inr ⟨.truncated, .truncated enc major short⟩
    · let body := tail.take (uint.Z argument).toNat
      let rest := tail.drop (uint.Z argument).toNat
      have count : uint.Z argument = body.length := by
        have nonneg : 0 ≤ uint.Z argument := by word
        simp only [body, List.length_take]
        rw [Nat.min_eq_left (by omega)]
        omega
      have splitBody : body ++ rest = tail := List.take_append_drop _ _
      exact Or.inl ⟨(head :: payload) ++ body, rest, body,
        by rw [List.append_assoc, splitBody], .bytes enc major count⟩
  · exact Or.inr ⟨.expected reason, .expected failure⟩

theorem bytesValid_nonempty (item value : List w8)
    (valid : BytesValid item value) : 0 < item.length := by cases valid; simp

theorem bytesValid_prefix_unique (left right a b restA restB : List w8)
    (first : BytesValid left a) (second : BytesValid right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | bytes enc major count =>
    cases second with
    | bytes enc2 major2 count2 =>
      obtain ⟨rfl, rfl, rfl, tails⟩ := encodedHead_prefix_unique _ _ _ _ (a ++ restA) (b ++ restB) _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      have length : a.length = b.length := by omega
      have bodies := congrArg (List.take a.length) tails
      rw [List.take_left, length, List.take_left] at bodies
      subst b
      have restEq := List.append_cancel_left tails
      subst restB
      exact ⟨rfl, rfl, rfl⟩

theorem bytesValid_not_rejected (item value rest : List w8) (reason : BytesFailureReason)
    (valid : BytesValid item value) (failure : BytesRejected (item ++ rest) reason) : False := by
  cases valid with
  | @bytes head payload body argument enc major count =>
    generalize same : (head :: payload) ++ value ++ rest = wire at failure
    cases failure with
    | expected failure =>
      exact expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major
        (by simpa only [List.append_assoc] using same.symm) failure
    | truncated enc2 major2 short =>
      obtain ⟨_, _, argEq, tails⟩ := encodedHead_prefix_unique _ _ _ _ (value ++ rest) _ _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      rw [← tails, ← argEq, count] at short
      simp only [List.length_append] at short
      omega

theorem bytesRejected_unique (wire : List w8) (a b : BytesFailureReason)
    (first : BytesRejected wire a) (second : BytesRejected wire b) : a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | expected failure =>
    cases second with
    | expected failure2 =>
      rw [← same] at failure2
      exact congrArg BytesFailureReason.expected (expectedFailure_reason_unique _ _ _ _ failure failure2)
    | truncated enc major short =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same failure)
  | truncated enc major short =>
    cases second with
    | expected failure =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same.symm failure)
    | truncated => rfl

inductive BytesResult where
  | success (value rest : List w8)
  | failure (reason : BytesFailureReason)

def BytesSpec (wire : List w8) : BytesResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ BytesValid item value
  | .failure reason => BytesRejected wire reason

theorem bytesSpec_unique (wire : List w8) (a b : BytesResult)
    (first : BytesSpec wire a) (second : BytesSpec wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := bytesValid_prefix_unique item item2 value value2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (bytesValid_not_rejected _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (bytesValid_not_rejected _ _ _ _ valid first)
    | failure reason2 => exact congrArg BytesResult.failure (bytesRejected_unique _ _ _ first second)

theorem bytesSpec_total_unique (wire : List w8) :
    ∃ outcome, BytesSpec wire outcome ∧ ∀ other, BytesSpec wire other → other = outcome := by
  have existsOutcome : ∃ outcome, BytesSpec wire outcome := by
    rcases bytes_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => bytesSpec_unique wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.bytes_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesSpec_total_unique
