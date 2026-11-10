import ProofCheck.KeyFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Complete atomic-head rejection has one diagnostic class. -/
theorem skipReject_reason_unique (head : w8) (argument : w64) (rest : List w8) (a b : SkipAtomError)
    (first : SkipReject head argument rest a) (second : SkipReject head argument rest b) : a = b := by
  cases first <;> cases second <;> first | rfl | word

/-- A failure at the current Skip item has exactly one diagnostic reason and
    byte offset, including head faults, incomplete arguments, atomic failures,
    count/depth precedence and chained-tag EOF/depth cases. -/
theorem skipFailureLeaf_unique (d offsetA offsetB : Nat) (wire : List w8) (a b : SkipFailureReason)
    (first : SkipFailureLeaf d wire offsetA a) (second : SkipFailureLeaf d wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @head d head tail reason fault =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      exact ⟨rfl, congrArg (SkipFailureReason.head head) (headFault_reason_unique head reason reason2 fault fault2)⟩
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault
      omega
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.head tail fault))
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.head tail fault))
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.head tail fault))
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 []
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.head tail fault))
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 (next2 :: tail2)
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.head tail fault))
  | @incomplete d head tail size info sizeEq short =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault2
      omega
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact ⟨rfl, rfl⟩
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.incomplete tail info sizeEq short))
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.incomplete tail info sizeEq short))
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.incomplete tail info sizeEq short))
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 []
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.incomplete tail info sizeEq short))
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 (next2 :: tail2)
        _ argument2 _ enc2 rfl (by simpa only [List.append_nil] using eqWire)
        (.incomplete tail info sizeEq short))
  | @eof d =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      simp [List.cons_append] at eqWire
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      simp [List.cons_append] at eqWire
    | @eof d2 =>
      exact ⟨rfl, rfl⟩
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      simp [List.cons_append] at eqWire
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      simp [List.cons_append] at eqWire
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      simp [List.cons_append] at eqWire
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      simp [List.cons_append] at eqWire
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      simp [List.cons_append] at eqWire
  | @atomic d head payload tail argument reason enc reject =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      subst tail2
      exact ⟨rfl, congrArg SkipFailureReason.atomic (skipReject_reason_unique head argument tail reason reason2 reject reject2)⟩
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject <;> word
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject <;> word
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail [] argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject <;> word
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (next2 :: tail2) argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject <;> word
  | @count d head payload tail argument per enc major depth short =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject2 <;> word
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      exact ⟨rfl, rfl⟩
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      omega
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail [] argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major with h | h <;> word
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (next2 :: tail2) argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major with h | h <;> word
  | @containerDepth head payload tail argument per enc major =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject2 <;> word
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      omega
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      exact ⟨rfl, rfl⟩
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail [] argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major with h | h <;> word
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        tail (next2 :: tail2) argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major with h | h <;> word
  | @tagEOF d head payload argument enc major number =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload []
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload []
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        [] tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject2 <;> word
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        [] tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major2 with h | h <;> word
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        [] tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major2 with h | h <;> word
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        [] [] argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      exact ⟨rfl, rfl⟩
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        [] (next2 :: tail2) argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      simp at tailsEq
  | @tagDepth head next payload tail argument enc major number nextTag =>
    cases second with
    | @head d2 head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload (next :: tail)
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete d2 head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload (next :: tail)
        _ argument _ enc rfl (by simpa only [List.append_nil] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | @eof d2 =>
      simp [List.cons_append] at eqWire
    | @atomic d2 head2 payload2 tail2 argument2 reason2 enc2 reject2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (next :: tail) tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      cases reject2 <;> word
    | @count d2 head2 payload2 tail2 argument2 per2 enc2 major2 depth2 short2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (next :: tail) tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major2 with h | h <;> word
    | @containerDepth head2 payload2 tail2 argument2 per2 enc2 major2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (next :: tail) tail2 argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      rcases major2 with h | h <;> word
    | @tagEOF d2 head2 payload2 argument2 enc2 major2 number2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (next :: tail) [] argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      simp at tailsEq
    | @tagDepth head2 next2 payload2 tail2 argument2 enc2 major2 number2 nextTag2 =>
      obtain ⟨headEq, payloadEq, argumentEq, tailsEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (next :: tail) (next2 :: tail2) argument argument2 enc enc2
        (by simpa only [List.append_nil] using eqWire)
      subst head2; subst payload2; subst argument2
      exact ⟨rfl, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipReject_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureLeaf_unique
