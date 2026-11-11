import ProofCheck.BigIntSpec
import ProofCheck.BytesSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Retain the tag width in payload diagnostics, independently of the reader. -/
def bigIntPayloadReason (width : Nat) : BytesFailureReason → BigIntFailureReason
  | .expected reason => .payloadExpected width reason
  | .truncated => .payloadTruncated width

/-- After a complete head, only a wrong major, unsupported tag, or rejected
    bignum byte payload can fail. -/
inductive BigIntAfterHeadFailure (head : w8) (payload : List w8) (argument : w64)
    (tail : List w8) : BigIntFailureReason → Prop where
  | typeMismatch (hu : head &&& W8 224 ≠ W8 0) (hn : head &&& W8 224 ≠ W8 32)
      (ht : head &&& W8 224 ≠ W8 192) : BigIntAfterHeadFailure head payload argument tail (.typeMismatch head)
  | tag (major : head &&& W8 224 = W8 192) (hp : argument ≠ W64 2) (hn : argument ≠ W64 3) :
      BigIntAfterHeadFailure head payload argument tail (.tag argument)
  | payload {reason : BytesFailureReason} (major : head &&& W8 224 = W8 192)
      (allowed : argument = W64 2 ∨ argument = W64 3) (failure : BytesRejected tail reason) :
      BigIntAfterHeadFailure head payload argument tail (bigIntPayloadReason (1 + payload.length) reason)

/-- A complete head excludes malformed-head rejection and fixes the remaining
    rejection alternatives and the exact suffix on which they operate. -/
theorem bigIntRejected_encoded (head : w8) (payload tail : List w8) (argument : w64)
    (reason : BigIntFailureReason) (enc : HeadArgument head payload argument)
    (failure : BigIntRejected ((head :: payload) ++ tail) reason) :
    BigIntAfterHeadFailure head payload argument tail reason := by
  generalize same : (head :: payload) ++ tail = wire at failure
  cases failure with
  | @head bad problem rest fault =>
    exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
      _ argument _ enc rfl same.symm (.head rest fault))
  | @incomplete bad rest size info sizeEq short =>
    exact False.elim (expectedFailure_excludes_encoded (head &&& W8 224) head payload tail
      _ argument _ enc rfl same.symm (.incomplete rest info sizeEq short))
  | eof => simp [List.cons_append] at same
  | @typeMismatch other bytes arg rest enc2 hu hn ht =>
    obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
    exact .typeMismatch hu hn ht
  | @tag other bytes arg rest enc2 major hp hn =>
    obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
    exact .tag major hp hn
  | @payloadExpected other bytes rest arg problem enc2 major allowed inner =>
    obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 same
    exact .payload major allowed (.expected inner)
  | @payloadTruncated other bodyHead bytes lengthBytes rest arg count enc2 major allowed bodyEnc bodyMajor short =>
    obtain ⟨rfl, rfl, rfl, rfl⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2
      (by simpa only [List.append_assoc] using same)
    exact .payload major allowed (.truncated bodyEnc bodyMajor short)

/-- The post-head alternatives have a unique diagnostic, including payload
    reason and tag width. -/
theorem bigIntAfterHeadFailure_unique (head : w8) (payload tail : List w8) (argument : w64)
    (a b : BigIntFailureReason) (first : BigIntAfterHeadFailure head payload argument tail a)
    (second : BigIntAfterHeadFailure head payload argument tail b) : a = b := by
  cases first with
  | typeMismatch hu hn ht =>
    cases second with
    | typeMismatch => rfl
    | tag major => exact False.elim (ht major)
    | payload major => exact False.elim (ht major)
  | tag major hp hn =>
    cases second with
    | typeMismatch hu hn ht => exact False.elim (ht major)
    | tag => rfl
    | payload major2 allowed inner =>
      rcases allowed with h | h
      · exact False.elim (hp h)
      · exact False.elim (hn h)
  | payload major allowed inner =>
    cases second with
    | typeMismatch hu hn ht => exact False.elim (ht major)
    | tag major2 hp hn =>
      rcases allowed with h | h
      · exact False.elim (hp h)
      · exact False.elim (hn h)
    | payload major2 allowed2 inner2 =>
      exact congrArg (bigIntPayloadReason (1 + payload.length)) (bytesRejected_unique _ _ _ inner inner2)

