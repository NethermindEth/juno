import ProofCheck.HeaderProjectionValues
import ProofCheck.WantedSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def headerProjectionNextSeen (seen : List Nat) (choice : Option Nat) : List Nat :=
  match wantedSelect seen choice with
  | none => seen
  | some index => index :: seen

inductive HeaderProjectionItemValid (d : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) (old : HeaderProjectionValue) : List w8 → HeaderProjectionValue → Prop where
  | ignored {wire} (ignored : wantedSelect seen choice = none) (valid : SkipValid d wire) :
      HeaderProjectionItemValid d kind seen choice old wire old
  | selected {index field wire final} (fresh : wantedSelect seen choice = some index)
      (lookup : (headerProjectionFields kind)[index]? = some field)
      (valid : HeaderProjectionFieldValid d field old wire final) :
      HeaderProjectionItemValid d kind seen choice old wire final

inductive HeaderProjectionFailure where
  | key (reason : KeyFailureReason)
  | ignored (reason : SkipFailureReason)
  | field (reason : HeaderProjectionFieldFailure)

inductive HeaderProjectionItemRejected (d : Nat) (kind : HeaderProjectionKind)
    (seen : List Nat) (choice : Option Nat) : List w8 → Nat → HeaderProjectionFailure → Prop where
  | ignored {wire offset reason} (ignored : wantedSelect seen choice = none) (failure : SkipFailure d wire offset reason) :
      HeaderProjectionItemRejected d kind seen choice wire offset (.ignored reason)
  | selected {index field wire offset reason} (fresh : wantedSelect seen choice = some index)
      (lookup : (headerProjectionFields kind)[index]? = some field)
      (failure : HeaderProjectionFieldRejected d field wire offset reason) :
      HeaderProjectionItemRejected d kind seen choice wire offset (.field reason)

theorem headerProjectionItem_classify (d : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (choice : Option Nat) (old : HeaderProjectionValue) (key wire : List w8)
    (keyValid : WantedKeyValid (headerProjectionNames kind) key choice) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderProjectionItemValid d kind seen choice old item final) ∨
    (∃ offset reason, HeaderProjectionItemRejected d kind seen choice wire offset reason) := by
  cases selected : wantedSelect seen choice with
  | none =>
    rcases skip_classify d wire depth with ⟨item, rest, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, old, split, .ignored selected valid⟩
    · exact Or.inr ⟨offset, _, .ignored selected failure⟩
  | some index =>
    have bound := (wantedValid_selected d 1 index 0 (headerProjectionNames kind) seen key (.selected keyValid selected)).1
    obtain ⟨field, lookup⟩ := headerProjectionField_exists kind index bound
    rcases headerProjectionField_classify d field old wire depth with
      ⟨item, rest, final, split, valid⟩ | ⟨offset, reason, failure⟩
    · exact Or.inl ⟨item, rest, final, split, .selected selected lookup valid⟩
    · exact Or.inr ⟨offset, _, .selected selected lookup failure⟩

