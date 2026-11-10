import ProofCheck.SchemaKeySpec
import ProofCheck.GasPriceFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Known amounts decode felts; unknown keys skip exactly one supported item.
    This relation uses decoded field identities, not a planner or cache layout. -/
inductive GasValueValid (d : Nat) : Option Bool → GasPriceValue → List w8 → GasPriceValue → Prop where
  | amount {wei : Bool} {initial : GasPriceValue} {wire : List w8} {value : Option (GoArray w64 4)}
      (valid : FeltValid d wire value) :
      GasValueValid d (some wei) initial wire (gasPriceValueSet wei initial value)
  | ignored {initial : GasPriceValue} {wire : List w8} (valid : SkipValid d wire) :
      GasValueValid d none initial wire initial

def GasChoiceFresh (seen : List Bool) (choice : Option Bool) : Prop :=
  match choice with
  | none => True
  | some wei => wei ∉ seen

def gasChoiceSeen (seen : List Bool) (choice : Option Bool) : List Bool :=
  match choice with
  | none => seen
  | some wei => wei :: seen

/-- Successful decoding of exactly n pairs, retaining their exact byte prefix.
    Unknown keys may repeat; known fields may appear in either order but once. -/
inductive GasPairsValid (d : Nat) : Nat → List Bool → GasPriceValue → List w8 → GasPriceValue → Prop where
  | nil (seen : List Bool) (initial : GasPriceValue) : GasPairsValid d 0 seen initial [] initial
  | step {n : Nat} {seen : List Bool} {choice : Option Bool} {initial middle final : GasPriceValue}
      {key value tail : List w8} (keyValid : GasKeyValid key choice) (fresh : GasChoiceFresh seen choice)
      (valueValid : GasValueValid d choice initial value middle)
      (rest : GasPairsValid d n (gasChoiceSeen seen choice) middle tail final) :
      GasPairsValid d (n + 1) seen initial (key ++ value ++ tail) final

/-- Complete failure during pair decoding, with the partial record, cursor
    consumption and diagnostic offset. Failed values restore their own cursor;
    duplicate errors occur after consuming the key and before reading its value. -/
