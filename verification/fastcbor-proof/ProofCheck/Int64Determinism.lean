import ProofCheck.Int64Spec
import ProofCheck.KeyFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Signed-integer rejection determines a unique diagnostic, including invalid
    heads, truncation, tags, type mismatch and either-sign overflow. -/
theorem int64Rejected_reason_unique (wire : List w8) (a b : Int64FailureReason)
    (first : Int64Rejected wire a) (second : Int64Rejected wire b) : a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @head head reason tail fault =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      exact congrArg (Int64FailureReason.head head) (headFault_reason_unique head reason reason2 fault fault2)
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault
      omega
    | eof =>
      simp [List.cons_append] at eqWire
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
  | @incomplete head tail size info sizeEq short =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault2
      omega
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      rfl
    | eof =>
      simp [List.cons_append] at eqWire
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
  | eof =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      simp [List.cons_append] at eqWire
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp [List.cons_append] at eqWire
    | eof =>
      rfl
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      simp [List.cons_append] at eqWire
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      simp [List.cons_append] at eqWire
  | @overflow head payload argument tail enc major bound =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      rfl
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
  | @tag head payload argument tail enc major =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      rfl
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
  | @typeMismatch head payload argument tail enc uint negative tag =>
    cases second with
    | @head head2 reason2 tail2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @overflow head2 payload2 argument2 tail2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 argument2 tail2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 argument2 tail2 enc2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      rfl
/-- A complete, in-range integer head excludes every signed-decoder failure,
    independently of trailing input and of the chosen sign. -/
theorem int64Encoded_not_rejected (head : w8) (payload rest : List w8) (argument : w64)
    (reason : Int64FailureReason) (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32)
    (bound : uint.Z argument < 2 ^ 63)
    (failure : Int64Rejected ((head :: payload) ++ rest) reason) : False := by
  generalize hwire : (head :: payload) ++ rest = wire at failure
  cases failure with
  | head tail fault =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload rest _
      argument _ enc rfl hwire.symm (.head tail fault)
  | incomplete tail info sizeEq short =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload rest _
      argument _ enc rfl hwire.symm (.incomplete tail info sizeEq short)
  | eof => simp at hwire
  | @overflow other payload2 argument2 tail enc2 major2 overflow =>
    obtain ⟨_, _, rfl, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 hwire
    omega
  | tag tail enc2 major2 =>
    simp only [List.cons_append, List.cons.injEq] at hwire
    obtain ⟨rfl, _⟩ := hwire
    rcases major with hm | hm <;> word
  | typeMismatch tail enc2 unsigned negative tag =>
    simp only [List.cons_append, List.cons.injEq] at hwire
    obtain ⟨rfl, _⟩ := hwire
    exact major.elim unsigned negative

theorem int64Valid_not_rejected (item rest : List w8) (value : Int) (reason : Int64FailureReason)
    (valid : Int64Valid item value) (failure : Int64Rejected (item ++ rest) reason) : False := by
  cases valid with
  | unsigned enc major bound => exact int64Encoded_not_rejected _ _ _ _ _ enc (Or.inl major) bound failure
  | negative enc major bound => exact int64Encoded_not_rejected _ _ _ _ _ enc (Or.inr major) bound failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Rejected_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Encoded_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.int64Valid_not_rejected
