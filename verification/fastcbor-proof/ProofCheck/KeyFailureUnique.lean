import ProofCheck.OpeningFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Complete key rejection has exactly one diagnostic reason, including the
    offending byte, integer overflow, tags, type mismatch, text truncation and
    arbitrary malformed UTF8. This is independent of lookup cache behavior. -/
theorem keyRejected_reason_unique (wire : List w8) (a b : KeyFailureReason)
    (first : KeyRejected wire a) (second : KeyRejected wire b) : a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | @head head tail reason fault =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      exact congrArg (KeyFailureReason.head head) (headFault_reason_unique head reason reason2 fault fault2)
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp only [List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      have := headFault_info _ _ fault
      omega
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 (body2 ++ rem2)
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.head tail fault))
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
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 tail2
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      exact False.elim (expectedFailure_excludes_encoded (head2 &&& W8 224) head2 payload2 (body2 ++ rem2)
        _ argument2 _ enc2 rfl (by simpa only [List.append_assoc] using eqWire)
        (.incomplete tail info sizeEq short))
  | eof =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      simp [List.cons_append] at eqWire
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      simp [List.cons_append] at eqWire
    | eof =>
      rfl
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      simp [List.cons_append] at eqWire
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      simp [List.cons_append] at eqWire
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp [List.cons_append] at eqWire
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      simp [List.cons_append] at eqWire
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      simp [List.cons_append] at eqWire
  | @integerRange head payload tail argument enc major bound =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      rfl
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
  | @tag head payload tail argument enc major =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      rfl
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
  | @typeMismatch head payload tail argument enc text uint negative tag =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      rfl
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
  | @textTruncated head payload tail argument enc major short =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      rfl
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      cases argumentEq
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      omega
  | @invalidUTF8 head payload body rem argument enc major count invalid =>
    cases second with
    | @head head2 tail2 reason2 fault2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload (body ++ rem)
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.head tail2 fault2))
    | @incomplete head2 tail2 size2 info2 sizeEq2 short2 =>
      exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload (body ++ rem)
        _ argument _ enc rfl (by simpa only [List.append_assoc] using eqWire.symm)
        (.incomplete tail2 info2 sizeEq2 short2))
    | eof =>
      simp [List.cons_append] at eqWire
    | @integerRange head2 payload2 tail2 argument2 enc2 major2 bound2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @tag head2 payload2 tail2 argument2 enc2 major2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @typeMismatch head2 payload2 tail2 argument2 enc2 text2 uint2 negative2 tag2 =>
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at eqWire
      obtain ⟨rfl, _⟩ := eqWire
      word
    | @textTruncated head2 payload2 tail2 argument2 enc2 major2 short2 =>
      obtain ⟨_, _, argumentEq, tailsEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      cases argumentEq
      have lengths := congrArg List.length tailsEq
      simp only [List.length_append] at lengths
      omega
    | @invalidUTF8 head2 payload2 body2 rem2 argument2 enc2 major2 count2 invalid2 =>
      rfl

/-- Existing production-proved key failures inherit the complete diagnostic
    uniqueness result under the already-required representable input bound. -/
theorem keyFailure_reason_unique (wire : List w8) (a b : KeyFailureReason) (bound : wire.length < 2^64)
    (first : KeyFailure wire a) (second : KeyFailure wire b) : a = b :=
  keyRejected_reason_unique wire a b (keyFailure_rejected wire a bound first) (keyFailure_rejected wire b bound second)

/-- Existing string-failure witnesses agree on their complete diagnostic too. -/
theorem stringFailure_reason_unique (wire : List w8) (a b : StringFailureReason) (bound : wire.length < 2^64)
    (first : StringFailure wire a) (second : StringFailure wire b) : a = b :=
  stringRejected_reason_unique wire a b (stringFailure_rejected wire a bound first) (stringFailure_rejected wire b bound second)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.keyRejected_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.keyFailure_reason_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.stringFailure_reason_unique
