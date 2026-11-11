import ProofCheck.SchemaKeySpec
import ProofCheck.SkipClassify

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Independent projection-name interpretation: an index names its folded
    schema entry; an unknown key matches no entry. -/
def WantedKeyMeaning (names : List GoString) (key : GoString) (choice : Option Nat) : Prop :=
  match choice with
  | none => ∀ name ∈ names, key.map fieldFoldByte ≠ name.map fieldFoldByte
  | some index => ∃ name, names[index]? = some name ∧ key.map fieldFoldByte = name.map fieldFoldByte

theorem wantedKeyMeaning_total (names : List GoString) (key : GoString) :
    ∃ choice, WantedKeyMeaning names key choice := by
  classical
  by_cases known : ∃ (index : Nat) (name : GoString), names[index]? = some name ∧ key.map fieldFoldByte = name.map fieldFoldByte
  · obtain ⟨index, name, lookup, folded⟩ := known
    exact ⟨some index, name, lookup, folded⟩
  · refine ⟨none, ?_⟩
    intro name member folded
    obtain ⟨index, lookup⟩ := List.mem_iff_getElem?.mp member
    exact known ⟨index, name, lookup, folded⟩

theorem wantedKeyMeaning_unique (names : List GoString) (key : GoString) (a b : Option Nat)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (first : WantedKeyMeaning names key a) (second : WantedKeyMeaning names key b) : a = b := by
  cases a with
  | none =>
    cases b with
    | none => rfl
    | some index =>
      obtain ⟨name, lookup, folded⟩ := second
      exact False.elim (first name (List.mem_of_getElem? lookup) folded)
  | some index =>
    obtain ⟨name, lookup, folded⟩ := first
    cases b with
    | none => exact False.elim (second name (List.mem_of_getElem? lookup) folded)
    | some other =>
      obtain ⟨name2, lookup2, folded2⟩ := second
      have equal : (names.map (List.map fieldFoldByte))[index]? = (names.map (List.map fieldFoldByte))[other]? := by
        simp only [List.getElem?_map, lookup, lookup2, Option.map_some]
        rw [← folded, ← folded2]
      have bound := (List.getElem?_eq_some_iff.mp lookup).choose
      exact congrArg some ((distinct.getElem?_inj (by simpa only [List.length_map] using bound)).mp equal)

abbrev WantedKeyValid (names : List GoString) := SchemaKeyValid (WantedKeyMeaning names)

/-- Selection is independent of machine masks and preserves first occurrence. -/
def wantedSelect (seen : List Nat) (choice : Option Nat) : Option Nat :=
  match choice with
  | none => none
  | some index => if index ∈ seen then none else some index

theorem wantedSelect_fresh (seen : List Nat) (choice : Option Nat) (index : Nat)
    (selected : wantedSelect seen choice = some index) : choice = some index ∧ index ∉ seen := by
  cases choice with
  | none => simp [wantedSelect] at selected
  | some field =>
    by_cases old : field ∈ seen
    · simp [wantedSelect, old] at selected
    · have eq : field = index := by simpa [wantedSelect, old] using selected
      subst field
      exact ⟨rfl, old⟩

inductive WantedResult where
  | done
  | field (index left : Nat)

/-- Bytes consumed by a successful NextWanted scan. A selected key stops before
    its value; otherwise complete unknown/duplicate pairs are skipped. Done
    consumes all remaining pairs. Container closure is a separate state update. -/
inductive WantedValid (d : Nat) (names : List GoString) (seen : List Nat) :
    Nat → List w8 → WantedResult → Prop where
  | done : WantedValid d names seen 0 [] .done
  | selected {n index : Nat} {key : List w8} {choice : Option Nat}
      (valid : WantedKeyValid names key choice) (fresh : wantedSelect seen choice = some index) :
      WantedValid d names seen (n + 1) key (.field index n)
  | skipped {n : Nat} {key value tail : List w8} {choice : Option Nat} {result : WantedResult}
      (keyValid : WantedKeyValid names key choice) (ignored : wantedSelect seen choice = none)
      (valueValid : SkipValid d value) (rest : WantedValid d names seen n tail result) :
      WantedValid d names seen (n + 1) (key ++ value ++ tail) result

inductive WantedFailureReason where
  | key (reason : KeyFailureReason)
  | skipped (reason : SkipFailureReason)

/-- A failed key leaves the cursor at the key; a failed Skip leaves it at the
    value but may diagnose deeper bytes. Neither decrements the current pair.
    Successful ignored pairs before an error do decrement the remaining count. -/
