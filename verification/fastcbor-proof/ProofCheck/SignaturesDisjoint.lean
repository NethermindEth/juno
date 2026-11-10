import ProofCheck.FeltDisjoint
import ProofCheck.SignaturesDeterminism
import ProofCheck.SignaturesFailureSound

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem feltPointersFailure_not_nullish (d consumed offset : Nat) (entered : Bool)
    (wire : List w8) (reason : FeltFailureReason) (failure : FeltPointersFailure d wire consumed offset entered reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | outer hn hu failure => exact ⟨hn, hu⟩
  | count enc major short =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | depth enc major fit =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | child items enc major depth more fit failure =>
    simp only [List.append_assoc, List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction

/-- Complete feltPointers success/error separation on representable input.
    The bound rules out wrapping the legacy failure witness's natural count. -/
theorem feltPointersValid_not_failure (d consumed offset : Nat) (entered : Bool)
    (item rest : List w8) (value : Option (List (Option (GoArray w64 4)))) (reason : FeltFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : FeltPointersValid d item value)
    (failure : FeltPointersFailure d (item ++ rest) consumed offset entered reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := feltPointersFailure_not_nullish d consumed offset entered _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | @array head payload count items enc major size depth =>
    generalize eqWire : ((head :: payload) ++ items.flatMap FeltWire.bytes) ++ rest = input at failure
    cases failure with
    | outer hn hu failure =>
      exact expectedFailure_excludes_encoded (W8 128) head payload
        (items.flatMap FeltWire.bytes ++ rest) _ count _ enc major
        (by simpa only [List.append_assoc] using eqWire.symm) failure
    | @count d other payload2 tail argument2 enc2 major2 short =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap FeltWire.bytes ++ rest) tail count argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst argument2; subst tail
      have bytes := feltWire_count_bound (d + 1) items
      simp only [List.length_append] at short
      omega
    | depth enc2 major2 fit => omega
    | @child d count2 offset2 other payload2 tail done reason enc2 major2 depth2 more fit failure =>
      have fitCount : uint.Z (W64 count2) = count2 := by
        have wholeBound : (((other :: payload2) ++ done.flatMap FeltWire.bytes) ++ tail).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at wholeBound fit
        word
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap FeltWire.bytes ++ rest) (done.flatMap FeltWire.bytes ++ tail) count (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      have countEq : count2 = items.length := by rw [argumentEq, fitCount] at size; omega
      have moreItems : done.length < items.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix FeltWire.bytes done items tail rest moreItems
        (fun a _ b _ restA restB same =>
          (feltValid_prefix_unique (d + 1) a.value b.value a.bytes b.bytes restA restB a.valid b.valid same).2.2)
        tailEq.symm
      subst tail
      exact feltValid_not_failure (d + 1) offset2 next.bytes suffix next.value reason next.valid failure

theorem signaturesFailure_not_nullish (d consumed offset : Nat) (extraDepth : Nat)
    (wire : List w8) (reason : FeltFailureReason) (failure : SignaturesFailure d wire consumed offset extraDepth reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | outer hn hu failure => exact ⟨hn, hu⟩
  | count enc major short =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | depth enc major fit =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction
  | child items enc major depth more fit failure =>
    simp only [List.append_assoc, List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> contradiction

/-- Complete signatures success/error separation on representable input.
    The bound rules out wrapping the legacy failure witness's natural count. -/
theorem signaturesValid_not_failure (d consumed offset : Nat) (extraDepth : Nat)
    (item rest : List w8) (value : Option (List SignatureValue)) (reason : FeltFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : SignaturesValid d item value)
    (failure : SignaturesFailure d (item ++ rest) consumed offset extraDepth reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := signaturesFailure_not_nullish d consumed offset extraDepth _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | @array head payload count items enc major size depth =>
    generalize eqWire : ((head :: payload) ++ items.flatMap SignatureWire.bytes) ++ rest = input at failure
    cases failure with
    | outer hn hu failure =>
      exact expectedFailure_excludes_encoded (W8 128) head payload
        (items.flatMap SignatureWire.bytes ++ rest) _ count _ enc major
        (by simpa only [List.append_assoc] using eqWire.symm) failure
    | @count d other payload2 tail argument2 enc2 major2 short =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap SignatureWire.bytes ++ rest) tail count argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst argument2; subst tail
      have bytes := signatureWire_count_bound (d + 1) items
      simp only [List.length_append] at short
      omega
    | depth enc2 major2 fit => omega
    | @child d count2 consumed2 offset2 entered2 other payload2 tail done reason enc2 major2 depth2 more fit failure =>
      have fitCount : uint.Z (W64 count2) = count2 := by
        have wholeBound : (((other :: payload2) ++ done.flatMap SignatureWire.bytes) ++ tail).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at wholeBound fit
        word
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap SignatureWire.bytes ++ rest) (done.flatMap SignatureWire.bytes ++ tail) count (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      have countEq : count2 = items.length := by rw [argumentEq, fitCount] at size; omega
      have moreItems : done.length < items.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix SignatureWire.bytes done items tail rest moreItems
        (fun a _ b _ restA restB same =>
          (feltPointersValid_prefix_unique (d + 1) a.value b.value a.bytes b.bytes restA restB a.valid b.valid same).2.2)
        tailEq.symm
      have tailBound : tail.length < 2^64 := by
        have wholeBound : (((other :: payload2) ++ done.flatMap SignatureWire.bytes) ++ tail).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at wholeBound
        omega
      subst tail
      exact feltPointersValid_not_failure (d + 1) consumed2 offset2 entered2 next.bytes suffix next.value reason
        tailBound next.valid failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersFailure_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.feltPointersValid_not_failure
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesFailure_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.signaturesValid_not_failure
