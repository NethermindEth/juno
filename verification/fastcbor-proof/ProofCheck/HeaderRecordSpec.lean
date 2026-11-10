import ProofCheck.HeaderItemSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

def HeaderChoiceFresh (seen : List HeaderField) (choice : Option HeaderField) : Prop :=
  match choice with
  | none => True
  | some field => field ∉ seen

def headerChoiceSeen (seen : List HeaderField) (choice : Option HeaderField) : List HeaderField :=
  match choice with
  | none => seen
  | some field => field :: seen

/-- Successful interpretation of exactly n Header pairs in their wire order.
    Known identities are unique after case folding; unknown keys may repeat. -/
inductive HeaderPairsValid (d : Nat) : Nat → List HeaderField → HeaderValue → List w8 → HeaderValue → Prop where
  | nil (seen : List HeaderField) (initial : HeaderValue) : HeaderPairsValid d 0 seen initial [] initial
  | step {n : Nat} {seen : List HeaderField} {choice : Option HeaderField} {initial middle final : HeaderValue}
      {key value tail : List w8} (keyValid : HeaderKeyValid key choice) (fresh : HeaderChoiceFresh seen choice)
      (valueValid : HeaderItemValid d choice initial value middle)
      (rest : HeaderPairsValid d n (headerChoiceSeen seen choice) middle tail final) :
      HeaderPairsValid d (n + 1) seen initial (key ++ value ++ tail) final

inductive HeaderRecordFailureReason where
  | opening (reason : FeltFailureReason)
  | value (reason : HeaderValueFailureReason)
  | duplicate (field : HeaderField)
  | key (reason : KeyFailureReason)

/-- Header pair failures retain the last published Header value. Child failures
    additionally retain their own cursor/depth progress and any private partial
    state carried by HeaderItemRejected; failed replacements are unpublished. -/
inductive HeaderPairsRejected (d : Nat) : Nat → List HeaderField → HeaderValue → List w8 →
    HeaderValue → Nat → Nat → Nat → HeaderRecordFailureReason → Prop where
  | key {n : Nat} {seen : List HeaderField} {initial : HeaderValue} {wire : List w8} {reason : KeyFailureReason}
      (failure : KeyRejected wire reason) :
      HeaderPairsRejected d (n + 1) seen initial wire initial 0 0 0 (.key reason)
  | duplicate {n : Nat} {seen : List HeaderField} {initial : HeaderValue} {key tail : List w8} {field : HeaderField}
      (valid : HeaderKeyValid key (some field)) (duplicate : field ∈ seen) :
      HeaderPairsRejected d (n + 1) seen initial (key ++ tail) initial key.length key.length 0 (.duplicate field)
  | value {n consumed offset extraDepth : Nat} {seen : List HeaderField} {choice : Option HeaderField}
      {initial : HeaderValue} {key tail : List w8} {reason : HeaderValueFailureReason}
      (valid : HeaderKeyValid key choice) (fresh : HeaderChoiceFresh seen choice)
      (failure : HeaderItemRejected d choice tail consumed offset extraDepth reason) :
      HeaderPairsRejected d (n + 1) seen initial (key ++ tail) initial
        (key.length + consumed) (key.length + offset) extraDepth (.value reason)
  | next {n consumed offset extraDepth : Nat} {seen : List HeaderField} {choice : Option HeaderField}
      {initial middle final : HeaderValue} {key value tail : List w8} {reason : HeaderRecordFailureReason}
      (keyValid : HeaderKeyValid key choice) (fresh : HeaderChoiceFresh seen choice)
      (valueValid : HeaderItemValid d choice initial value middle)
      (failure : HeaderPairsRejected d n (headerChoiceSeen seen choice) middle tail final consumed offset extraDepth reason) :
      HeaderPairsRejected d (n + 1) seen initial (key ++ value ++ tail) final
        (key.length + value.length + consumed) (key.length + value.length + offset) extraDepth reason