/-- A supported integer prefix cannot also be rejected on the same input. -/
theorem bigIntValid_not_rejected (item rest : List w8) (value : Int) (reason : BigIntFailureReason)
    (valid : BigIntValid item value) (failure : BigIntRejected (item ++ rest) reason) : False := by
  cases valid with
  | unsigned enc major =>
    have afterHead := bigIntRejected_encoded _ _ _ _ _ enc failure
    cases afterHead with
    | typeMismatch hu hn ht => exact hu major
    | tag tagMajor => rw [major] at tagMajor; contradiction
    | payload tagMajor => rw [major] at tagMajor; contradiction
  | negative enc major =>
    have afterHead := bigIntRejected_encoded _ _ _ _ _ enc failure
    cases afterHead with
    | typeMismatch hu hn ht => exact hn major
    | tag tagMajor => rw [major] at tagMajor; contradiction
    | payload tagMajor => rw [major] at tagMajor; contradiction
  | @tagged tagHead bodyHead tagBytes lengthBytes body tag count enc major allowed bodyEnc bodyMajor length =>
    have afterHead := bigIntRejected_encoded tagHead tagBytes ((bodyHead :: lengthBytes) ++ body ++ rest) tag reason enc
      (by simpa only [List.append_assoc] using failure)
    cases afterHead with
    | typeMismatch hu hn ht => exact ht major
    | tag tagMajor hp hn =>
      rcases allowed with h | h
      · exact hp h
      · exact hn h
    | payload tagMajor allowed2 inner =>
      exact bytesValid_not_rejected _ _ _ _ (.bytes bodyEnc bodyMajor length) inner

theorem bigIntRejected_head_unique (head : w8) (tail : List w8) (problem : HeadError)
    (reason : BigIntFailureReason) (fault : HeadFault head problem)
    (failure : BigIntRejected (head :: tail) reason) : reason = .head head problem := by
  generalize same : head :: tail = wire at failure
  cases failure with
  | @head other problem2 rest fault2 =>
    simp only [List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    exact congrArg (BigIntFailureReason.head head) (headFault_reason_unique _ _ _ fault2 fault)
  | @incomplete other rest size info sizeEq short =>
    simp only [List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    have := headFault_info _ _ fault
    omega
  | eof => simp at same
  | @typeMismatch other bytes arg rest enc hu hn ht =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.head tail fault))
  | @tag other bytes arg rest enc major hp hn =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.head tail fault))
  | @payloadExpected other bytes rest arg problem enc major allowed inner =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.head tail fault))
  | @payloadTruncated other bodyHead bytes lengthBytes rest arg count enc major allowed bodyEnc bodyMajor short2 =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes ((bodyHead :: lengthBytes) ++ rest)
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.head tail fault))

theorem bigIntRejected_incomplete_unique (head : w8) (tail : List w8) (size : w64)
    (reason : BigIntFailureReason)
    (info : 24 ≤ uint.Z (head &&& W8 31) ∧ uint.Z (head &&& W8 31) ≤ 27)
    (sizeEq : W64 1 <<< W64 (uint.Z ((head &&& W8 31) - W8 24)) = size)
    (short : tail.length < sint.Z size) (failure : BigIntRejected (head :: tail) reason) :
    reason = .incomplete := by
  generalize same : head :: tail = wire at failure
  cases failure with
  | @head other problem rest fault =>
    simp only [List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    have := headFault_info _ _ fault
    omega
  | incomplete => rfl
  | eof => simp at same
  | @typeMismatch other bytes arg rest enc hu hn ht =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.incomplete tail info sizeEq short))
  | @tag other bytes arg rest enc major hp hn =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.incomplete tail info sizeEq short))
  | @payloadExpected other bytes rest arg problem enc major allowed inner =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes rest
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.incomplete tail info sizeEq short))
  | @payloadTruncated other bodyHead bytes lengthBytes rest arg count enc major allowed bodyEnc bodyMajor short2 =>
    exact False.elim (expectedFailure_excludes_encoded (other &&& W8 224) other bytes ((bodyHead :: lengthBytes) ++ rest)
      _ arg _ enc rfl (by simpa only [List.append_assoc] using same) (.incomplete tail info sizeEq short))

/-- All specified BigInt failures are deterministic, including inner offsets. -/
theorem bigIntRejected_unique (wire : List w8) (a b : BigIntFailureReason)
    (first : BigIntRejected wire a) (second : BigIntRejected wire b) : a = b := by
  cases first with
  | head tail fault => exact (bigIntRejected_head_unique _ _ _ _ fault second).symm
  | incomplete tail info sizeEq short =>
    exact (bigIntRejected_incomplete_unique _ _ _ _ info sizeEq short second).symm
  | eof => cases second; rfl
  | typeMismatch tail enc hu hn ht =>
    exact bigIntAfterHeadFailure_unique _ _ _ _ _ _ (.typeMismatch hu hn ht)
      (bigIntRejected_encoded _ _ _ _ _ enc second)
  | tag tail enc major hp hn =>
    exact bigIntAfterHeadFailure_unique _ _ _ _ _ _ (.tag major hp hn)
      (bigIntRejected_encoded _ _ _ _ _ enc second)
  | payloadExpected enc major allowed inner =>
    exact bigIntAfterHeadFailure_unique _ _ _ _ _ _ (.payload major allowed (.expected inner))
      (bigIntRejected_encoded _ _ _ _ _ enc second)
  | payloadTruncated enc major allowed bodyEnc bodyMajor short =>
    exact bigIntAfterHeadFailure_unique _ _ _ _ _ _ (.payload major allowed (.truncated bodyEnc bodyMajor short))
      (bigIntRejected_encoded _ _ _ _ _ enc (by simpa only [List.append_assoc] using second))

