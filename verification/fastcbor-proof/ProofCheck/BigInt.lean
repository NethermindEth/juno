import ProofCheck.Fields

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The real math/big Set implementation does nothing when receiver equals argument. -/
theorem wp_bigSet_self (z : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big }}
      (App (Val (z @!! go.PointerType math.big.Int'.ty @!! go!"Set")) (Val #z))
    {{ RET #z; True }} := by
  wp_start
  wp_auto
  wp_end

/-- In-place Neg changes only the sign and does not copy the magnitude. -/
theorem wp_bigNeg_self (z : Loc) (v : math.big.Int') :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ z ↦ v }}
      (App (Val (z @!! go.PointerType math.big.Int'.ty @!! go!"Neg")) (Val #z))
    {{ RET #z; z ↦ {v with neg' := decide (0 < sint.Z v.abs'.len) && !v.neg'} }} := by
  wp_start as Hz
  wp_auto
  wp_apply wp_bigSet_self z
  by_cases h : 0 < sint.Z v.abs'.len
  · simp only [show sint.Z (W64 0) = 0 from rfl, h, _root_.decide_true]
    wp_auto
    wp_end
  · simp only [show sint.Z (W64 0) = 0 from rfl, h, _root_.decide_false]
    wp_auto
    wp_end

/-- All discarded high words are zero and a nonempty retained prefix ends nonzero. -/
def normalizedPrefix (xs : List w64) (n : Int) : Prop :=
  0 ≤ n ∧ n ≤ xs.length ∧
  (∀ j : Nat, n ≤ j → j < xs.length → xs[j]? = some (W64 0)) ∧
  (n = 0 ∨ ∃ w, xs[(n - 1).toNat]? = some w ∧ w ≠ W64 0)

/-- General normalization scans only the input slice and removes precisely zero high words. -/
theorem wp_bigNorm (s : GoSlice) (xs : List w64) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ s ↦*{dq} xs }}
      (App (Val (s @!! math.big.nat.ty @!! go!"norm")) (Val #()))
    {{ (n : w64), RET #(slice.slice s w64 (W64 0) n);
       s ↦*{dq} xs ∗ ⌜normalizedPrefix xs (sint.Z n)⌝ }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hlen : (xs.length : Int) = sint.Z s.len := by
    simp only [sint.nat, sint.Z] at Hlen Hwf ⊢
    omega
  wp_auto
  ihave HI : (∃ (i : w64), "i" ∷ i_ptr ↦ i ∗
      "%Hbounds" ∷ ⌜0 ≤ sint.Z i ∧ sint.Z i ≤ sint.Z s.len⌝ ∗
      "%Hzero" ∷ ⌜∀ j : Nat, sint.Z i ≤ j → j < xs.length → xs[j]? = some (W64 0)⌝ : IProp GF) $$ [i]
  · iexists s.len; iframe; ipureintro
    exact ⟨⟨Hwf.1, Int.le_refl _⟩, fun j hj hj' => by omega⟩
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  by_cases hpos : 0 < sint.Z i
  · simp only [show sint.Z (W64 0) = 0 from rfl, hpos, _root_.decide_true]
    wp_auto
    have hdec : sint.Z (i - W64 1) = sint.Z i - 1 := by
      simp only [sint.Z] at *
      word
    have hindex : 0 ≤ sint.Z (i - W64 1) ∧ sint.Z (i - W64 1) < sint.Z s.len := by
      rw [hdec]; omega
    list_elem xs (sint.nat (i - W64 1)) as w
    rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
    wp_apply wp_load_slice_index s (sint.Z (i - W64 1)) xs dq w hindex.1 $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact Hw_lookup
    by_cases hw : w = W64 0
    · simp only [hw, _root_.decide_true]
      simp only [ite_true]
      wp_auto
      wp_for_post
      iframe
      iexists (i - W64 1); iframe; ipureintro
      constructor
      · rw [hdec]; omega
      · intro j hj hj'
        by_cases hjold : sint.Z i ≤ j
        · exact Hzero j hjold hj'
        · have hjidx : j = sint.nat (i - W64 1) := by
            simp only [sint.nat, hdec] at *
            omega
          rw [hjidx, Hw_lookup, hw]
    · simp only [hw, _root_.decide_false]
      simp only [hf, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false]
      wp_auto
      rw [ite_eq_left_of_eq_true _ _ (eq_true (by
        simp only [show sint.Z (W64 0) = 0 from rfl]
        exact ⟨by omega, Hbounds.1, by omega⟩))]
      wp_auto
      iapply HΦ
      iframe Hs
      ipureintro
      refine ⟨Hbounds.1, by omega, Hzero, Or.inr ⟨w, ?_, hw⟩⟩
      simpa only [sint.nat, hdec] using Hw_lookup
  · simp only [show sint.Z (W64 0) = 0 from rfl, hpos, _root_.decide_false]
    wp_auto
    simp only [hf, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true (by
      simp only [show sint.Z (W64 0) = 0 from rfl]
      exact ⟨by omega, Hbounds.1, by omega⟩))]
    wp_auto
    iapply HΦ
    iframe Hs
    ipureintro
    exact ⟨Hbounds.1, by omega, Hzero, Or.inl (by omega)⟩

/-- Mathematical value of math/big's little-endian words on the pinned 64-bit target. -/
def magnitudeValue : List w64 → Int
  | [] => 0
  | w :: ws => uint.Z w + 2^64 * magnitudeValue ws

/-- Discarding only zero high words preserves the represented natural number. -/
theorem magnitudeValue_take (xs : List w64) (n : Nat)
    (hz : ∀ j : Nat, n ≤ j → j < xs.length → xs[j]? = some (W64 0)) :
    magnitudeValue (xs.take n) = magnitudeValue xs := by
  induction xs generalizing n with
  | nil => simp only [List.take_nil]
  | cons w ws ih =>
    cases n with
    | zero =>
      have hw : w = W64 0 := Option.some.inj (hz 0 (by omega) (by simp))
      have ht : magnitudeValue (ws.take 0) = magnitudeValue ws :=
        ih 0 (fun j _ hj => by simpa using hz (j+1) (by omega) (by simp; omega))
      simp only [List.take_zero, magnitudeValue] at ht ⊢
      rw [hw, ← ht]
      rfl
    | succ n =>
      simp only [List.take_succ_cons, magnitudeValue]
      rw [ih n (fun j hj hj' => by simpa using hz (j+1) (by omega) (by simp; omega))]

/-- General SetBits installs normalized words without changing their numeric value. -/
theorem wp_bigSetBits (z : Loc) (v : math.big.Int') (s : GoSlice)
    (xs : List w64) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ z ↦ v ∗ s ↦*{dq} xs }}
      (App (Val (z @!! go.PointerType math.big.Int'.ty @!! go!"SetBits")) (Val #s))
    {{ (n : w64), RET #z;
       z ↦ (math.big.Int'.mk false (slice.slice s w64 (W64 0) n)) ∗ s ↦*{dq} xs ∗
       ⌜normalizedPrefix xs (sint.Z n) ∧
         magnitudeValue (xs.take (sint.nat n)) = magnitudeValue xs⌝ }} := by
  wp_start as ⟨Hz, Hs⟩
  wp_auto
  wp_apply wp_bigNorm s xs dq $$ [Hs] as %n ⟨Hs, %Hnorm⟩
  · iframe Hs
  iapply HΦ
  iframe Hz Hs
  ipureintro
  refine ⟨Hnorm, magnitudeValue_take xs (sint.nat n) ?_⟩
  intro j hj hj'
  exact Hnorm.2.2.1 j (by simp only [sint.nat, sint.Z] at hj ⊢; omega) hj'

/-- The fresh one-word allocation used by SetUint64 has owned, zeroed storage. -/
theorem wp_bigMake_nil_one :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big }}
      (App (Val (slice.nil @!! math.big.nat.ty @!! go!"make")) (Val #(W64 1)))
    {{ (p : GoSlice), RET #p; p ↦* [W64 0] }} := by
  wp_start
  wp_auto
  wp_apply wp_slice_make2 (V := w64) (W64 1) as %p ⟨Hs, -⟩
  · ipureintro; decide
  wp_end

/-- SetWord on a fresh magnitude stores a nonzero word without borrowing storage. -/
theorem wp_bigSetWord_nil_nonzero (w : w64) (hw : w ≠ W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big }}
      (App (Val (slice.nil @!! math.big.nat.ty @!! go!"setWord")) (Val #w))
    {{ (p : GoSlice), RET #p; p ↦* [w] }} := by
  wp_start
  wp_auto
  simp only [hw, _root_.decide_false]
  wp_auto
  wp_method_call
  wp_auto
  wp_apply wp_bigMake_nil_one as %p Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hp : 0 < sint.Z p.len := by
    simp only [sint.nat, sint.Z, List.length_cons, List.length_nil] at Hlen ⊢
    omega
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, hp⟩)]
  wp_auto
  wp_apply wp_store_slice_index (t := math.big.Word.ty) p (sint.Z (W64 0)) [W64 0] w $$ [Hs] with Hs
  · iframe Hs; ipureintro; decide
  wp_end

/-- SetUint64 on a fresh Int stores every nonzero uint64 in a separately owned word. -/
theorem wp_bigSetUint64_fresh_nonzero (z : Loc) (w : w64) (hw : w ≠ W64 0) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ z ↦ (math.big.Int'.mk false slice.nil) }}
      (App (Val (z @!! go.PointerType math.big.Int'.ty @!! go!"SetUint64")) (Val #w))
    {{ (s : GoSlice), RET #z; z ↦ (math.big.Int'.mk false s) ∗ s ↦* [w] }} := by
  wp_start as Hz
  wp_auto
  wp_method_call
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_method_call
  wp_auto
  wp_apply wp_bigSetWord_nil_nonzero w hw as %s Hs
  wp_end

/-- Zero keeps the fresh magnitude nil; it does not allocate an empty backing array. -/
theorem wp_bigSetUint64_fresh_zero (z : Loc) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.math.big ∗ z ↦ (math.big.Int'.mk false slice.nil) }}
      (App (Val (z @!! go.PointerType math.big.Int'.ty @!! go!"SetUint64")) (Val #(W64 0)))
    {{ RET #z; z ↦ (math.big.Int'.mk false slice.nil) }} := by
  wp_start as Hz
  wp_auto
  wp_method_call
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  wp_method_call
  wp_auto
  wp_method_call
  wp_call
  wp_auto
  have hs : slice.slice slice.nil w64 (W64 0) (W64 0) = slice.nil :=
    (slice_slice_trivial slice.nil).symm
  isimp only [hs] at Hz
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSet_self
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigNeg_self



#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigNorm

#print axioms Perennial.github_com.NethermindEth.juno.core.magnitudeValue_take
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSetBits

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigMake_nil_one
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSetWord_nil_nonzero

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSetUint64_fresh_nonzero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigSetUint64_fresh_zero
