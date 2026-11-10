import ProofCheck.SkipDeterminism
import ProofCheck.SkipFailureSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Successful body shape after a decoded head, grouping arrays and maps by
    their number of child items. This is a normal form of SkipValid. -/
inductive SkipBodyValid (d : Nat) (head : w8) (argument : w64) : List w8 → Prop where
  | atom {body : List w8} (valid : SkipAtom head argument body) : SkipBodyValid d head argument body
  | container (perCount : w64) (items : List (List w8))
      (major : (head &&& W8 224 = W8 128 ∧ perCount = W64 1) ∨
        (head &&& W8 224 = W8 160 ∧ perCount = W64 2))
      (count : uint.Z argument * uint.Z perCount = items.length) (bound : d < 32)
      (children : ∀ item ∈ items, SkipValid (d + 1) item) :
      SkipBodyValid d head argument items.flatten
  | tag {next : w8} {tail : List w8} (major : head &&& W8 224 = W8 192)
      (number : 3 < uint.Z argument) (bound : next &&& W8 224 = W8 192 → d < 32)
      (child : SkipValid (d + if next &&& W8 224 = W8 192 then 1 else 0) (next :: tail)) :
      SkipBodyValid d head argument (next :: tail)

theorem skipValid_parts (d : Nat) (wire : List w8) (valid : SkipValid d wire) :
    ∃ (head : w8) (payload : List w8) (argument : w64) (body : List w8),
      wire = (head :: payload) ++ body ∧ HeadArgument head payload argument ∧ SkipBodyValid d head argument body := by
  cases valid with
  | atom enc valid => exact ⟨_, _, _, _, rfl, enc, .atom valid⟩
  | array items enc major count bound children =>
    exact ⟨_, _, _, _, rfl, enc, .container (W64 1) items (Or.inl ⟨major, rfl⟩)
      (by simpa only [show uint.Z (W64 1) = 1 from rfl, Int.mul_one] using count) bound children⟩
  | map items enc major count bound children =>
    exact ⟨_, _, _, _, rfl, enc, .container (W64 2) items (Or.inr ⟨major, rfl⟩)
      (by simpa only [show uint.Z (W64 2) = 2 from rfl, Int.mul_comm] using count) bound children⟩
  | tag enc major number bound child => exact ⟨_, _, _, _, rfl, enc, .tag major number bound child⟩

/-- Aligning independently decoded heads also aligns their remaining bodies. -/
theorem encodedHead_prefix_unique (head other : w8) (payload payload2 body body2 : List w8)
    (argument argument2 : w64) (enc : HeadArgument head payload argument) (enc2 : HeadArgument other payload2 argument2)
    (same : (head :: payload) ++ body = (other :: payload2) ++ body2) :
    head = other ∧ payload = payload2 ∧ argument = argument2 ∧ body = body2 := by
  simp only [List.cons_append, List.cons.injEq] at same
  obtain ⟨rfl, tailEq⟩ := same
  exact ⟨rfl, headArgument_prefix_unique head payload payload2 body body2 argument argument2 enc enc2 tailEq⟩

/-- No accepted current item can also have any specified failure at that item,
    even when arbitrary bytes follow the successful prefix. -/
