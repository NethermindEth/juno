import ProofCheck.FeltValuesEmpty

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Pure byte condition checked by the fixed-width felt fast path. -/
def LimbWideGuard (wire : List w8) : Prop :=
  37 ≤ wire.length ∧ wire[0]? = some (W8 132) ∧ wire[1]? = some (W8 27) ∧
    wire[10]? = some (W8 27) ∧ wire[19]? = some (W8 27) ∧ wire[28]? = some (W8 27)

/-- The translated readLimbs condition, with its local buffer location supplied.
    This abbreviation must match the production expression when composed. -/
def limbWideGuardExpr (p : Loc) : Expr :=
(App (Val (GoInstruction (Convert go.untypedBool go.bool))) (If (If (If (If (If (App (Val (GoInstruction (GoOp GoGe go.int))) (Pair (Let "$a0" (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p))
  (App (App (Val (GoInstruction (FuncResolve go.len [(go.GoType.SliceType go.byte)]))) (Val #())) (Var "$a0"))) (App (Val (GoInstruction (Convert go.untypedInt go.int))) (Val cursor.feltLen)))) (App (Val (GoInstruction (GoOp GoEquals go.byte))) (Pair (App (Val (GoInstruction (GoLoad go.byte))) (App (Val (GoInstruction (IndexRef (go.GoType.SliceType go.byte)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p)) (Val #(W64 0))))) (App (Val (GoInstruction (Convert go.untypedInt go.byte))) (Val cursor.feltHead)))) (Val #false)) (App (Val (GoInstruction (GoOp GoEquals go.byte))) (Pair (App (Val (GoInstruction (GoLoad go.byte))) (App (Val (GoInstruction (IndexRef (go.GoType.SliceType go.byte)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p)) (Val #(W64 1))))) (Val #(W8 27)))) (Val #false)) (App (Val (GoInstruction (GoOp GoEquals go.byte))) (Pair (App (Val (GoInstruction (GoLoad go.byte))) (App (Val (GoInstruction (IndexRef (go.GoType.SliceType go.byte)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p)) (Val #(W64 10))))) (Val #(W8 27)))) (Val #false)) (App (Val (GoInstruction (GoOp GoEquals go.byte))) (Pair (App (Val (GoInstruction (GoLoad go.byte))) (App (Val (GoInstruction (IndexRef (go.GoType.SliceType go.byte)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p)) (Val #(W64 19))))) (Val #(W8 27)))) (Val #false)) (App (Val (GoInstruction (GoOp GoEquals go.byte))) (Pair (App (Val (GoInstruction (GoLoad go.byte))) (App (Val (GoInstruction (IndexRef (go.GoType.SliceType go.byte)))) (Pair (App (Val (GoInstruction (GoLoad (go.GoType.SliceType go.byte)))) (Val #p)) (Val #(W64 28))))) (Val #(W8 27)))) (Val #false)))

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The short-circuit shape check is safe on every byte slice and preserves
    fractional ownership, reading later offsets only after its length guard. -/
theorem wp_limbWideGuard (p : Loc) (buf : GoSlice) (wire : List w8) (dq : DFrac) :
    {{ (p ↦ buf ∗ buf ↦*{dq} wire : IProp GF) }}
      (limbWideGuardExpr p)
    {{ (ok : Bool), RET #ok; p ↦ buf ∗ buf ↦*{dq} wire ∗ ⌜ok = true ↔ LimbWideGuard wire⌝ }} := by
  iintro %Φ ⟨Hp, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  unfold limbWideGuardExpr
  wp_auto
  by_cases hlong : sint.Z (W64 37) ≤ sint.Z buf.len
  · have hsize : 37 ≤ wire.length := by
      have hlen : wire.length = (sint.Z buf.len).toNat := Hlen.1
      change (37 : Int) ≤ sint.Z buf.len at hlong
      omega
    simp only [hlong, _root_.decide_true]
    wp_auto
    list_elem wire 0 as byte0
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z buf.len; word⟩)]
    wp_apply wp_load_slice_index buf _ wire dq byte0 (by decide) $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact Hbyte0_lookup
    by_cases heq0 : byte0 = W8 132
    · have hg0 : wire[0]? = some (W8 132) := by simpa only [heq0] using Hbyte0_lookup
      simp only [heq0, _root_.decide_true]
      wp_auto
      list_elem wire 1 as byte1
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z buf.len; word⟩)]
      wp_apply wp_load_slice_index buf _ wire dq byte1 (by decide) $$ [Hs] with Hs
      · iframe Hs; ipureintro; exact Hbyte1_lookup
      by_cases heq1 : byte1 = W8 27
      · have hg1 : wire[1]? = some (W8 27) := by simpa only [heq1] using Hbyte1_lookup
        simp only [heq1, _root_.decide_true]
        wp_auto
        list_elem wire 10 as byte10
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z buf.len; word⟩)]
        wp_apply wp_load_slice_index buf _ wire dq byte10 (by decide) $$ [Hs] with Hs
        · iframe Hs; ipureintro; exact Hbyte10_lookup
        by_cases heq10 : byte10 = W8 27
        · have hg10 : wire[10]? = some (W8 27) := by simpa only [heq10] using Hbyte10_lookup
          simp only [heq10, _root_.decide_true]
          wp_auto
          list_elem wire 19 as byte19
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z buf.len; word⟩)]
          wp_apply wp_load_slice_index buf _ wire dq byte19 (by decide) $$ [Hs] with Hs
          · iframe Hs; ipureintro; exact Hbyte19_lookup
          by_cases heq19 : byte19 = W8 27
          · have hg19 : wire[19]? = some (W8 27) := by simpa only [heq19] using Hbyte19_lookup
            simp only [heq19, _root_.decide_true]
            wp_auto
            list_elem wire 28 as byte28
            rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z buf.len; word⟩)]
            wp_apply wp_load_slice_index buf _ wire dq byte28 (by decide) $$ [Hs] with Hs
            · iframe Hs; ipureintro; exact Hbyte28_lookup
            by_cases heq28 : byte28 = W8 27
            · have hg28 : wire[28]? = some (W8 27) := by simpa only [heq28] using Hbyte28_lookup
              simp only [heq28, _root_.decide_true]
              iapply HΦ
              iframe Hp Hs
              ipureintro
              exact ⟨fun _ => ⟨hsize, hg0, hg1, hg10, hg19, hg28⟩, fun _ => rfl⟩
            · simp only [heq28, _root_.decide_false]
              iapply HΦ
              iframe Hp Hs
              ipureintro
              simp only [Bool.false_eq_true, false_iff]
              intro guards
              have hg := guards.2.2.2.2.2
              rw [Hbyte28_lookup] at hg
              exact heq28 (Option.some.inj hg)
          · simp only [heq19, _root_.decide_false]
            wp_auto
            iapply HΦ
            iframe Hp Hs
            ipureintro
            simp only [Bool.false_eq_true, false_iff]
            intro guards
            have hg := guards.2.2.2.2.1
            rw [Hbyte19_lookup] at hg
            exact heq19 (Option.some.inj hg)
        · simp only [heq10, _root_.decide_false]
          wp_auto
          iapply HΦ
          iframe Hp Hs
          ipureintro
          simp only [Bool.false_eq_true, false_iff]
          intro guards
          have hg := guards.2.2.2.1
          rw [Hbyte10_lookup] at hg
          exact heq10 (Option.some.inj hg)
      · simp only [heq1, _root_.decide_false]
        wp_auto
        iapply HΦ
        iframe Hp Hs
        ipureintro
        simp only [Bool.false_eq_true, false_iff]
        intro guards
        have hg := guards.2.2.1
        rw [Hbyte1_lookup] at hg
        exact heq1 (Option.some.inj hg)
    · simp only [heq0, _root_.decide_false]
      wp_auto
      iapply HΦ
      iframe Hp Hs
      ipureintro
      simp only [Bool.false_eq_true, false_iff]
      intro guards
      have hg := guards.2.1
      rw [Hbyte0_lookup] at hg
      exact heq0 (Option.some.inj hg)
  · simp only [hlong, _root_.decide_false]
    wp_auto
    iapply HΦ
    iframe Hp Hs
    ipureintro
    simp only [Bool.false_eq_true, false_iff]
    intro guards
    have hsize := guards.1
    have hlen : wire.length = (sint.Z buf.len).toNat := Hlen.1
    change ¬ (37 : Int) ≤ sint.Z buf.len at hlong
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_limbWideGuard