theorem headerProjectionItem_nonempty (d : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (choice : Option Nat) (old final : HeaderProjectionValue) (wire : List w8)
    (valid : HeaderProjectionItemValid d kind seen choice old wire final) : 0 < wire.length := by
  cases valid with
  | ignored ignored valid => exact skipValid_nonempty d wire valid
  | selected fresh lookup valid => exact headerProjectionField_nonempty _ _ _ _ _ valid

/-- Independent wire-order interpretation: consume exactly n pairs, skip unknown
    and already-seen fields, and publish only successful first occurrences. -/
inductive HeaderProjectionPairsValid (d : Nat) (kind : HeaderProjectionKind) :
    Nat → List Nat → HeaderProjectionValue → List w8 → HeaderProjectionValue → Prop where
  | nil (seen : List Nat) (old : HeaderProjectionValue) : HeaderProjectionPairsValid d kind 0 seen old [] old
  | step {n seen choice old middle final key value tail}
      (keyValid : WantedKeyValid (headerProjectionNames kind) key choice)
      (valueValid : HeaderProjectionItemValid d kind seen choice old value middle)
      (rest : HeaderProjectionPairsValid d kind n (headerProjectionNextSeen seen choice) middle tail final) :
      HeaderProjectionPairsValid d kind (n + 1) seen old (key ++ value ++ tail) final

/-- Rejection retains earlier published fields. Both Skip and the selected field
    helpers roll back to the value start; their diagnostic may point deeper. -/
inductive HeaderProjectionPairsRejected (d : Nat) (kind : HeaderProjectionKind) :
    Nat → List Nat → HeaderProjectionValue → List w8 → HeaderProjectionValue → Nat → Nat → HeaderProjectionFailure → Prop where
  | key {n seen old wire reason} (failure : KeyRejected wire reason) :
      HeaderProjectionPairsRejected d kind (n + 1) seen old wire old 0 0 (.key reason)
  | value {n seen choice old key tail offset reason}
      (keyValid : WantedKeyValid (headerProjectionNames kind) key choice)
      (failure : HeaderProjectionItemRejected d kind seen choice tail offset reason) :
      HeaderProjectionPairsRejected d kind (n + 1) seen old (key ++ tail) old key.length (key.length + offset) reason
  | next {n seen choice old middle final key value tail consumed offset reason}
      (keyValid : WantedKeyValid (headerProjectionNames kind) key choice)
      (valueValid : HeaderProjectionItemValid d kind seen choice old value middle)
      (failure : HeaderProjectionPairsRejected d kind n (headerProjectionNextSeen seen choice) middle tail final consumed offset reason) :
      HeaderProjectionPairsRejected d kind (n + 1) seen old (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) reason

theorem headerProjectionPairs_classify (d n : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (old : HeaderProjectionValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderProjectionPairsValid d kind n seen old item final) ∨
    (∃ final consumed offset reason, HeaderProjectionPairsRejected d kind n seen old wire final consumed offset reason) := by
  induction n generalizing seen old wire with
  | zero => exact Or.inl ⟨[], wire, old, rfl, .nil seen old⟩
  | succ n ih =>
    rcases schemaKey_classify (WantedKeyMeaning (headerProjectionNames kind))
      (wantedKeyMeaning_total (headerProjectionNames kind)) wire with ⟨key, tail, choice, split, keyValid⟩ | ⟨reason, failure⟩
    · subst wire
      rcases headerProjectionItem_classify d kind seen choice old key tail keyValid depth with
        ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨offset, reason, failure⟩
      · subst tail
        rcases ih (headerProjectionNextSeen seen choice) middle rest with
          ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, reason, failure⟩
        · subst rest
          exact Or.inl ⟨key ++ value ++ items, suffix, final,
            by simp only [List.append_assoc], .step keyValid valueValid validRest⟩
        · refine Or.inr ⟨final, key.length + value.length + consumed, key.length + value.length + offset, reason, ?_⟩
          simpa only [List.append_assoc] using HeaderProjectionPairsRejected.next keyValid valueValid failure
      · exact Or.inr ⟨old, key.length, key.length + offset, reason, .value keyValid failure⟩
    · exact Or.inr ⟨old, 0, 0, .key reason, .key failure⟩

theorem headerProjectionPairs_count_bound (d n : Nat) (kind : HeaderProjectionKind) (seen : List Nat)
    (old final : HeaderProjectionValue) (wire : List w8)
    (valid : HeaderProjectionPairsValid d kind n seen old wire final) : 2 * n ≤ wire.length := by
  induction valid with
  | nil => simp
  | step keyValid valueValid rest ih =>
    have hk := schemaKey_nonempty (WantedKeyMeaning (headerProjectionNames kind)) _ _ keyValid
    have hv := headerProjectionItem_nonempty _ _ _ _ _ _ _ valueValid
    simp only [List.length_append]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItem_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionItem_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerProjectionPairs_count_bound
