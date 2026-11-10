import ProofCheck.SkipLeafFailureUnique

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A container whose head/count/depth checks pass cannot fail at its own head;
    any remaining failure must arise while reading a child. -/
theorem skipFailureLeaf_not_container (d offset : Nat) (head : w8) (payload body : List w8)
    (argument per : w64) (reason : SkipFailureReason) (enc : HeadArgument head payload argument)
    (major : (head &&& W8 224 = W8 128 ∧ per = W64 1) ∨ (head &&& W8 224 = W8 160 ∧ per = W64 2))
    (bound : d < 32) (fit : uint.Z argument * uint.Z per ≤ body.length)
    (failure : SkipFailureLeaf d ((head :: payload) ++ body) offset reason) : False := by
  generalize eqWire : (head :: payload) ++ body = input at failure
  cases failure with
  | head tail fault =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload body _ argument _ enc rfl eqWire.symm (.head tail fault)
  | incomplete tail info sizeEq short =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload body _ argument _ enc rfl eqWire.symm
      (.incomplete tail info sizeEq short)
  | atomic enc2 reject =>
    obtain ⟨headEq, _, argumentEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
    cases headEq; cases argumentEq
    cases reject <;> word
  | eof => simp [List.cons_append] at eqWire
  | count enc2 major2 depth short =>
    obtain ⟨headEq, _, argumentEq, tailEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
    cases headEq; cases argumentEq; cases tailEq
    rcases major with ⟨major, rfl⟩ | ⟨major, rfl⟩ <;>
      rcases major2 with ⟨major2, rfl⟩ | ⟨major2, rfl⟩ <;> word
  | containerDepth enc2 major2 => omega
  | tagEOF enc2 major2 number =>
    have headEq := congrArg List.head? eqWire
    simp only [List.cons_append, List.head?_cons, Option.some.injEq] at headEq
    cases headEq
    rcases major with h | h <;> word
  | tagDepth enc2 major2 number nextTag => omega

/-- A supported tag with a present child and available chained-tag depth cannot
    fail locally; its remaining errors must come from that child. -/
theorem skipFailureLeaf_not_tag (d offset : Nat) (head next : w8) (payload body : List w8)
    (argument : w64) (reason : SkipFailureReason) (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = W8 192) (number : 3 < uint.Z argument)
    (bound : next &&& W8 224 = W8 192 → d < 32)
    (failure : SkipFailureLeaf d ((head :: payload) ++ (next :: body)) offset reason) : False := by
  generalize eqWire : (head :: payload) ++ (next :: body) = input at failure
  cases failure with
  | head tail fault =>
    exact expectedFailure_excludes_encoded _ head payload (next :: body) _ argument _ enc major eqWire.symm (.head tail fault)
  | incomplete tail info sizeEq short =>
    exact expectedFailure_excludes_encoded _ head payload (next :: body) _ argument _ enc major eqWire.symm
      (.incomplete tail info sizeEq short)
  | atomic enc2 reject =>
    obtain ⟨headEq, _, argumentEq, _⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
    cases headEq; cases argumentEq
    cases reject <;> word
  | eof => simp [List.cons_append] at eqWire
  | count enc2 major2 depth short =>
    have headEq := congrArg List.head? eqWire
    simp only [List.cons_append, List.head?_cons, Option.some.injEq] at headEq
    cases headEq
    rcases major2 with h | h <;> word
  | containerDepth enc2 major2 =>
    have headEq := congrArg List.head? eqWire
    simp only [List.cons_append, List.head?_cons, Option.some.injEq] at headEq
    cases headEq
    rcases major2 with h | h <;> word
  | tagEOF enc2 major2 number2 =>
    obtain ⟨_, _, _, tailEq⟩ := encodedHead_prefix_unique _ _ _ _ _ [] _ _ enc enc2
      (by simpa only [List.append_nil] using eqWire)
    simp at tailEq
  | @tagDepth head2 next2 payload2 body2 argument2 enc2 major2 number2 nextTag =>
    obtain ⟨_, _, _, tailEq⟩ := encodedHead_prefix_unique _ _ _ _ _ _ _ _ enc enc2 eqWire
    have nextEq := (List.cons.inj tailEq).1
    subst next2
    have := bound nextTag
    omega

