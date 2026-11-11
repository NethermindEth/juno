import ProofCheck.IntsDeterminism
import ProofCheck.IntElementDeterminism
import ProofCheck.IntsFailureSpec

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

theorem intsRejected_not_nullish (d consumed offset : Nat) (entered : Bool)
    (wire : List w8) (reason : IntsFailureReason) (failure : IntsRejected d wire consumed offset entered reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | outer hn hu failure => exact ⟨hn, hu⟩
  | count enc major short =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> word
  | depth enc major fit =>
    simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> word
  | element items enc major depth more fit failure =>
    simp only [List.append_assoc, List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
    constructor <;> intro h <;> subst h <;> word

/-- Complete signed-array success/error separation on representable input.
    The bound rules out wrapping the failure witness's natural count. -/
theorem intsValid_not_rejected (d consumed offset : Nat) (entered : Bool)
    (item rest : List w8) (value : Option (List Int)) (reason : IntsFailureReason)
    (bound : (item ++ rest).length < 2^64) (valid : IntsValid d item value)
    (failure : IntsRejected d (item ++ rest) consumed offset entered reason) : False := by
  cases valid with
  | nullish hn =>
    have excluded := intsRejected_not_nullish d consumed offset entered _ reason failure
    rcases hn with rfl | rfl
    · exact excluded.1 rfl
    · exact excluded.2 rfl
  | @array head payload count items enc major size depth =>
    generalize eqWire : ((head :: payload) ++ items.flatMap IntElementWire.bytes) ++ rest = input at failure
    cases failure with
    | outer hn hu failure =>
      exact expectedFailure_excludes_encoded (W8 128) head payload
        (items.flatMap IntElementWire.bytes ++ rest) _ count _ enc major
        (by simpa only [List.append_assoc] using eqWire.symm) failure
    | @count d other payload2 tail argument2 enc2 major2 short =>
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap IntElementWire.bytes ++ rest) tail count argument2 enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      subst argument2; subst tail
      have bytes := intElementWire_count_bound items
      simp only [List.length_append] at short
      omega
    | depth enc2 major2 fit => omega
    | @element d count2 consumed2 other payload2 tail done reason enc2 major2 depth2 more fit failure =>
      have fitCount : uint.Z (W64 count2) = count2 := by
        have wholeBound : (((other :: payload2) ++ done.flatMap IntElementWire.bytes) ++ tail).length < 2^64 := by
          rw [← eqWire]; exact bound
        simp only [List.length_append, List.length_cons] at wholeBound fit
        word
      obtain ⟨headEq, payloadEq, argumentEq, tailEq⟩ := encodedHead_prefix_unique head other payload payload2
        (items.flatMap IntElementWire.bytes ++ rest) (done.flatMap IntElementWire.bytes ++ tail) count (W64 count2) enc enc2
        (by simpa only [List.append_assoc] using eqWire)
      have countEq : count2 = items.length := by rw [argumentEq, fitCount] at size; omega
      have moreItems : done.length < items.length := by omega
      obtain ⟨next, suffix, member, split⟩ := wireSequence_after_prefix IntElementWire.bytes done items tail rest moreItems
        (fun a _ b _ restA restB same =>
          (intElementWire_prefix_unique a b restA restB same).2.2)
        tailEq.symm
      subst tail
      exact intElementWire_not_rejected next suffix consumed2 reason failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.intsRejected_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.intsValid_not_rejected
