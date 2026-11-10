import ProofCheck.BigIntMagnitude

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Fresh SetUint64 has the same canonical integer-ownership contract as the
    byte-magnitude helper, including zero's nil magnitude. -/
theorem wp_bigSetUint64_owned (p : Loc) (word : w64) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ p ↦ (math.big.Int'.mk false slice.nil) }}
      (App (Val (p @!! go.PointerType math.big.Int'.ty @!! go!"SetUint64")) (Val #word))
    {{ RET #p; BigMagnitudeOwned p (uint.Z word) }} := by
  iintro %Φ ⟨#Hpkg, Hp⟩ HΦ
  by_cases hz : word = W64 0
  · subst word
    wp_apply wp_bigSetUint64_fresh_zero p $$ [Hp] with Hp
    · iframe Hp
    iapply HΦ
    unfold BigMagnitudeOwned
    iexists slice.nil, [], false
    iframe Hp
    isplitr
    · iapply ownSlice_nil
    ipureintro
    refine ⟨?_, rfl, fun _ => rfl⟩
    unfold normalizedPrefix
    exact ⟨by decide, by decide, fun j _ h => by simp at h, Or.inl rfl⟩
  · wp_apply wp_bigSetUint64_fresh_nonzero p word hz $$ [Hp] as %s ⟨Hp, Hs⟩
    · iframe Hp
    iapply HΦ
    unfold BigMagnitudeOwned
    iexists s, [word], false
    iframe Hp Hs
    ipureintro
    refine ⟨?_, ?_, fun _ => rfl⟩
    · unfold normalizedPrefix
      refine ⟨by simp, by omega, ?_, Or.inr ⟨word, rfl, hz⟩⟩
      intro j hj hj'; simp only [List.length_singleton] at hj hj'; omega
    · simp only [Bool.false_eq_true, ite_false, magnitudeValue, Int.mul_zero, Int.add_zero]

/-- In-place Neg flips a canonical owned integer's mathematical value, including
    canonical zero. It preserves the complete magnitude allocation. -/
theorem wp_bigNeg_owned (p : Loc) (value : Int) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ BigMagnitudeOwned p value }}
      (App (Val (p @!! go.PointerType math.big.Int'.ty @!! go!"Neg")) (Val #p))
    {{ RET #p; BigMagnitudeOwned p (-value) }} := by
  iintro %Φ ⟨#Hpkg, Howned⟩ HΦ
  unfold BigMagnitudeOwned at *
  icases Howned with ⟨%s, %words, %negative, Hp, Hs, %Hnorm, %Hvalue, %Hzero⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hlen : sint.Z s.len = words.length := by
    simp only [sint.nat, sint.Z] at Hlen ⊢; omega
  wp_apply wp_bigNeg_self p (math.big.Int'.mk negative s) $$ [Hp] with Hp
  · iframe Hp
  iapply HΦ
  iexists s, words, (decide (0 < sint.Z s.len) && !negative)
  iframe Hp Hs
  ipureintro
  refine ⟨Hnorm, ?_, ?_⟩
  · cases words with
    | nil =>
      have hn : negative = false := Hzero rfl
      subst negative
      simp only [magnitudeValue, Bool.false_eq_true, ite_false] at Hvalue
      simp [Hvalue, magnitudeValue]
    | cons w ws =>
      have hp : 0 < sint.Z s.len := by simp only [List.length_cons] at hlen; omega
      rw [Hvalue]
      cases negative <;> simp [hp]
  · intro hempty
    rw [hempty] at hlen
    simp only [List.length_nil] at hlen
    simp [hlen]

/-- Borrow an arbitrary magnitude window, decode it, and restore the entire input. -/
theorem wp_bigMagnitude_window (data : GoSlice) (xs bytes : List w8) (low high : w64)
    (dq : DFrac) (negative increment : Bool)
    (hb : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat low) (sint.nat high) xs = bytes) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ data ↦*{dq} xs }}
      (App (App (App (Val (@! cursor.bigIntMagnitude)) (Val #(slice.slice data w8 low high)))
        (Val #negative)) (Val #increment))
    {{ (p : Loc), RET #p; data ↦*{dq} xs ∗
       BigMagnitudeOwned p ((if negative then -1 else 1) *
         (bigEndianMagnitude bytes + if increment then 1 else 0)) }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice low high data dq xs hb).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply wp_bigMagnitude (slice.slice data w8 low high) bytes dq negative increment
    $$ [Hbytes] as %p ⟨Hbytes, Hout⟩
  · iframe Hbytes
  iapply HΦ
  iframe Hout
  iapply (ownSlice_slice low high data dq xs hb).2
  rw [hbytes]
  iframe Hpre Hbytes Hpost

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSetUint64_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigNeg_owned
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigMagnitude_window
