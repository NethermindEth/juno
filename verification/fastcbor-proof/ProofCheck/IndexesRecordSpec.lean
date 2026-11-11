import ProofCheck.IntegerKeySpec
import ProofCheck.IntsFailureSpec
import ProofCheck.SkipClassify
import ProofCheck.StructOpenSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The stored signed offsets, distinguishing nil from allocated empty slices. -/
structure IndexesValue where
  transactions : Option (List Int)
  receipts : Option (List Int)
  deriving DecidableEq

def indexesZeroValue : IndexesValue := ⟨none, none⟩

/-- False selects transactions (key 1); true selects receipts (key 2). -/
def indexesSet (old : IndexesValue) (field : Bool) (value : Option (List Int)) : IndexesValue :=
  if field then {old with receipts := value} else {old with transactions := value}

def indexesChoice (seen : List Bool) (key : Option w64) : Option Bool :=
  match key with
  | none => none
  | some number =>
    if number = W64 1 ∧ false ∉ seen then some false
    else if number = W64 2 ∧ true ∉ seen then some true
    else none

def indexesSeen (seen : List Bool) (choice : Option Bool) : List Bool :=
  match choice with
  | none => seen
  | some field => field :: seen

/-- Duplicate and unknown values obey the ignored-value contract, even when
    their shape differs from the selected signed-array fields. -/
inductive IndexesItemValid (d : Nat) : Option Bool → IndexesValue → List w8 → IndexesValue → Prop where
  | ignored {old : IndexesValue} {wire : List w8} (valid : SkipValid d wire) :
      IndexesItemValid d none old wire old
  | selected {old : IndexesValue} {field : Bool} {wire : List w8} {value : Option (List Int)}
      (valid : IntsValid d wire value) :
      IndexesItemValid d (some field) old wire (indexesSet old field value)

inductive IndexesItemFailureReason where
  | ignored (reason : SkipFailureReason)
  | selected (reason : IntsFailureReason)

/-- Failed array replacements leave the old published field unchanged. Skip
    restores its starting cursor but can report a deeper diagnostic offset. -/
inductive IndexesItemRejected (d : Nat) : Option Bool → List w8 → Nat → Nat → Nat → IndexesItemFailureReason → Prop where
  | ignored {wire : List w8} {offset : Nat} {reason : SkipFailureReason}
      (failure : SkipFailure d wire offset reason) :
      IndexesItemRejected d none wire 0 offset 0 (.ignored reason)
  | selected {field : Bool} {wire : List w8} {consumed offset : Nat} {entered : Bool} {reason : IntsFailureReason}
      (failure : IntsRejected d wire consumed offset entered reason) :
      IndexesItemRejected d (some field) wire consumed offset (if entered then 1 else 0) (.selected reason)

theorem indexesItem_classify (d : Nat) (choice : Option Bool) (initial : IndexesValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ IndexesItemValid d choice initial item final) ∨
    (∃ consumed offset extraDepth reason, IndexesItemRejected d choice wire consumed offset extraDepth reason) := by
  cases choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, initial, split, .ignored valid⟩
    · exact Or.inr ⟨0, offset, 0, .ignored reason, .ignored failure⟩
  | some field =>
    rcases ints_classify d wire depth with ⟨item, rest, value, split, valid⟩ | ⟨consumed, offset, entered, reason, failure⟩
    · exact Or.inl ⟨item, rest, indexesSet initial field value, split, .selected valid⟩
    · exact Or.inr ⟨consumed, offset, (if entered then 1 else 0), .selected reason, .selected failure⟩

theorem indexesItem_nonempty (d : Nat) (choice : Option Bool) (initial final : IndexesValue)
    (wire : List w8) (valid : IndexesItemValid d choice initial wire final) : 0 < wire.length := by
  cases valid with
  | ignored valid => exact skipValid_nonempty _ _ valid
  | selected valid => cases valid <;> simp