/-- All Header pair sequences have a specified result, with unrestricted
    supported key/value encodings, arbitrary field order and exact suffixes. -/
theorem headerPairs_classify (d n : Nat) (seen : List HeaderField) (initial : HeaderValue)
    (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderPairsValid d n seen initial item final) ∨
    (∃ final consumed offset extraDepth reason,
      HeaderPairsRejected d n seen initial wire final consumed offset extraDepth reason) := by
  classical
  induction n generalizing seen initial wire with
  | zero => exact Or.inl ⟨[], wire, initial, rfl, .nil seen initial⟩
  | succ n ih =>
    rcases headerKey_classify wire with ⟨key, tail, choice, split, valid⟩ | ⟨reason, failure⟩
    · subst wire
      by_cases fresh : HeaderChoiceFresh seen choice
      · rcases headerItem_classify d choice initial tail depth with
          ⟨value, rest, middle, splitValue, valueValid⟩ | ⟨consumed, offset, extraDepth, reason, failure⟩
        · subst tail
          rcases ih (headerChoiceSeen seen choice) middle rest with
            ⟨items, suffix, final, splitRest, validRest⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
          · subst rest
            exact Or.inl ⟨key ++ value ++ items, suffix, final,
              by simp only [List.append_assoc], .step valid fresh valueValid validRest⟩
          · apply Or.inr
            refine ⟨final, key.length + value.length + consumed, key.length + value.length + offset, extraDepth, reason, ?_⟩
            simpa only [List.append_assoc] using HeaderPairsRejected.next valid fresh valueValid failure
        · exact Or.inr ⟨initial, key.length + consumed, key.length + offset, extraDepth, .value reason,
            .value valid fresh failure⟩
      · cases choice with
        | none => exact False.elim (fresh trivial)
        | some field => exact Or.inr ⟨initial, key.length, key.length, 0, .duplicate field,
            .duplicate valid (by simpa only [HeaderChoiceFresh, Classical.not_not] using fresh)⟩
    · exact Or.inr ⟨initial, 0, 0, 0, .key reason, .key failure⟩

/-- Complete Header record success. Null/undefined preserve the initial record;
    map decoding starts with no known fields seen and enforces the depth limit. -/
inductive HeaderRecordValid (d : Nat) (initial : HeaderValue) : List w8 → HeaderValue → Prop where
  | nullish {head : w8} (hn : head = W8 246 ∨ head = W8 247) : HeaderRecordValid d initial [head] initial
  | map {head : w8} {payload body : List w8} {argument : w64} {count : Nat} {final : HeaderValue}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32)
      (valid : HeaderPairsValid (d + 1) count [] initial body final) :
      HeaderRecordValid d initial ((head :: payload) ++ body) final

/-- Complete Header record failures preserve their partial public value until
    the public API clears it. Opening errors leave depth and cursor unchanged;
    map errors retain its depth increment plus any entered child containers. -/
inductive HeaderRecordRejected (d : Nat) (initial : HeaderValue) : List w8 → HeaderValue →
    Nat → Nat → Nat → HeaderRecordFailureReason → Prop where
  | opening {wire : List w8} {reason : FeltFailureReason} (failure : StructOpenFailure d wire reason) :
      HeaderRecordRejected d initial wire initial 0 0 0 (.opening reason)
  | map {head : w8} {payload body : List w8} {argument : w64} {count consumed offset extraDepth : Nat}
      {final : HeaderValue} {reason : HeaderRecordFailureReason}
      (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 160)
      (length : uint.Z argument = count) (depth : d < 32) (fit : 2 * count ≤ body.length)
      (failure : HeaderPairsRejected (d + 1) count [] initial body final consumed offset extraDepth reason) :
      HeaderRecordRejected d initial ((head :: payload) ++ body) final
        (1 + payload.length + consumed) (1 + payload.length + offset) (1 + extraDepth) reason