theorem skipValid_not_failureLeaf (d offset : Nat) (item rest : List w8) (reason : SkipFailureReason)
    (valid : SkipValid d item) (failure : SkipFailureLeaf d (item ++ rest) offset reason) : False := by
  obtain ⟨head, payload, argument, body, rfl, enc, bodyValid⟩ := skipValid_parts d item valid
  generalize eqWire : ((head :: payload) ++ body) ++ rest = wire at failure
  cases failure with
  | head tail fault =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload (body ++ rest) _ argument _ enc rfl
      (by simpa only [List.append_assoc] using eqWire.symm) (.head tail fault)
  | incomplete tail info sizeEq short =>
    exact expectedFailure_excludes_encoded (head &&& W8 224) head payload (body ++ rest) _ argument _ enc rfl
      (by simpa only [List.append_assoc] using eqWire.symm) (.incomplete tail info sizeEq short)
  | @atomic d other payload2 tail argument2 reason enc2 reject =>
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
      (body ++ rest) tail argument argument2 enc enc2 (by simpa only [List.append_assoc] using eqWire)
    subst other; subst argument2; subst tail
    cases bodyValid with
    | atom accepted => cases accepted <;> cases reject <;> simp_all only [List.length_append] <;> word
    | container per items major count bound children => cases reject <;> word
    | tag major number bound child => cases reject <;> word
  | eof => simp only [List.append_assoc, List.cons_append, List.cons_ne_nil] at eqWire
  | @count d other payload2 tail argument2 per enc2 major depth short =>
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
      (body ++ rest) tail argument argument2 enc enc2 (by simpa only [List.append_assoc] using eqWire)
    subst other; subst argument2; subst tail
    cases bodyValid with
    | atom accepted => have := skipAtom_nonrecursive _ _ _ accepted; rcases major with h | h <;> word
    | container per2 items major2 count bound children =>
      have bytes := skipValid_items_bound (d + 1) items children
      rcases major with ⟨major, rfl⟩ | ⟨major, rfl⟩ <;>
        rcases major2 with ⟨major2, rfl⟩ | ⟨major2, rfl⟩ <;>
        simp only [List.length_append] at short <;> word
    | tag major2 number bound child => rcases major with h | h <;> word
  | @containerDepth other payload2 tail argument2 per enc2 major =>
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
      (body ++ rest) tail argument argument2 enc enc2 (by simpa only [List.append_assoc] using eqWire)
    subst other; subst argument2
    cases bodyValid with
    | atom accepted => have := skipAtom_nonrecursive _ _ _ accepted; rcases major with h | h <;> word
    | container per2 items major2 count bound children => omega
    | tag major2 number bound child => rcases major with h | h <;> word
  | @tagEOF d other payload2 argument2 enc2 major number =>
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
      (body ++ rest) [] argument argument2 enc enc2 (by simpa only [List.append_assoc, List.append_nil] using eqWire)
    subst other; subst argument2
    cases bodyValid with
    | atom accepted => exact (skipAtom_nonrecursive _ _ _ accepted).2.2 major
    | container per items major2 count bound children => rcases major2 with h | h <;> word
    | tag major2 number2 bound child => simp at tailEq
  | @tagDepth other next payload2 tail argument2 enc2 major number nextTag =>
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
      (body ++ rest) (next :: tail) argument argument2 enc enc2 (by simpa only [List.append_assoc] using eqWire)
    subst other; subst argument2
    cases bodyValid with
    | atom accepted => exact (skipAtom_nonrecursive _ _ _ accepted).2.2 major
    | container per items major2 count bound children => rcases major2 with h | h <;> word
    | @tag next2 tail2 major2 number2 bound child =>
      have nextEq : next2 = next := (List.cons.inj (by simpa only [List.cons_append] using tailEq)).1
      subst next2
      have := bound nextTag
      omega

/-- A strict successful sibling prefix leaves a complete next item whenever
    the enclosing complete sequence is valid. Exact boundaries follow from
    item determinism, rather than from a count of bytes alone. -/
theorem skipItems_after_prefix (d : Nat) (done items : List (List w8)) (tail rest : List w8)
    (prefixValid : ∀ item ∈ done, SkipValid d item)
    (allValid : ∀ item ∈ items, SkipValid d item)
    (more : done.length < items.length) (same : done.flatten ++ tail = items.flatten ++ rest) :
    ∃ item suffix, tail = item ++ suffix ∧ SkipValid d item := by
  induction done generalizing items with
  | nil =>
    cases items with
    | nil => simp at more
    | cons item items =>
      exact ⟨item, items.flatten ++ rest,
        by simpa only [List.flatten_nil, List.nil_append, List.flatten_cons, List.append_assoc] using same,
        allValid item (by simp)⟩
  | cons item done ih =>
    cases items with
    | nil => simp at more
    | cons other items =>
      obtain ⟨_, tailsEq⟩ := skipValid_prefix_unique d item other
        (done.flatten ++ tail) (items.flatten ++ rest)
        (prefixValid item (by simp)) (allValid other (by simp))
        (by simpa only [List.flatten_cons, List.append_assoc] using same)
      exact ih items (fun x hx => prefixValid x (by simp [hx]))
        (fun x hx => allValid x (by simp [hx])) (by simpa using more) tailsEq