inductive GasPairsRejected (d : Nat) : Nat → List Bool → GasPriceValue → List w8 →
    GasPriceValue → Nat → Nat → GasPriceFailureReason → Prop where
  | key {n : Nat} {seen : List Bool} {initial : GasPriceValue} {wire : List w8} {reason : KeyFailureReason}
      (failure : KeyRejected wire reason) :
      GasPairsRejected d (n + 1) seen initial wire initial 0 0 (.key reason)
  | duplicate {n : Nat} {seen : List Bool} {initial : GasPriceValue} {key tail : List w8} {wei : Bool}
      (valid : GasKeyValid key (some wei)) (duplicate : wei ∈ seen) :
      GasPairsRejected d (n + 1) seen initial (key ++ tail) initial key.length key.length (.duplicate wei)
  | value {n offset : Nat} {seen : List Bool} {choice : Option Bool} {initial : GasPriceValue}
      {key tail : List w8} {reason : FeltFailureReason}
      (valid : GasKeyValid key choice) (fresh : GasChoiceFresh seen choice)
      (failure : GasPriceValueFailure d choice tail offset reason) :
      GasPairsRejected d (n + 1) seen initial (key ++ tail) initial key.length (key.length + offset) (.inner reason)
  | next {n consumed offset : Nat} {seen : List Bool} {choice : Option Bool}
      {initial middle final : GasPriceValue} {key value tail : List w8} {reason : GasPriceFailureReason}
      (keyValid : GasKeyValid key choice) (fresh : GasChoiceFresh seen choice)
      (valueValid : GasValueValid d choice initial value middle)
      (failure : GasPairsRejected d n (gasChoiceSeen seen choice) middle tail final consumed offset reason) :
      GasPairsRejected d (n + 1) seen initial (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

/-- Every value has an independently specified felt/Skip outcome. -/
theorem gasValue_classify (d : Nat) (choice : Option Bool) (initial : GasPriceValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasValueValid d choice initial item final) ∨
    (∃ offset reason, GasPriceValueFailure d choice wire offset reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, initial, split, .ignored valid⟩
    · exact Or.inr ⟨offset, .skipped reason, .ignored failure⟩
  | some wei =>
    rcases felt_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, gasPriceValueSet wei initial value, split, .amount valid⟩
    · exact Or.inr ⟨offset, reason, .amount failure⟩

/-- All pair sequences are classified, including errors after an arbitrary
    successful prefix. The count decreases even when unknown names repeat. -/
theorem gasPairs_classify (d n : Nat) (seen : List Bool) (initial : GasPriceValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasPairsValid d n seen initial item final) ∨
    (∃ final consumed offset reason, GasPairsRejected d n seen initial wire final consumed offset reason) := by
  classical
  induction n generalizing seen initial wire with
  | zero => exact Or.inl ⟨[], wire, initial, rfl, .nil seen initial⟩
  | succ n ih =>
    rcases gasKey_classify wire with ⟨key, tail, choice, split, valid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : GasChoiceFresh seen choice
      · rcases gasValue_classify d choice initial tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
        · subst tail
          rcases ih (gasChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step valid fresh valueValid validRest⟩
          · apply Or.inr
            refine ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
            simpa only [List.append_assoc] using GasPairsRejected.next valid fresh valueValid failure
        · exact Or.inr ⟨initial, key.length, key.length + offset, .inner reason, .value valid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some wei => exact Or.inr ⟨initial, key.length, key.length, .duplicate wei,
            .duplicate valid (by simpa only [GasChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨initial, 0, 0, .key reason, .key failure⟩

/-- Complete direct GasPrice record success. Null/undefined leave the previous
    struct contents untouched; the nullable pointer wrapper is a separate layer. -/
inductive GasRecordValid (d : Nat) (initial : GasPriceValue) : List w8 → GasPriceValue → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : GasRecordValid d initial [head] initial
  | map {head : w8} {payload body : List w8} {argument : w64} {count : Nat} {final : GasPriceValue}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : GasPairsValid (d + 1) count [] initial body final) :
      GasRecordValid d initial ((head :: payload) ++ body) final

/-- Opening errors precede pair processing; a map failure retains exactly its
    partial contents and one entered depth level, with offsets relative to the
    record start. Count preflight includes the entire unread suffix, as in Go. -/
inductive GasRecordRejected (d : Nat) (initial : GasPriceValue) : List w8 → GasPriceValue →
    Nat → Nat → Bool → GasPriceFailureReason → Prop where
  | opening {wire : List w8} {reason : FeltFailureReason} (failure : StructOpenFailure d wire reason) :
      GasRecordRejected d initial wire initial 0 0 false (.inner reason)
  | map {head : w8} {payload body : List w8} {argument : w64} {count consumed offset : Nat}
      {final : GasPriceValue} {reason : GasPriceFailureReason}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : GasPairsRejected (d + 1) count [] initial body final consumed offset reason) :
      GasRecordRejected d initial ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) true reason

/-- Every input has a complete GasPrice record outcome under the nesting limit.
    This is a specification theorem; no production key or validator WP is assumed. -/
theorem gasRecord_classify (d : Nat) (initial : GasPriceValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ GasRecordValid d initial item final) ∨
    (∃ final consumed offset entered reason,
      GasRecordRejected d initial wire final consumed offset entered reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, initial, split, .nullish nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases gasPairs_classify (d + 1) count [] initial body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, true, reason,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨initial, 0, 0, false, .inner reason, .opening failure⟩

theorem gasValue_nonempty (d : Nat) (choice : Option Bool) (initial final : GasPriceValue)
    (wire : List w8) (valid : GasValueValid d choice initial wire final) : 0 < wire.length := by
  cases valid with
  | amount valid =>
    cases valid <;> simp only [List.length_append, List.length_cons, List.length_nil] <;> omega
  | ignored valid => exact skipValid_nonempty d wire valid

/-- Every successful pair consumes at least one key byte and one value byte,
    so successful maps satisfy the production pair-count preflight. -/
theorem gasPairs_count_bound (d n : Nat) (seen : List Bool) (initial final : GasPriceValue)
    (wire : List w8) (valid : GasPairsValid d n seen initial wire final) : 2 * n ≤ wire.length := by
  induction valid with
  | nil => simp
  | @step n seen choice initial middle final key value tail keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty GasKeyTextMeaning key choice keyValid
    have hv := gasValue_nonempty d choice initial middle value valueValid
    simp only [List.length_append]
    omega

/-- Pair failure consumption precedes its diagnostic, and both stay within
    the unread suffix (including the EOF position). -/
theorem gasPairsRejected_bounds (d n consumed offset : Nat) (seen : List Bool)
    (initial final : GasPriceValue) (wire : List w8) (reason : GasPriceFailureReason)
    (failure : GasPairsRejected d n seen initial wire final consumed offset reason) :
    consumed ≤ offset ∧ offset ≤ wire.length := by
  induction failure with
  | key => simp
  | duplicate => simp
  | value valid fresh failure =>
    have bound := gasPriceValueFailure_offset _ _ _ _ _ failure
    simp only [List.length_append]
    omega
  | next keyValid fresh valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem gasRecord_nonempty (d : Nat) (initial final : GasPriceValue) (wire : List w8)
    (valid : GasRecordValid d initial wire final) : 0 < wire.length := by
  cases valid <;> simp

/-- Every complete record failure has safe relative offsets and retains at
    most the map's own depth increment. Child failures have already rolled back. -/
theorem gasRecordRejected_bounds (d consumed offset : Nat) (entered : Bool)
    (initial final : GasPriceValue) (wire : List w8) (reason : GasPriceFailureReason)
    (failure : GasRecordRejected d initial wire final consumed offset entered reason) (depth : d ≤ 32) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + (if entered then 1 else 0) ≤ 32 := by
  cases failure with
  | opening failure => simp; omega
  | map enc major length hd fit failure =>
    have bound := gasPairsRejected_bounds _ _ _ _ _ _ _ _ _ failure
    simp only [List.length_append, List.length_cons, ite_true]
    omega

def gasAmount (wei : Bool) (value : GasPriceValue) : Option (GoArray w64 4) :=
  if wei then value.wei else value.fri

/-- A fresh field or an ignored value cannot overwrite an amount already seen. -/
theorem gasValue_preserves_seen (d : Nat) (seen : List Bool) (choice : Option Bool)
    (wei : Bool) (initial final : GasPriceValue) (wire : List w8)
    (member : wei ∈ seen) (fresh : GasChoiceFresh seen choice)
    (valid : GasValueValid d choice initial wire final) : gasAmount wei final = gasAmount wei initial := by
  cases valid with
  | @amount other initial wire value valid =>
    have different : other ≠ wei := by intro same; subst other; exact fresh member
    cases wei <;> cases other <;> simp_all [gasAmount, gasPriceValueSet]
  | ignored => rfl

/-- Successfully decoding the remaining pairs preserves all previously seen
    amounts, regardless of ordering, unknown fields or nullish felt values. -/
theorem gasPairs_preserves_seen (d n : Nat) (seen : List Bool) (wei : Bool)
    (initial final : GasPriceValue) (wire : List w8) (valid : GasPairsValid d n seen initial wire final)
    (member : wei ∈ seen) : gasAmount wei final = gasAmount wei initial := by
  revert member
  induction valid with
  | nil => intro _; rfl
  | @step n seen choice initial middle final key value tail keyValid fresh valueValid rest ih =>
    intro member
    have remains : wei ∈ gasChoiceSeen seen choice := by cases choice <;> simp [gasChoiceSeen, member]
    exact (ih remains).trans (gasValue_preserves_seen d seen choice wei initial middle value member fresh valueValid)

/-- The same retention holds on every failure, including a later duplicate or
    malformed nested value after successful intervening fields. -/
theorem gasPairsRejected_preserves_seen (d n consumed offset : Nat) (seen : List Bool) (wei : Bool)
    (initial final : GasPriceValue) (wire : List w8) (reason : GasPriceFailureReason)
    (failure : GasPairsRejected d n seen initial wire final consumed offset reason)
    (member : wei ∈ seen) : gasAmount wei final = gasAmount wei initial := by
  revert member
  induction failure with
  | key => intro _; rfl
  | duplicate => intro _; rfl
  | value => intro _; rfl
  | @next n consumed offset seen choice initial middle final key value tail reason keyValid fresh valueValid failure ih =>
    intro member
    have remains : wei ∈ gasChoiceSeen seen choice := by cases choice <;> simp [gasChoiceSeen, member]
    exact (ih remains).trans (gasValue_preserves_seen d seen choice wei initial middle value member fresh valueValid)

/-- The Header's nullable price field distinguishes nil from an allocated
    record whose two amounts are nil. Each non-null record starts from zero. -/
inductive GasPointerValid (d : Nat) : List w8 → Option GasPriceValue → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : GasPointerValid d [head] none
  | map {head : w8} {payload body : List w8} {argument : w64} {count : Nat} {final : GasPriceValue}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : GasPairsValid (d + 1) count [] ⟨none, none⟩ body final) :
      GasPointerValid d ((head :: payload) ++ body) (some final)

/-- The nullable price wrapper also has an outcome for every input. Failed
    non-null records retain their private partial value until caller cleanup. -/
theorem gasPointer_classify (d : Nat) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest value, wire = item ++ rest ∧ GasPointerValid d item value) ∨
    (∃ retained consumed offset entered reason,
      GasRecordRejected d ⟨none, none⟩ wire retained consumed offset entered reason) := by
  rcases gasRecord_classify d ⟨none, none⟩ wire depth with ⟨item, rest, final, split, valid⟩ | failure
  · cases valid with
    | nullish hn => exact Or.inl ⟨_, rest, none, split, .nullish hn⟩
    | map enc major length depth valid => exact Or.inl ⟨_, rest, some final, split, .map enc major length depth valid⟩
  · exact Or.inr failure

theorem gasPointer_nonempty (d : Nat) (wire : List w8) (value : Option GasPriceValue)
    (valid : GasPointerValid d wire value) : 0 < wire.length := by
  cases valid <;> simp

/-- No specified record failure is intercepted by the wrapper's null check. -/
theorem gasRecordRejected_not_nullish (d consumed offset : Nat) (entered : Bool)
    (initial final : GasPriceValue) (wire : List w8) (reason : GasPriceFailureReason)
    (failure : GasRecordRejected d initial wire final consumed offset entered reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | opening failure =>
    exact gasPriceFailure_not_nullish d 0 0 false wire initial initial _ (.opening initial failure)
  | map enc major length depth fit failure =>
    constructor <;> intro h <;> have hh := Option.some.inj h <;> rw [hh] at major <;> contradiction

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.gasValue_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecord_classify

#print axioms Perennial.github_com.NethermindEth.juno.core.gasValue_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairs_count_bound

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairsRejected_bounds

#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecord_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordRejected_bounds

#print axioms Perennial.github_com.NethermindEth.juno.core.gasValue_preserves_seen

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairs_preserves_seen

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPairsRejected_preserves_seen

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPointer_classify

#print axioms Perennial.github_com.NethermindEth.juno.core.gasPointer_nonempty

#print axioms Perennial.github_com.NethermindEth.juno.core.gasRecordRejected_not_nullish
