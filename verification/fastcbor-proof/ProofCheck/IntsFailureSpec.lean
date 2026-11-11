import ProofCheck.IntsFailure

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

inductive IntsFailureReason where
  | outer (reason : ExpectedFailureReason)
  | count
  | depth
  | element (reason : IntElementFailureReason)

/-- Retained cursor consumption, failed-element offset and whether the array
    was entered. An unsigned overflow advances beyond the failed element start. -/
inductive IntsRejected : Nat → List w8 → Nat → Nat → Bool → IntsFailureReason → Prop where
  | outer {d : Nat} {wire : List w8} {reason : ExpectedFailureReason}
      (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
      (failure : ExpectedFailure (W8 128) wire reason) :
      IntsRejected d wire 0 0 false (.outer reason)
  | count {d : Nat} {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (short : tail.length < uint.Z argument) :
      IntsRejected d ((head :: payload) ++ tail) 0 0 false .count
  | depth {head : w8} {payload tail : List w8} {argument : w64}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 128)
      (fit : uint.Z argument ≤ tail.length) :
      IntsRejected 32 ((head :: payload) ++ tail) 0 0 false .depth
  | element {d count consumed : Nat} {head : w8} {payload wire : List w8}
      (items : List IntElementWire) {reason : IntElementFailureReason}
      (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 128)
      (depth : d < 32) (more : items.length < count)
      (fit : count ≤ (items.flatMap IntElementWire.bytes ++ wire).length)
      (failure : IntElementRejected wire consumed reason) :
      IntsRejected d ((head :: payload) ++ items.flatMap IntElementWire.bytes ++ wire)
        (1 + payload.length + (items.flatMap IntElementWire.bytes).length + consumed)
        (1 + payload.length + (items.flatMap IntElementWire.bytes).length) true (.element reason)

theorem intsRejected_bounds (d consumed offset : Nat) (wire : List w8) (entered : Bool)
    (reason : IntsFailureReason) (failure : IntsRejected d wire consumed offset entered reason) :
    offset ≤ consumed ∧ consumed ≤ wire.length := by
  cases failure with
  | outer _ _ _ | count _ _ _ | depth _ _ _ => simp
  | element items enc major depth more fit failure =>
    have := intElementRejected_bounds _ _ _ failure
    simp only [List.length_append, List.length_cons]
    omega

theorem intsRejected_depth (d consumed offset : Nat) (wire : List w8) (entered : Bool)
    (reason : IntsFailureReason) (hd : d ≤ 32)
    (failure : IntsRejected d wire consumed offset entered reason) :
    d + (if entered then 1 else 0 : Nat) ≤ 32 := by
  cases failure with
  | outer _ _ _ | count _ _ _ | depth _ _ _ => simpa using hd
  | element items enc major depth more fit failure => simp; omega

/-- Requested elements either all exist or have a valid prefix followed by an
    independently specified failure. No implementation behavior is assumed. -/
theorem int_items_classify (n : Nat) (wire : List w8) :
    (∃ (items : List IntElementWire) (rest : List w8),
      wire = items.flatMap IntElementWire.bytes ++ rest ∧ items.length = n) ∨
    (∃ (items : List IntElementWire) (tail : List w8) (consumed : Nat) (reason : IntElementFailureReason),
      wire = items.flatMap IntElementWire.bytes ++ tail ∧ items.length < n ∧ IntElementRejected tail consumed reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, rfl, rfl⟩
  | succ n ih =>
    rcases intElement_classify wire with ⟨item, rest, split⟩ | ⟨consumed, reason, failure⟩
    · rcases ih rest with ⟨items, tail, htail, hcount⟩ | ⟨items, tail, consumed, reason, htail, hcount, failure⟩
      · refine Or.inl ⟨item :: items, tail, ?_, by simp only [List.length_cons, hcount]⟩
        simpa only [List.flatMap_cons, List.append_assoc, htail] using split
      · refine Or.inr ⟨item :: items, tail, consumed, reason, ?_, by simp only [List.length_cons]; omega, failure⟩
        simpa only [List.flatMap_cons, List.append_assoc, htail] using split
    · exact Or.inr ⟨[], wire, consumed, reason, rfl, by simp, failure⟩

def IntsClassified (d : Nat) (wire : List w8) : Prop :=
  (∃ item rest value, wire = item ++ rest ∧ IntsValid d item value) ∨
  (∃ consumed offset entered reason, IntsRejected d wire consumed offset entered reason)

theorem ints_array_classify (d : Nat) (head : w8) (payload body : List w8) (count : w64)
    (enc : HeadArgument head payload count) (major : head &&& W8 224 = W8 128)
    (hdepth : d ≤ 32) : IntsClassified d ((head :: payload) ++ body) := by
  by_cases short : (body.length : Int) < uint.Z count
  · exact Or.inr ⟨0, 0, false, .count, .count enc major short⟩
  have fit : uint.Z count ≤ body.length := by omega
  by_cases depth : d < 32
  · have hc : 0 ≤ uint.Z count := by word
    let n := (uint.Z count).toNat
    have hn : (n : Int) = uint.Z count := by dsimp only [n]; omega
    have hnword : W64 n = count := by word
    rcases int_items_classify n body with
      ⟨items, rest, split, hcount⟩ | ⟨items, tail, consumed, reason, split, hcount, failure⟩
    · refine Or.inl ⟨(head :: payload) ++ items.flatMap IntElementWire.bytes, rest, some (items.map IntElementWire.number), ?_,
        .array items enc major (by omega) depth⟩
      rw [split, List.append_assoc]
    · refine Or.inr ⟨1 + payload.length + (items.flatMap IntElementWire.bytes).length + consumed,
        1 + payload.length + (items.flatMap IntElementWire.bytes).length, true, .element reason, ?_⟩
      rw [split, ← List.append_assoc]
      exact .element items (by simpa only [hnword] using enc) major depth hcount
        (by rw [← split]; omega) failure
  · have heq : d = 32 := by omega
    subst d
    exact Or.inr ⟨0, 0, false, .depth, .depth enc major fit⟩

/-- Every finite input at supported depth has a specified success or rejection,
    with null precedence and count-before-depth checks matching the API contract. -/
theorem ints_classify (d : Nat) (wire : List w8) (hdepth : d ≤ 32) : IntsClassified d wire := by
  by_cases nullish : wire.head? = some (W8 246) ∨ wire.head? = some (W8 247)
  · cases wire with
    | nil => simp at nullish
    | cons head tail =>
      have hn : head = W8 246 ∨ head = W8 247 := by simpa using nullish
      exact Or.inl ⟨[head], tail, none, rfl, .nullish hn⟩
  · have hn : wire.head? ≠ some (W8 246) := fun h => nullish (Or.inl h)
    have hu : wire.head? ≠ some (W8 247) := fun h => nullish (Or.inr h)
    rcases expected_classify (W8 128) wire with ⟨head, payload, rest, count, split, enc, major⟩ | ⟨reason, failure⟩
    · rw [split]
      exact ints_array_classify d head payload rest count enc major hdepth
    · exact Or.inr ⟨0, 0, false, .outer reason, .outer hn hu failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.intsRejected_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.int_items_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.ints_array_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.ints_classify