/-- A complete valid enclosing value forces the input at every supported
    failure context to start with a complete valid leaf value. -/
theorem skipValid_context_leaf (tail front item rest : List w8) (d leaf : Nat)
    (context : SkipFailureContext tail d front leaf) (valid : SkipValid d item)
    (same : item ++ rest = front ++ tail) :
    ∃ child suffix, tail = child ++ suffix ∧ SkipValid leaf child := by
  induction context generalizing item rest with
  | nil d => exact ⟨item, rest, by simpa using same.symm, valid⟩
  | @container d leaf head payload front count per items enc major bound more fit children context ih =>
    obtain ⟨other, payload2, argument, body, rfl, enc2, bodyValid⟩ := skipValid_parts d item valid
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique other head payload2 payload
      (body ++ rest) (items.flatten ++ (front ++ tail)) argument count enc2 enc
      (by simpa only [List.append_assoc] using same)
    subst other; subst argument
    cases bodyValid with
    | atom accepted => have := skipAtom_nonrecursive _ _ _ accepted; rcases major with h | h <;> word
    | container per2 allItems major2 count2 bound2 allChildren =>
      have perEq : per = per2 := by
        rcases major with ⟨major, rfl⟩ | ⟨major, rfl⟩ <;>
          rcases major2 with ⟨major2, rfl⟩ | ⟨major2, rfl⟩ <;> word
      subst per2
      have moreItems : items.length < allItems.length := by omega
      obtain ⟨child, suffix, split, childValid⟩ := skipItems_after_prefix (d + 1) items allItems
        (front ++ tail) rest children allChildren moreItems tailEq.symm
      exact ih child suffix childValid split.symm
    | tag major2 number bound2 child => rcases major with h | h <;> word
  | @tag d leaf head next payload front rest2 argument enc major number nonempty bound context ih =>
    obtain ⟨other, payload2, argument2, body, rfl, enc2, bodyValid⟩ := skipValid_parts d item valid
    obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique other head payload2 payload
      (body ++ rest) (front ++ tail) argument2 argument enc2 enc
      (by simpa only [List.append_assoc] using same)
    subst other; subst argument2
    cases bodyValid with
    | atom accepted => exact False.elim ((skipAtom_nonrecursive _ _ _ accepted).2.2 major)
    | container per items major2 count bound2 children => rcases major2 with h | h <;> word
    | @tag next2 childTail major2 number2 bound2 child =>
      have nextEq : next2 = next :=
        (List.cons.inj (by simpa only [List.cons_append] using tailEq.trans nonempty)).1
      subst next2
      exact ih _ rest child tailEq

/-- Complete separation of recursive Skip success and every specified error,
    including failures after arbitrary valid siblings and nested contexts. -/
theorem skipValid_not_failure (d offset : Nat) (item rest : List w8) (reason : SkipFailureReason)
    (valid : SkipValid d item) (failure : SkipFailure d (item ++ rest) offset reason) : False := by
  generalize eqWire : item ++ rest = wire at failure
  cases failure with
  | @within front tail leaf offset reason context failure =>
    obtain ⟨child, suffix, split, childValid⟩ := skipValid_context_leaf tail front item rest d leaf context valid eqWire
    subst tail
    exact skipValid_not_failureLeaf leaf offset child suffix reason childValid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_parts
#print axioms Perennial.github_com.NethermindEth.juno.core.encodedHead_prefix_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_not_failureLeaf
#print axioms Perennial.github_com.NethermindEth.juno.core.skipItems_after_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_context_leaf
#print axioms Perennial.github_com.NethermindEth.juno.core.skipValid_not_failure