/-- A local failure cannot simultaneously be reached by entering a supported
    outer context. Only the empty context can describe that same root failure. -/
theorem skipFailureLeaf_context_empty (tail front : List w8) (d leaf offset : Nat) (reason : SkipFailureReason)
    (context : SkipFailureContext tail d front leaf) (failure : SkipFailureLeaf d (front ++ tail) offset reason) :
    front = [] := by
  cases context with
  | nil => rfl
  | container items enc major bound more fit children child =>
    exact False.elim (skipFailureLeaf_not_container _ _ _ _ _ _ _ _ enc major bound fit
      (by simpa only [List.append_assoc] using failure))
  | tag enc major number nonempty bound child =>
    exact False.elim (skipFailureLeaf_not_tag _ _ _ _ _ _ _ _ enc major number bound
      (by simpa only [List.append_assoc, nonempty] using failure))

/-- An empty context has entered no containers or chained tags. -/
theorem skipFailureContext_empty_depth (tail : List w8) (d leaf : Nat)
    (context : SkipFailureContext tail d [] leaf) : d = leaf := by
  cases context with
  | nil => rfl

/-- Comparing complete failure paths fixes the first failing child's boundary,
    then its exact diagnostic. Different sibling counts would put a valid child
    where the other path already fails, contradicting success/error separation. -/