theorem indexesItemRejected_bounds (d consumed offset extraDepth : Nat) (choice : Option Bool)
    (wire : List w8) (reason : IndexesItemFailureReason) (depth : d ≤ 32)
    (failure : IndexesItemRejected d choice wire consumed offset extraDepth reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | ignored failure =>
    have := skipFailure_offset _ _ _ _ failure
    omega
  | selected failure =>
    have := intsRejected_bounds _ _ _ _ _ _ failure
    have := intsRejected_depth _ _ _ _ _ _ depth failure
    omega

inductive IndexesPairsValid (d : Nat) : Nat → List Bool → IndexesValue → List w8 → IndexesValue → Prop where
  | nil (seen : List Bool) (initial : IndexesValue) : IndexesPairsValid d 0 seen initial [] initial
  | step {n : Nat} {seen : List Bool} {decoded : Option w64} {initial middle final : IndexesValue}
      {key value tail : List w8} (keyValid : IntegerKeyValid key decoded)
      (valueValid : IndexesItemValid d (indexesChoice seen decoded) initial value middle)
      (rest : IndexesPairsValid d n (indexesSeen seen (indexesChoice seen decoded)) middle tail final) :
      IndexesPairsValid d (n + 1) seen initial (key ++ value ++ tail) final

inductive IndexesRecordFailureReason where
  | opening (reason : FeltFailureReason)
  | key (reason : IntegerKeyFailureReason)
  | value (reason : IndexesItemFailureReason)

inductive IndexesPairsRejected (d : Nat) : Nat → List Bool → IndexesValue → List w8 →
    IndexesValue → Nat → Nat → Nat → IndexesRecordFailureReason → Prop where
  | key {n consumed : Nat} {seen : List Bool} {initial : IndexesValue} {wire : List w8} {reason : IntegerKeyFailureReason}
      (failure : IntegerKeyRejected wire consumed reason) :
      IndexesPairsRejected d (n + 1) seen initial wire initial consumed 0 0 (.key reason)
  | value {n consumed offset extraDepth : Nat} {seen : List Bool} {decoded : Option w64}
      {initial : IndexesValue} {key tail : List w8} {reason : IndexesItemFailureReason}
      (valid : IntegerKeyValid key decoded)
      (failure : IndexesItemRejected d (indexesChoice seen decoded) tail consumed offset extraDepth reason) :
      IndexesPairsRejected d (n + 1) seen initial (key ++ tail) initial
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n consumed offset extraDepth : Nat} {seen : List Bool} {decoded : Option w64}
      {initial middle final : IndexesValue} {key value tail : List w8} {reason : IndexesRecordFailureReason}
      (keyValid : IntegerKeyValid key decoded)
      (valueValid : IndexesItemValid d (indexesChoice seen decoded) initial value middle)
      (failure : IndexesPairsRejected d n (indexesSeen seen (indexesChoice seen decoded)) middle tail final consumed offset extraDepth reason) :
      IndexesPairsRejected d (n + 1) seen initial (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

/-- All key widths, supported unknown keys, signed arrays and duplicates are
    classified independently of the generated loop's operational correctness. -/
theorem indexesPairs_classify (d n : Nat) (seen : List Bool) (initial : IndexesValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ IndexesPairsValid d n seen initial item final) ∨
    (∃ final consumed offset extraDepth reason,
      IndexesPairsRejected d n seen initial wire final consumed offset extraDepth reason) := by
  induction n generalizing seen initial wire with
  | zero => exact Or.inl ⟨[], wire, initial, rfl, .nil seen initial⟩
  | succ n ih =>
    rcases integerKey_classify wire with ⟨key, tail, decoded, split, valid⟩ | ⟨consumed, reason, failure⟩
    · subst wire
      rcases indexesItem_classify d (indexesChoice seen decoded) initial tail depth with
        ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
      · subst tail
        rcases ih (indexesSeen seen (indexesChoice seen decoded)) middle rest with
          ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
        · subst rest
          exact Or.inl ⟨key ++ value ++ items, suffix, final,
            by simp only [List.append_assoc], .step valid valueValid validRest⟩
        · apply Or.inr
          refine ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
          simpa only [List.append_assoc] using IndexesPairsRejected.next valid valueValid failure
      · exact Or.inr ⟨initial, key.length + consumed, key.length + offset, extraDepth, .value reason,
          .value valid failure⟩
    · exact Or.inr ⟨initial, consumed, 0, 0, .key reason, .key failure⟩

inductive IndexesRecordValid (d : Nat) (initial : IndexesValue) : List w8 → IndexesValue → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : IndexesRecordValid d initial [head] initial
  | map {head : w8} {payload body : List w8} {argument : w64} {count : Nat} {final : IndexesValue}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : IndexesPairsValid (d + 1) count [] initial body final) :
      IndexesRecordValid d initial ((head :: payload) ++ body) final