inductive WantedRejected (d : Nat) (names : List GoString) (seen : List Nat) :
    Nat → List w8 → Nat → Nat → Nat → WantedFailureReason → Prop where
  | key {n : Nat} {wire : List w8} {reason : KeyFailureReason}
      (failure : KeyRejected wire reason) :
      WantedRejected d names seen (n + 1) wire 0 0 (n + 1) (.key reason)
  | skipped {n offset : Nat} {key tail : List w8} {choice : Option Nat} {reason : SkipFailureReason}
      (valid : WantedKeyValid names key choice) (ignored : wantedSelect seen choice = none)
      (failure : SkipFailure d tail offset reason) :
      WantedRejected d names seen (n + 1) (key ++ tail) key.length (key.length + offset) (n + 1) (.skipped reason)
  | next {n consumed offset left : Nat} {key value tail : List w8} {choice : Option Nat} {reason : WantedFailureReason}
      (keyValid : WantedKeyValid names key choice) (ignored : wantedSelect seen choice = none)
      (valueValid : SkipValid d value)
      (failure : WantedRejected d names seen n tail consumed offset left reason) :
      WantedRejected d names seen (n + 1) (key ++ value ++ tail)
        (key.length + value.length + consumed) (key.length + value.length + offset) left reason

/-- Complete scan classification permits arbitrary byte input, names and seen
    fields; the depth bound is the same one required by recursive Skip. -/
theorem wanted_classify (d n : Nat) (names : List GoString) (seen : List Nat) (wire : List w8)
    (depth : d ≤ 32) :
    (∃ item rest result, wire = item ++ rest ∧ WantedValid d names seen n item result) ∨
    (∃ consumed offset left reason, WantedRejected d names seen n wire consumed offset left reason) := by
  induction n generalizing wire with
  | zero => exact Or.inl ⟨[], wire, .done, rfl, .done⟩
  | succ n ih =>
    rcases schemaKey_classify (WantedKeyMeaning names) (wantedKeyMeaning_total names) wire with
      ⟨key, tail, choice, split, valid⟩ | ⟨reason, failure⟩
    · subst wire
      cases selected : wantedSelect seen choice with
      | some index => exact Or.inl ⟨key, tail, .field index n, rfl, .selected valid selected⟩
      | none =>
        rcases skip_classify d tail depth with ⟨value, rest, split, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih rest with ⟨items, suffix, result, split, restValid⟩ | ⟨consumed, offset, left, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, result, by simp only [List.append_assoc],
              .skipped valid selected valueValid restValid⟩
          · refine Or.inr ⟨key.length + value.length + consumed, key.length + value.length + offset, left, reason, ?_⟩
            simpa only [List.append_assoc] using WantedRejected.next valid selected valueValid failure
        · exact Or.inr ⟨_, _, n + 1, .skipped reason, .skipped valid selected failure⟩
    · exact Or.inr ⟨0, 0, n + 1, .key reason, .key failure⟩

/-- Diagnostic position and retained consumption are bounded independently:
    Skip can roll back while reporting an offset beyond the retained cursor. -/
theorem wantedRejected_bounds (d n consumed offset left : Nat) (names : List GoString) (seen : List Nat)
    (wire : List w8) (reason : WantedFailureReason)
    (failure : WantedRejected d names seen n wire consumed offset left reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ 0 < left ∧ left ≤ n := by
  induction failure with
  | key failure => simp
  | skipped valid ignored failure =>
    have := skipFailure_offset _ _ _ _ failure
    simp only [List.length_append]
    omega
  | next keyValid ignored valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem wantedValid_selected (d n index left : Nat) (names : List GoString) (seen : List Nat)
    (wire : List w8) (valid : WantedValid d names seen n wire (.field index left)) :
    index < names.length ∧ index ∉ seen ∧ left < n := by
  induction n generalizing wire index left with
  | zero => cases valid
  | succ n ih =>
    cases valid with
    | selected valid fresh =>
      obtain ⟨rfl, unseen⟩ := wantedSelect_fresh seen _ index fresh
      obtain ⟨value, _, meaning⟩ := valid
      cases value with
      | none => simp at meaning
      | some key =>
        obtain ⟨name, lookup, _⟩ := meaning
        have bound := (List.getElem?_eq_some_iff.mp lookup).choose
        exact ⟨bound, unseen, by omega⟩
    | skipped keyValid ignored valueValid rest =>
      have result := ih _ _ _ rest
      exact ⟨result.1, result.2.1, by omega⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedKeyMeaning_total
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedKeyMeaning_unique
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSelect_fresh
#print axioms Perennial.github_com.NethermindEth.juno.core.wanted_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedValid_selected
