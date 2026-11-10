import ProofCheck.HeaderDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Unsupported indefinite heads, break codes and invalid additional information
    are mutually exclusive diagnostic classes. -/
theorem headFault_reason_unique (head : w8) (a b : HeadError)
    (first : HeadFault head a) (second : HeadFault head b) : a = b := by
  cases first with
  | indefinite info major =>
    cases second with
    | indefinite => rfl
    | breakCode info2 major2 => word
    | invalid info2 notIndefinite notBreak => exact False.elim (notIndefinite ⟨info, major⟩)
  | breakCode info major =>
    cases second with
    | indefinite info2 major2 => word
    | breakCode => rfl
    | invalid info2 notIndefinite notBreak => exact False.elim (notBreak ⟨info, major⟩)
  | invalid info notIndefinite notBreak =>
    cases second with
    | indefinite info2 major2 => exact False.elim (notIndefinite ⟨info2, major2⟩)
    | breakCode info2 major2 => exact False.elim (notBreak ⟨info2, major2⟩)
    | invalid => rfl

/-- The same requested type and byte input have one expectation diagnostic,
    including the exact offending head byte in head/type errors. -/
theorem expectedFailure_reason_unique (want : w8) (wire : List w8) (a b : ExpectedFailureReason)
    (first : ExpectedFailure want wire a) (second : ExpectedFailure want wire b) : a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @head head tail reason fault =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      simp only [List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      exact congrArg (ExpectedFailureReason.head head) (headFault_reason_unique head reason reason2 fault fault2)
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault
      omega
    | eof =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 tail2 argument2 enc2 major2 notTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl eqWire (.head tail fault))
    | @tag head2 payload2 tail2 argument2 enc2 major2 isTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl eqWire (.head tail fault))
  | @incomplete head tail size info sizeEq short =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault2
      omega
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      rfl
    | eof =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 tail2 argument2 enc2 major2 notTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl eqWire (.incomplete tail info sizeEq short))
    | @tag head2 payload2 tail2 argument2 enc2 major2 isTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl eqWire (.incomplete tail info sizeEq short))
  | eof =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      simp [List.cons_append] at eqWire
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp [List.cons_append] at eqWire
    | eof =>
      rfl
    | @typeMismatch head2 payload2 tail2 argument2 enc2 major2 notTag2 =>
      simp [List.cons_append] at eqWire
    | @tag head2 payload2 tail2 argument2 enc2 major2 isTag2 =>
      simp [List.cons_append] at eqWire
  | @typeMismatch head payload tail argument enc major notTag =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl eqWire.symm (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl eqWire.symm (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 tail2 argument2 enc2 major2 notTag2 =>
      simp only [List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      rfl
    | @tag head2 payload2 tail2 argument2 enc2 major2 isTag2 =>
      simp only [List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      contradiction
  | @tag head payload tail argument enc major isTag =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl eqWire.symm (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl eqWire.symm (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 tail2 argument2 enc2 major2 notTag2 =>
      simp only [List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      contradiction
    | @tag head2 payload2 tail2 argument2 enc2 major2 isTag2 =>
      rfl

/-- Scalar diagnostics inherit expectation determinism after the nullish guard. -/
theorem scalarFailure_reason_unique (wire : List w8) (a b : ExpectedFailureReason)
    (first : ScalarFailure wire a) (second : ScalarFailure wire b) : a = b :=
  expectedFailure_reason_unique (W8 0) wire a b first.2.2 second.2.2

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headFault_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.expectedFailure_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.scalarFailure_reason_unique
