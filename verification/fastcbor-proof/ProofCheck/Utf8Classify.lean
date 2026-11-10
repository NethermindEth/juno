import ProofCheck.Utf8Spec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

abbrev Utf8Two (a b : w8) : Prop :=
  (194 ≤ uint.Z a ∧ uint.Z a ≤ 223) ∧ (128 ≤ uint.Z b ∧ uint.Z b ≤ 191)

abbrev Utf8Three (a b c : w8) : Prop :=
  ((a = W8 224 ∧ 160 ≤ uint.Z b ∧ uint.Z b ≤ 191) ∨
   (((225 ≤ uint.Z a ∧ uint.Z a ≤ 236) ∨ (238 ≤ uint.Z a ∧ uint.Z a ≤ 239)) ∧
     128 ≤ uint.Z b ∧ uint.Z b ≤ 191) ∨
   (a = W8 237 ∧ 128 ≤ uint.Z b ∧ uint.Z b ≤ 159)) ∧
  (128 ≤ uint.Z c ∧ uint.Z c ≤ 191)

abbrev Utf8Four (a b c d : w8) : Prop :=
  ((a = W8 240 ∧ 144 ≤ uint.Z b ∧ uint.Z b ≤ 191) ∨
   ((241 ≤ uint.Z a ∧ uint.Z a ≤ 243) ∧ 128 ≤ uint.Z b ∧ uint.Z b ≤ 191) ∨
   (a = W8 244 ∧ 128 ≤ uint.Z b ∧ uint.Z b ≤ 143)) ∧
  (128 ≤ uint.Z c ∧ uint.Z c ≤ 191) ∧ (128 ≤ uint.Z d ∧ uint.Z d ≤ 191)

/-- An independent single-character classifier. It consumes exactly one valid
    UTF-8 character, or rejects an illegal/truncated character. It does not call
    Go's tables or implementation, and does not replace their proof obligations. -/
def utf8Next : List w8 → Option (List w8)
  | [] => none
  | a :: rest =>
    if uint.Z a < 128 then some rest else
    match rest with
    | [] => none
    | b :: rest =>
      if Utf8Two a b then some rest else
      match rest with
      | [] => none
      | c :: rest =>
        if Utf8Three a b c then some rest else
        match rest with
        | [] => none
        | d :: rest => if Utf8Four a b c d then some rest else none