theorem skipFailureContext_diagnostic_unique (tail front : List w8) (d leaf offsetA offsetB : Nat)
    (a b : SkipFailureReason) (firstContext : SkipFailureContext tail d front leaf)
    (first : SkipFailureLeaf leaf tail offsetA a) (tail2 front2 : List w8) (leaf2 : Nat)
    (secondContext : SkipFailureContext tail2 d front2 leaf2)
    (second : SkipFailureLeaf leaf2 tail2 offsetB b) (same : front ++ tail = front2 ++ tail2) :
    front.length + offsetA = front2.length + offsetB ∧ a = b := by
  induction firstContext generalizing tail2 front2 leaf2 offsetB b with
  | nil d =>
    have empty : front2 = [] := skipFailureLeaf_context_empty tail2 front2 d leaf2 offsetA a secondContext
      (by simpa only [List.nil_append] using (same ▸ first))
    subst front2
    have depthEq := skipFailureContext_empty_depth tail2 d leaf2 secondContext
    subst leaf2
    simp only [List.nil_append] at same
    subst tail2
    simpa only [List.length_nil, Nat.zero_add] using skipFailureLeaf_unique d offsetA offsetB tail a b first second
  | @container d leaf head payload front count per items enc major bound more fit children context ih =>
    cases secondContext with
    | nil =>
      have failure : SkipFailureLeaf d (((head :: payload) ++ (items.flatten ++ front)) ++ tail) offsetB b := by
        simpa only [List.nil_append] using (same ▸ second)
      exact False.elim (skipFailureLeaf_not_container d offsetB head payload (items.flatten ++ (front ++ tail)) count per b
        enc major bound fit (by simpa only [List.append_assoc] using failure))
    | @container d2 leaf2 head2 payload2 front2 count2 per2 items2 enc2 major2 bound2 more2 fit2 children2 context2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (items.flatten ++ (front ++ tail)) (items2.flatten ++ (front2 ++ tail2)) count count2 enc enc2
        (by simpa only [List.append_assoc] using same)
      subst head2; subst payload2; subst count2
      by_cases lengths : items.length = items2.length
      · have result := wireSequence_prefix_unique (id : List w8 → List w8) (fun _ => ()) items items2
          (front ++ tail) (front2 ++ tail2) lengths (by
            intro x hx y hy restX restY split
            obtain ⟨wire, rest⟩ := skipValid_prefix_unique (d + 1) x y restX restY (children x hx) (children2 y hy) split
            exact ⟨wire, rfl, rest⟩) (by simpa only [List.flatMap_id] using bodiesEq)
        have wires : items.flatten = items2.flatten := by simpa only [List.flatMap_id] using result.1
        have diagnostic := ih offsetB b first tail2 front2 leaf2 context2 second result.2.2
        have byteLengths := congrArg List.length wires
        constructor
        · simp only [List.length_append, List.length_cons]
          have := diagnostic.1
          omega
        · exact diagnostic.2
      · by_cases shorter : items.length < items2.length
        · obtain ⟨item, suffix, split, valid⟩ := skipItems_after_prefix (d + 1) items items2
            (front ++ tail) (front2 ++ tail2) children children2 shorter bodiesEq
          have failure : SkipFailure (d + 1) (front ++ tail) (front.length + offsetA) a := .within context first
          rw [split] at failure
          exact False.elim (skipValid_not_failure _ _ _ _ _ valid failure)
        · have shorter2 : items2.length < items.length := by omega
          obtain ⟨item, suffix, split, valid⟩ := skipItems_after_prefix (d + 1) items2 items
            (front2 ++ tail2) (front ++ tail) children2 children shorter2 bodiesEq.symm
          have failure : SkipFailure (d + 1) (front2 ++ tail2) (front2.length + offsetB) b := .within context2 second
          rw [split] at failure
          exact False.elim (skipValid_not_failure _ _ _ _ _ valid failure)
    | tag enc2 major2 number2 nonempty2 bound2 context2 =>
      have headEq := congrArg List.head? same
      simp only [List.append_assoc, List.cons_append, List.head?_cons, Option.some.injEq] at headEq
      cases headEq
      exfalso
      rcases major with h | h <;> word
  | @tag d leaf head next payload front rest argument enc major number nonempty bound context ih =>
    cases secondContext with
    | nil =>
      have failure : SkipFailureLeaf d (((head :: payload) ++ front) ++ tail) offsetB b := by
        simpa only [List.nil_append] using (same ▸ second)
      exact False.elim (skipFailureLeaf_not_tag d offsetB head next payload rest argument b enc major number bound
        (by simpa only [List.append_assoc, nonempty] using failure))
    | container items2 enc2 major2 bound2 more2 fit2 children2 context2 =>
      have headEq := congrArg List.head? same
      simp only [List.append_assoc, List.cons_append, List.head?_cons, Option.some.injEq] at headEq
      cases headEq
      exfalso
      rcases major2 with h | h <;> word
    | @tag d2 leaf2 head2 next2 payload2 front2 rest2 argument2 enc2 major2 number2 nonempty2 bound2 context2 =>
      obtain ⟨headEq, payloadEq, argumentEq, bodiesEq⟩ := encodedHead_prefix_unique head head2 payload payload2
        (front ++ tail) (front2 ++ tail2) argument argument2 enc enc2
        (by simpa only [List.append_assoc] using same)
      subst head2; subst payload2
      have nextEq : next = next2 := (List.cons.inj (nonempty.symm.trans (bodiesEq.trans nonempty2))).1
      subst next2
      have diagnostic := ih offsetB b first tail2 front2 leaf2 context2 second bodiesEq
      constructor
      · simp only [List.length_append, List.length_cons]
        have := diagnostic.1
        omega
      · exact diagnostic.2

/-- Arbitrary recursive Skip failures have one offset and one diagnostic reason. -/
theorem skipFailure_unique (d offsetA offsetB : Nat) (wire : List w8) (a b : SkipFailureReason)
    (first : SkipFailure d wire offsetA a) (second : SkipFailure d wire offsetB b) :
    offsetA = offsetB ∧ a = b := by
  generalize eqWire : wire = wire2 at second
  cases first with
  | within context first =>
    cases second with
    | within context2 second =>
      exact skipFailureContext_diagnostic_unique _ _ _ _ _ _ _ _ context first _ _ _ context2 second eqWire

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureLeaf_not_container
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureLeaf_not_tag
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureLeaf_context_empty
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureContext_empty_depth
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailureContext_diagnostic_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.skipFailure_unique
