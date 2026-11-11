import ProofCheck.ExpectedFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- BytesInto accepts exactly one definite byte string matching the destination
    length. Unlike the typed address wrapper, it does not accept nullish input. -/
inductive BytesIntoValid (size : Nat) : List w8 → List w8 → Prop where
  | bytes {head payload body argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
      (count : uint.Z argument = body.length) (length : body.length = size) :
      BytesIntoValid size ((head :: payload) ++ body) body

inductive BytesIntoFailureReason where
  | expected (reason : ExpectedFailureReason)
  | truncated
  | size (actual : Nat)

/-- Payload truncation precedes destination-width checking. A complete but
    differently sized payload is rejected without copying or retaining progress. -/
inductive BytesIntoRejected (size : Nat) : List w8 → BytesIntoFailureReason → Prop where
  | expected {wire reason} (failure : ExpectedFailure (W8 64) wire reason) :
      BytesIntoRejected size wire (.expected reason)
  | truncated {head payload tail argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
      (short : tail.length < uint.Z argument) : BytesIntoRejected size ((head :: payload) ++ tail) .truncated
  | size {head payload body rest argument}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 64)
      (count : uint.Z argument = body.length) (different : body.length ≠ size) :
      BytesIntoRejected size ((head :: payload) ++ body ++ rest) (.size body.length)

theorem bytesInto_classify (size : Nat) (wire : List w8) :
    (∃ item rest value, wire = item ++ rest ∧ BytesIntoValid size item value) ∨
    (∃ reason, BytesIntoRejected size wire reason) := by
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
      by_cases length : body.length = size
      · exact Or.inl ⟨(head :: payload) ++ body, rest, body,
          by rw [List.append_assoc, splitBody], .bytes enc major count length⟩
      · apply Or.inr
        refine ⟨.size body.length, ?_⟩
        rw [← splitBody, ← List.append_assoc]
        exact .size enc major count length
  · exact Or.inr ⟨.expected reason, .expected failure⟩

theorem bytesIntoValid_nonempty (size : Nat) (item value : List w8)
    (valid : BytesIntoValid size item value) : 0 < item.length := by cases valid; simp

theorem bytesIntoValid_prefix_unique (size : Nat) (left right a b restA restB : List w8)
    (first : BytesIntoValid size left a) (second : BytesIntoValid size right b)
    (same : left ++ restA = right ++ restB) : left = right ∧ a = b ∧ restA = restB := by
  cases first with
  | bytes enc major count length =>
    cases second with
    | bytes enc2 major2 count2 length2 =>
      obtain ⟨rfl, rfl, rfl, tails⟩ := encodedHead_prefix_unique _ _ _ _ (a ++ restA) (b ++ restB) _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      have bodies := congrArg (List.take size) tails
      rw [← length, List.take_left] at bodies
      rw [length, ← length2, List.take_left] at bodies
      subst b
      have restEq := List.append_cancel_left tails
      subst restB
      exact ⟨rfl, rfl, rfl⟩

theorem bytesIntoValid_not_rejected (size : Nat) (item value rest : List w8) (reason : BytesIntoFailureReason)
    (valid : BytesIntoValid size item value) (failure : BytesIntoRejected size (item ++ rest) reason) : False := by
  cases valid with
  | @bytes head payload body argument enc major count length =>
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
    | size enc2 major2 count2 different =>
      obtain ⟨_, _, argEq, _⟩ := encodedHead_prefix_unique _ _ _ _ (value ++ rest) _ _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      rw [← argEq, count] at count2
      exact different (by omega)

theorem bytesIntoRejected_unique (size : Nat) (wire : List w8) (a b : BytesIntoFailureReason)
    (first : BytesIntoRejected size wire a) (second : BytesIntoRejected size wire b) : a = b := by
  generalize same : wire = wire2 at second
  cases first with
  | expected failure =>
    cases second with
    | expected failure2 =>
      rw [← same] at failure2
      exact congrArg BytesIntoFailureReason.expected (expectedFailure_reason_unique _ _ _ _ failure failure2)
    | truncated enc major short =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same failure)
    | size enc major count different =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major
        (by simpa only [List.append_assoc] using same) failure)
  | truncated enc major short =>
    cases second with
    | expected failure =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major same.symm failure)
    | truncated => rfl
    | size enc2 major2 count2 different =>
      obtain ⟨_, _, argEq, tails⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      rw [tails, argEq, count2] at short
      simp only [List.length_append] at short
      omega
  | size enc major count different =>
    cases second with
    | expected failure =>
      exact False.elim (expectedFailure_excludes_encoded _ _ _ _ _ _ _ enc major
        (by simpa only [List.append_assoc] using same.symm) failure)
    | truncated enc2 major2 short =>
      obtain ⟨_, _, argEq, tails⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      rw [← tails, ← argEq, count] at short
      simp only [List.length_append] at short
      omega
    | size enc2 major2 count2 different2 =>
      obtain ⟨_, _, argEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using same)
      rw [← argEq, count] at count2
      congr 1
      omega

inductive BytesIntoResult where
  | success (value rest : List w8)
  | failure (reason : BytesIntoFailureReason)

def BytesIntoSpec (size : Nat) (wire : List w8) : BytesIntoResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ BytesIntoValid size item value
  | .failure reason => BytesIntoRejected size wire reason

theorem bytesIntoSpec_unique (size : Nat) (wire : List w8) (a b : BytesIntoResult)
    (first : BytesIntoSpec size wire a) (second : BytesIntoSpec size wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := bytesIntoValid_prefix_unique size item item2 value value2 rest rest2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (bytesIntoValid_not_rejected _ _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (bytesIntoValid_not_rejected _ _ _ _ _ valid first)
    | failure reason2 => exact congrArg BytesIntoResult.failure (bytesIntoRejected_unique _ _ _ _ first second)

theorem bytesIntoSpec_total_unique (size : Nat) (wire : List w8) :
    ∃ outcome, BytesIntoSpec size wire outcome ∧ ∀ other, BytesIntoSpec size wire other → other = outcome := by
  have existsOutcome : ∃ outcome, BytesIntoSpec size wire outcome := by
    rcases bytesInto_classify size wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => bytesIntoSpec_unique size wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesInto_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoValid_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoValid_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bytesIntoSpec_total_unique