/-- Exact one-character decomposition of the independent RFC byte grammar. -/
theorem utf8Valid_step (a : w8) (rest : List w8) :
    Utf8Valid (a :: rest) ↔ ∃ next, utf8Next (a :: rest) = some next ∧ Utf8Valid next := by
  constructor
  · intro valid
    cases valid with
    | ascii h hx => exact ⟨_, by simp [utf8Next, h], hx⟩
    | two h0 h1 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Tail, uint.Z] <;> omega
    | threeLow h1 h2 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Tail, uint.Z] <;> omega
    | three h0 h1 h2 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Tail, uint.Z] <;> omega
    | threeSurrogate h1 h2 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Tail, uint.Z] <;> omega
    | fourLow h1 h2 h3 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Four, Utf8Tail, uint.Z] <;> omega
    | four h0 h1 h2 h3 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Four, Utf8Tail, uint.Z] <;> word
    | fourHigh h1 h2 h3 hx =>
      refine ⟨_, ?_, hx⟩
      simp only [utf8Next]
      (repeat' split) <;> simp_all [Utf8Two, Utf8Three, Utf8Four, Utf8Tail, uint.Z] <;> omega
  · rintro ⟨next, step, valid⟩
    by_cases h0 : uint.Z a < 128
    · simp only [utf8Next, ite_eq_left h0, Option.some.injEq] at step
      subst next
      exact .ascii h0 valid
    · cases rest with
      | nil => simp [utf8Next, h0] at step
      | cons b rest =>
        by_cases h1 : Utf8Two a b
        · simp only [utf8Next, ite_eq_right h0, ite_eq_left h1, Option.some.injEq] at step
          subst next
          exact .two h1.1 h1.2 valid
        · cases rest with
          | nil => simp [utf8Next, h0, h1] at step
          | cons c rest =>
            by_cases h2 : Utf8Three a b c
            · simp only [utf8Next, ite_eq_right h0, ite_eq_right h1, ite_eq_left h2, Option.some.injEq] at step
              subst next
              rcases h2 with ⟨⟨rfl, hb⟩ | ⟨ha, hb⟩ | ⟨rfl, hb⟩, hc⟩
              · exact .threeLow hb hc valid
              · exact .three ha hb hc valid
              · exact .threeSurrogate hb hc valid
            · cases rest with
              | nil => simp [utf8Next, h0, h1, h2] at step
              | cons d rest =>
                by_cases h3 : Utf8Four a b c d
                · simp only [utf8Next, ite_eq_right h0, ite_eq_right h1, ite_eq_right h2, ite_eq_left h3, Option.some.injEq] at step
                  subst next
                  rcases h3 with ⟨⟨rfl, hb⟩ | ⟨ha, hb⟩ | ⟨rfl, hb⟩, hc, hd⟩
                  · exact .fourLow hb hc hd valid
                  · exact .four ha hb hc hd valid
                  · exact .fourHigh hb hc hd valid
                · simp [utf8Next, h0, h1, h2, h3] at step

/-- Every successful classifier step consumes between one and four bytes. -/
theorem utf8Next_progress (xs next : List w8) (h : utf8Next xs = some next) :
    next.length < xs.length ∧ xs.length ≤ next.length + 4 := by
  unfold utf8Next at h
  repeat' split at h
  all_goals simp_all
  all_goals omega

/-- Bounded specification evaluator. A nonempty successful step consumes at
    least one byte, so an input-length budget suffices for every input. -/
def utf8CheckFuel : Nat → List w8 → Bool
  | _, [] => true
  | 0, _ :: _ => false
  | fuel + 1, a :: rest =>
    match utf8Next (a :: rest) with
    | none => false
    | some next => utf8CheckFuel fuel next

/-- A total independent decision procedure. This is specification machinery,
    not a theorem about Go's validator or its termination. -/
def utf8Check (xs : List w8) : Bool := utf8CheckFuel xs.length xs

theorem utf8CheckFuel_spec (fuel : Nat) (xs : List w8) (bound : xs.length ≤ fuel) :
    utf8CheckFuel fuel xs = true ↔ Utf8Valid xs := by
  induction fuel generalizing xs with
  | zero =>
    have hx : xs = [] := List.eq_nil_of_length_eq_zero (by omega)
    subst xs
    change true = true ↔ Utf8Valid []
    exact ⟨fun _ => .empty, fun _ => rfl⟩
  | succ fuel ih =>
    cases xs with
    | nil =>
      change true = true ↔ Utf8Valid []
      exact ⟨fun _ => .empty, fun _ => rfl⟩
    | cons a rest =>
      have step := utf8Valid_step a rest
      change (match utf8Next (a :: rest) with
        | none => false
        | some next => utf8CheckFuel fuel next) = true ↔ Utf8Valid (a :: rest)
      cases h : utf8Next (a :: rest) with
      | none =>
        simp only [h, reduceCtorEq, false_and, exists_false] at step
        simpa only [Bool.false_eq_true] using step.symm
      | some next =>
        have hn := ih next (by have := (utf8Next_progress _ _ h).1; omega)
        simp only [h, Option.some.injEq] at step
        apply hn.trans
        constructor
        · intro valid
          exact step.mpr ⟨next, rfl, valid⟩
        · intro valid
          obtain ⟨ys, hys, hy⟩ := step.mp valid
          exact hys ▸ hy

theorem utf8Check_spec (xs : List w8) : utf8Check xs = true ↔ Utf8Valid xs :=
  utf8CheckFuel_spec xs.length xs (Nat.le_refl _)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Valid_step
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Next_progress
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Check_spec

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8CheckFuel_spec
