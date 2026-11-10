import ProofCheck.SkipDeterminism
import ProofCheck.FeltPointersSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Unsigned limb witnesses have unique bytes, decoded word and suffix. -/
theorem unsignedWire_prefix_unique (a b : UnsignedWire) (restA restB : List w8)
    (same : a.bytes ++ restA = b.bytes ++ restB) :
    a.bytes = b.bytes ∧ a.value = b.value ∧ restA = restB := by
  exact scalarValid_prefix_unique (W64 0) a.value b.value a.bytes b.bytes restA restB
    (.unsigned a.argument a.major) (.unsigned b.argument b.major) same

/-- Normalize both short and extended felt arrays into the same decomposition:
    up to four limbs followed by the exact number of ignored extra items. -/
theorem feltValid_some_parts (d : Nat) (wire : List w8) (value : GoArray w64 4)
    (valid : FeltValid d wire (some value)) :
    ∃ (head : w8) (payload : List w8) (argument : w64) (items : List UnsignedWire) (extras : List (List w8)),
      wire = (head :: payload) ++ unsignedBytes items ++ extras.flatten ∧
      HeadArgument head payload argument ∧ head &&& W8 224 = W8 128 ∧
      uint.Z argument = items.length + extras.length ∧
      items.length = min 4 (uint.Z argument).toNat ∧
      (∀ item ∈ extras, SkipValid (d + 1) item) ∧ value = paddedLimbs items := by
  cases valid with
  | small items enc major count small bound =>
    refine ⟨_, _, _, items, [], ?_, enc, major, ?_, ?_, ?_, rfl⟩
    · simp
    · simpa using count
    · omega
    · simp
  | extended items extras enc major count four bound children =>
    refine ⟨_, _, _, items, extras, rfl, enc, major, ?_, ?_, children, rfl⟩
    · omega
    · omega

/-- A nonnil felt cannot start with null or undefined. -/
theorem feltValid_some_not_nullish (d : Nat) (wire : List w8) (value : GoArray w64 4)
    (valid : FeltValid d wire (some value)) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  obtain ⟨head, tail, rfl, major⟩ := feltValid_some_head d wire value valid
  simp only [List.head?_cons, ne_eq, Option.some.injEq]
  constructor <;> intro h <;> subst head <;> contradiction

/-- Every felt encoding has one decoded value and boundary, including null,
    zero-filled short arrays and arrays with arbitrary recursively skipped extras. -/
theorem feltValid_prefix_unique (d : Nat) (a b : Option (GoArray w64 4))
    (left right restLeft restRight : List w8)
    (first : FeltValid d left a) (second : FeltValid d right b)
    (same : left ++ restLeft = right ++ restRight) :
    left = right ∧ a = b ∧ restLeft = restRight := by
  cases a with
  | none =>
    obtain ⟨head, rfl, hn⟩ := feltValid_none d left first
    cases b with
    | none =>
      obtain ⟨other, rfl, ho⟩ := feltValid_none d right second
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, rfl⟩ := same
      exact ⟨rfl, rfl, rfl⟩
    | some b =>
      obtain ⟨other, tail, rfl, major⟩ := feltValid_some_head d right b second
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
  | some a =>
    cases b with
    | none =>
      obtain ⟨head, tail, rfl, major⟩ := feltValid_some_head d left a first
      obtain ⟨other, rfl, hn⟩ := feltValid_none d right second
      simp only [List.cons_append, List.nil_append, List.cons.injEq] at same
      obtain ⟨rfl, _⟩ := same
      rcases hn with rfl | rfl <;> contradiction
    | some b =>
      obtain ⟨head, payload, argument, items, extras, rfl, enc, major, count, size, children, rfl⟩ :=
        feltValid_some_parts d left a first
      obtain ⟨head2, payload2, argument2, items2, extras2, rfl, enc2, major2, count2, size2, children2, rfl⟩ :=
        feltValid_some_parts d right b second
      simp only [List.append_assoc, List.cons_append, List.cons.injEq] at same
      obtain ⟨rfl, same⟩ := same
      obtain ⟨payloadEq, argumentEq, tailsEq⟩ := headArgument_prefix_unique head payload payload2
        (unsignedBytes items ++ extras.flatten ++ restLeft)
        (unsignedBytes items2 ++ extras2.flatten ++ restRight) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      subst payload2
      subst argument2
      have lengths : items.length = items2.length := by omega
      have extraLengths : extras.length = extras2.length := by omega
      obtain ⟨wires, values, suffixes⟩ := wireSequence_prefix_unique UnsignedWire.bytes UnsignedWire.value
        items items2 (extras.flatten ++ restLeft) (extras2.flatten ++ restRight) lengths
        (fun x _ y _ => unsignedWire_prefix_unique x y)
        (by simpa only [unsignedBytes, List.append_assoc] using tailsEq)
      have limbs : paddedLimbs items = paddedLimbs items2 := by
        simp only [paddedLimbs, values, lengths]
      have result := wireSequence_prefix_unique (id : List w8 → List w8) (fun _ => ())
        extras extras2 restLeft restRight extraLengths (by
          intro x hx y hy tailX tailY split
          obtain ⟨wire, rest⟩ := skipValid_prefix_unique (d + 1) x y tailX tailY
            (children x hx) (children2 y hy) split
          exact ⟨wire, rfl, rest⟩) (by simpa only [List.flatMap_id] using suffixes)
      have extraWires : extras.flatten = extras2.flatten := by
        simpa only [List.flatMap_id] using result.1
      exact ⟨by simp only [unsignedBytes, wires, extraWires], congrArg some limbs, result.2.2⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.unsignedWire_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_some_parts
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_some_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.feltValid_prefix_unique