/-- Every byte input has a complete Header application-specification outcome,
    covering every field and nested value. This does not assume the production
    decoder or any missing helper WP is correct. -/
theorem headerRecord_classify (d : Nat) (initial : HeaderValue) (wire : List w8) (depth : d ≤ 32) :
    (∃ item rest final, wire = item ++ rest ∧ HeaderRecordValid d initial item final) ∨
    (∃ final consumed offset extraDepth reason,
      HeaderRecordRejected d initial wire final consumed offset extraDepth reason) := by
  rcases struct_open_classify d wire depth with
    ⟨head, tail, split, nullish⟩ | ⟨head, payload, body, argument, split, enc, major, hd, fit⟩ | ⟨reason, failure⟩
  · exact Or.inl ⟨[head], tail, initial, split, .nullish nullish⟩
  · subst wire
    let count := (uint.Z argument).toNat
    have length : uint.Z argument = (count : Int) := by dsimp [count]
    have countFit : 2 * count ≤ body.length := by omega
    rcases headerPairs_classify (d + 1) count [] initial body (by omega) with
      ⟨items, rest, final, splitBody, valid⟩ | ⟨final, consumed, offset, extraDepth, reason, failure⟩
    · exact Or.inl ⟨(head :: payload) ++ items, rest, final,
        by rw [List.append_assoc, ← splitBody], .map enc major length hd valid⟩
    · exact Or.inr ⟨final, 1 + payload.length + consumed, 1 + payload.length + offset, 1 + extraDepth, reason,
        .map enc major length hd countFit failure⟩
  · exact Or.inr ⟨initial, 0, 0, 0, .opening reason, .opening failure⟩

theorem headerPairs_count_bound (d n : Nat) (seen : List HeaderField) (initial final : HeaderValue)
    (wire : List w8) (valid : HeaderPairsValid d n seen initial wire final) : 2 * n ≤ wire.length := by
  induction valid with
  | nil => simp
  | @step n seen choice initial middle final key value tail keyValid fresh valueValid rest ih =>
    have hk := schemaKey_nonempty HeaderKeyTextMeaning key choice keyValid
    have hv := headerItem_nonempty d choice initial middle value valueValid
    simp only [List.length_append]
    omega

theorem headerPairsRejected_bounds (d n consumed offset extraDepth : Nat) (seen : List HeaderField)
    (initial final : HeaderValue) (wire : List w8) (reason : HeaderRecordFailureReason) (depth : d ≤ 32)
    (failure : HeaderPairsRejected d n seen initial wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  induction failure with
  | key => simp; omega
  | duplicate => simp; omega
  | value valid fresh failure =>
    have bound := headerItemRejected_bounds _ _ _ _ _ _ _ depth failure
    simp only [List.length_append]
    omega
  | next keyValid fresh valueValid failure ih =>
    simp only [List.length_append]
    omega

theorem headerRecord_nonempty (d : Nat) (initial final : HeaderValue) (wire : List w8)
    (valid : HeaderRecordValid d initial wire final) : 0 < wire.length := by
  cases valid <;> simp

theorem headerRecordRejected_bounds (d consumed offset extraDepth : Nat)
    (initial final : HeaderValue) (wire : List w8) (reason : HeaderRecordFailureReason) (depth : d ≤ 32)
    (failure : HeaderRecordRejected d initial wire final consumed offset extraDepth reason) :
    consumed ≤ offset ∧ offset ≤ wire.length ∧ d + extraDepth ≤ 32 := by
  cases failure with
  | opening failure => simp; omega
  | map enc major length hd fit failure =>
    have bound := headerPairsRejected_bounds _ _ _ _ _ _ _ _ _ _ (by omega) failure
    simp only [List.length_append, List.length_cons]
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairs_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecord_classify
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairs_count_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.headerPairsRejected_bounds
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecord_nonempty
#print axioms Perennial.github_com.NethermindEth.juno.core.headerRecordRejected_bounds