/-- None of the integer encodings starts with null or undefined. -/
theorem bigIntValid_not_nullish (item rest : List w8) (value : Int) (valid : BigIntValid item value) :
    (item ++ rest).head? ≠ some (W8 246) ∧ (item ++ rest).head? ≠ some (W8 247) := by
  cases valid <;>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq] <;>
    (constructor <;> intro h <;> subst h <;> contradiction)

/-- Payload type mismatches are reclassified at the outer tag; other payload
    errors keep the offset immediately after the complete tag head. -/
def bigIntDiagnosticOffset : BigIntFailureReason → Nat
  | .payloadExpected _ (.typeMismatch _) => 0
  | .payloadExpected width _ | .payloadTruncated width => width
  | _ => 0

/-- Every diagnostic offset lies within the original input even though the
    actual cursor rolls back to the start on every error. -/
theorem bigIntRejected_offset_bounds (wire : List w8) (reason : BigIntFailureReason)
    (failure : BigIntRejected wire reason) : bigIntDiagnosticOffset reason ≤ wire.length := by
  cases failure with
  | head => simp [bigIntDiagnosticOffset]
  | incomplete => simp [bigIntDiagnosticOffset]
  | eof => simp [bigIntDiagnosticOffset]
  | typeMismatch => simp [bigIntDiagnosticOffset]
  | tag => simp [bigIntDiagnosticOffset]
  | @payloadExpected head bytes tail tag problem enc major allowed inner =>
    cases problem <;> simp [bigIntDiagnosticOffset, List.length_append] <;> omega
  | payloadTruncated => simp [bigIntDiagnosticOffset, List.length_append]; omega

inductive BigIntResult where
  | success (value : Int) (rest : List w8)
  | failure (reason : BigIntFailureReason)

def BigIntSpec (wire : List w8) : BigIntResult → Prop
  | .success value rest => ∃ item, wire = item ++ rest ∧ BigIntValid item value
  | .failure reason => BigIntRejected wire reason

theorem bigIntSpec_unique (wire : List w8) (a b : BigIntResult)
    (first : BigIntSpec wire a) (second : BigIntSpec wire b) : a = b := by
  cases a with
  | success value rest =>
    obtain ⟨item, split, valid⟩ := first
    cases b with
    | success value2 rest2 =>
      obtain ⟨item2, split2, valid2⟩ := second
      obtain ⟨_, rfl, rfl⟩ := bigIntValid_prefix_unique item item2 rest rest2 value value2 valid valid2 (split.symm.trans split2)
      rfl
    | failure reason =>
      subst wire
      exact False.elim (bigIntValid_not_rejected _ _ _ _ valid second)
  | failure reason =>
    cases b with
    | success value rest =>
      obtain ⟨item, split, valid⟩ := second
      subst wire
      exact False.elim (bigIntValid_not_rejected _ _ _ _ valid first)
    | failure reason2 => exact congrArg BigIntResult.failure (bigIntRejected_unique _ _ _ first second)

theorem bigIntSpec_total_unique (wire : List w8) :
    ∃ outcome, BigIntSpec wire outcome ∧ ∀ other, BigIntSpec wire other → other = outcome := by
  have existsOutcome : ∃ outcome, BigIntSpec wire outcome := by
    rcases bigInt_classify wire with ⟨item, rest, value, split, valid⟩ | ⟨reason, failure⟩
    · exact ⟨.success value rest, item, split, valid⟩
    · exact ⟨.failure reason, failure⟩
  obtain ⟨outcome, spec⟩ := existsOutcome
  exact ⟨outcome, spec, fun other otherSpec => bigIntSpec_unique wire other outcome otherSpec spec⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntRejected_encoded
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntAfterHeadFailure_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntValid_not_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntRejected_head_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntRejected_incomplete_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntRejected_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntSpec_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntSpec_total_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntValid_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntRejected_offset_bounds