inductive IndexesRecordRejected (d : Nat) (initial : IndexesValue) : List w8 → IndexesValue →
    Nat → Nat → Nat → IndexesRecordFailureReason → Prop where
  | opening {wire : List w8} {reason : FeltFailureReason} (failure : StructOpenFailure d wire reason) :
      IndexesRecordRejected d initial wire initial 0 0 0 (.opening reason)
  | map {head : w8} {payload body : List w8} {argument : w64} {count consumed offset extraDepth : Nat}
      {final : IndexesValue} {reason : IndexesRecordFailureReason}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : IndexesPairsRejected (d + 1) count [] initial body final consumed offset extraDepth reason) :
      IndexesRecordRejected d initial ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) (1 + extraDepth) reason

/-- Every byte input has a specified record outcome, including partially
    published values and retained cursor/depth state before public cleanup. -/
theorem indexesRecord_classify (d : Nat) (initial : IndexesValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ IndexesRecordValid d initial item final) ∨
    (∃ final consumed offset extraDepth reason,
      IndexesRecordRejected d initial wire final consumed offset extraDepth reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, initial, split, .nullish nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases indexesPairs_classify (d + 1) count [] initial body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, 1 + extraDepth, reason,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨initial, 0, 0, 0, .opening reason, .opening failure⟩

theorem indexesPairs_count_bound (d n : Nat) (seen : List Bool) (initial final : IndexesValue)
    (wire : List w8) (valid : IndexesPairsValid d n seen initial wire final) : 2 * n ≤ wire.length := by
  induction valid with
  | nil => simp
  | @step n seen decoded initial middle final key value tail keyValid valueValid rest ih =>
    have hk : 0 < key.length := by
      cases keyValid with
      | unsigned _ _ _ => simp
      | unknown valid _ => exact keyValid_nonempty _ _ valid
    have hv := indexesItem_nonempty d _ initial middle value valueValid
    simp only [List.length_append]
    omega

theorem indexesPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List Bool)
    (initial final : IndexesValue) (wire : List w8) (reason : IndexesRecordFailureReason) (depth : d ≤ 32)
    (failure : IndexesPairsRejected d n seen initial wire final consumed offset extraDepth reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  induction failure with
  | key failure => have := integerKeyRejected_bounds _ _ _ failure; omega
  | value valid failure =>
    have := indexesItemRejected_bounds _ _ _ _ _ _ _ depth failure
    simp only [List.length_append]
    omega
  | next keyValid valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem indexesRecord_nonempty (d : Nat) (initial final : IndexesValue) (wire : List w8)
    (valid : IndexesRecordValid d initial wire final) : 0 < wire.length := by
  cases valid <;> simp

theorem indexesRecordRejected_bounds (d consumed offset extraDepth : Nat)
    (initial final : IndexesValue) (wire : List w8) (reason : IndexesRecordFailureReason) (depth : d ≤ 32)
    (failure : IndexesRecordRejected d initial wire final consumed offset extraDepth reason) :
    consumed ≤ wire.length ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | opening failure => simp; omega
  | map enc major length hd fit failure =>
    have := indexesPairsRejected_bounds _ _ _ _ _ _ _ _ _ _ (by omega) failure
    simp only [List.length_append, List.length_cons]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesItemRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecord_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecord_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesRecordRejected_bounds
