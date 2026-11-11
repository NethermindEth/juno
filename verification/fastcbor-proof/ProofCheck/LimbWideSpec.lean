import ProofCheck.LimbWideGuard
import ProofCheck.FeltDisjoint

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A marker at a known offset splits the prefix without imposing a canonical
    representation on the arbitrary bytes before it. -/
theorem limb_marker_split (wire : List w8) (n : Nat) (marker : w8)
    (lookup : wire[n]? = some marker) :
    ∃ front suffix, front.length = n ∧ wire = front ++ marker :: suffix := by
  obtain ⟨bound, value⟩ := List.getElem?_eq_some_iff.mp lookup
  refine ⟨wire.take n, wire.drop (n + 1), by simp; omega, ?_⟩
  have split := List.take_append_drop n wire
  rw [List.drop_eq_getElem_cons bound, value] at split
  exact split.symm

/-- Passing all fixed offsets identifies a complete 37-byte felt, even on an
    otherwise arbitrary input. No encoder validity assumption is needed. -/
theorem limbWideGuard_decompose (wire : List w8) (guard : LimbWideGuard wire) :
    ∃ a b c d rest, a.length = 8 ∧ b.length = 8 ∧ c.length = 8 ∧ d.length = 8 ∧
      wire = wideFeltBytes a b c d ++ rest := by
  obtain ⟨size, h0, h1, h10, h19, h28⟩ := guard
  cases wire with
  | nil => simp at h0
  | cons head tail =>
    have hhead : head = W8 132 := Option.some.inj h0
    subst head
    cases tail with
    | nil => simp at h1
    | cons tag tail =>
      have htag : tag = W8 27 := Option.some.inj h1
      subst tag
      have haTag : tail[8]? = some (W8 27) := h10
      obtain ⟨a, tail, ha, rfl⟩ := limb_marker_split tail 8 (W8 27) haTag
      have hbTag : tail[8]? = some (W8 27) := by
        simpa [List.getElem?_append, ha] using h19
      obtain ⟨b, tail, hb, rfl⟩ := limb_marker_split tail 8 (W8 27) hbTag
      have hcTag : tail[8]? = some (W8 27) := by
        simpa [List.getElem?_append, ha, hb] using h28
      obtain ⟨c, tail, hc, rfl⟩ := limb_marker_split tail 8 (W8 27) hcTag
      have htail : 8 ≤ tail.length := by simp only [List.length_cons, List.length_append, ha, hb, hc] at size; omega
      refine ⟨a, b, c, tail.take 8, tail.drop 8, ha, hb, hc, by simp; omega, ?_⟩
      simp only [wideFeltBytes, List.append_assoc, List.cons_append, List.nil_append, List.take_append_drop]

/-- The wide decoder's four big-endian words satisfy the independent felt
    relation at any depth where a felt array is permitted. -/
theorem limbWide_valid (a b c d : List w8) (depth : Nat)
    (ha : a.length = 8) (hb : b.length = 8) (hc : c.length = 8) (hd : d.length = 8)
    (bound : depth < 32) :
    FeltValid depth (wideFeltBytes a b c d) (some (wideFeltValue a b c d)) := by
  let word (bs : List w8) (len : bs.length = 8) : UnsignedWire :=
    ⟨W8 27, bs, leToU64 bs.reverse, by decide, .u64 (W8 27) bs len (by decide)⟩
  have valid := FeltValid.small [word a ha, word b hb, word c hc, word d hd]
    (HeadArgument.inline (W8 132) (by decide)) (by decide) (by rfl) (by simp) bound
  simpa [word, wideFeltBytes, wideFeltValue, unsignedBytes, UnsignedWire.bytes, paddedLimbs, List.append_assoc] using valid

/-- In-place failure and successful nonnullable felt parsing cannot overlap. -/
theorem limbsFailure_not_valid (depth offset : Nat) (item rest : List w8)
    (words : GoArray w64 4) (reason : FeltFailureReason)
    (valid : FeltValid depth item (some words))
    (failure : LimbsFailure depth (item ++ rest) offset reason) : False := by
  cases failure with
  | typed failure => exact feltValid_not_failure depth offset item rest (some words) reason valid failure
  | outer failure =>
    obtain ⟨head, payload, argument, items, extras, rfl, enc, major, count, size, children, rfl⟩ :=
      feltValid_some_parts depth item words valid
    exact expectedFailure_excludes_encoded (W8 128) head payload
      (unsignedBytes items ++ extras.flatten ++ rest) _ argument _ enc major
      (by simp only [List.append_assoc]) failure

/-- Every failed element declines the fixed-width shortcut at a permitted felt
    depth. This justifies falling back to the fully checked Limbs decoder. -/
theorem limbWideGuard_not_failure (depth offset : Nat) (wire : List w8)
    (reason : FeltFailureReason) (bound : depth < 32)
    (failure : LimbsFailure depth wire offset reason) : ¬ LimbWideGuard wire := by
  intro guard
  obtain ⟨a, b, c, d, rest, ha, hb, hc, hd, rfl⟩ := limbWideGuard_decompose wire guard
  exact limbsFailure_not_valid depth offset _ rest _ reason
    (limbWide_valid a b c d depth ha hb hc hd bound) failure

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.limb_marker_split
#print axioms Perennial.github_com.NethermindEth.juno.core.limbWideGuard_decompose
#print axioms Perennial.github_com.NethermindEth.juno.core.limbWide_valid

#print axioms Perennial.github_com.NethermindEth.juno.core.limbsFailure_not_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.limbWideGuard_not_failure
