import ProofCheck.FieldMark

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

/-- Application field-name folding: only the ASCII uppercase range changes. -/
def fieldFoldByte (b : w8) : w8 :=
  if 65 ≤ uint.Z b ∧ uint.Z b ≤ 90 then b + W8 32 else b

def fieldFoldPrefix (key name : List w8) (n : Nat) : Prop :=
  ∀ i, i < n → (key[i]?).map fieldFoldByte = (name[i]?).map fieldFoldByte

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Case-insensitive matching over equal-length byte strings, including non-ASCII mismatches. -/
theorem wp_equalFoldASCII (key : GoSlice) (xs name : List w8) (dq : DFrac)
    (hlen : xs.length = name.length) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗ key ↦*{dq} xs }}
      (App (App (Val (@! cursor.equalFoldASCII)) (Val #key)) (Val #name))
    {{ (b : Bool), RET #b; key ↦*{dq} xs ∗ ⌜b = true ↔ xs.map fieldFoldByte = name.map fieldFoldByte⌝ }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  wp_auto
  wp_apply github_com.mit_pdos.perennial.goose.model.strings.wp_string_len with %Hname
  ihave HI : (∃ (n : Nat) (oldi : w64),
      "Hrange" ∷ «$range_i_ptr» ↦ W64 n ∗ "Hi" ∷ i_ptr ↦ oldi ∗
      "%Hbound" ∷ ⌜n ≤ name.length⌝ ∗ "%Hprefix" ∷ ⌜fieldFoldPrefix xs name n⌝ : IProp GF)
      $$ [«$range_i» i]
  · iexists 0, (W64 0)
    iframe
    ipureintro
    exact ⟨Nat.zero_le _, fun j hj => False.elim (by omega)⟩
  wp_for HI
  by_cases hmore : n < name.length
  · have hcond : sint.Z (W64 n) < sint.Z (W64 name.length) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hi : sint.Z (W64 n) = n := by word
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by word, by word⟩)]
    rw [hi]
    let a := xs[n]'(by omega)
    let b := name[n]'hmore
    wp_apply wp_load_slice_index key n xs dq a (by omega) $$ [Hs] with Hs
    · iframe Hs; ipureintro; simp only [Int.toNat_natCast]; exact List.getElem?_eq_getElem (by omega)
    have hin : name[sint.nat (W64 n)]? = some b := by
      simp only [sint.nat, hi, Int.toNat_natCast]
      exact List.getElem?_eq_getElem hmore
    rw [ite_eq_left (show 0 ≤ sint.Z (W64 n) by word), hin]
    wp_auto
    wp_join iprop(a_ptr ↦ fieldFoldByte a) at next with [a] as a
    · by_cases hlo : 65 ≤ uint.Z a
      · simp only [show uint.Z (W8 65) = 65 by decide, hlo, _root_.decide_true]
        wp_auto
        by_cases hhi : uint.Z a ≤ 90
        · simp only [show uint.Z (W8 90) = 90 by decide, hhi, _root_.decide_true]
          wp_auto
          wp_join_done
          simp only [fieldFoldByte, hlo, hhi, and_self, ite_true]
          iframe
        · simp only [show uint.Z (W8 90) = 90 by decide, hhi, _root_.decide_false]
          wp_auto
          wp_join_done
          simp only [fieldFoldByte, hlo, hhi, and_false, ite_false]
          iframe
      · simp only [show uint.Z (W8 65) = 65 by decide, hlo, _root_.decide_false]
        wp_auto
        wp_join_done
        simp only [fieldFoldByte, hlo, false_and, ite_false]
        iframe
    wp_join iprop(b_ptr ↦ fieldFoldByte b) at next with [b] as b
    · by_cases hlo : 65 ≤ uint.Z b
      · simp only [show uint.Z (W8 65) = 65 by decide, hlo, _root_.decide_true]
        wp_auto
        by_cases hhi : uint.Z b ≤ 90
        · simp only [show uint.Z (W8 90) = 90 by decide, hhi, _root_.decide_true]
          wp_auto
          wp_join_done
          simp only [fieldFoldByte, hlo, hhi, and_self, ite_true]
          iframe
        · simp only [show uint.Z (W8 90) = 90 by decide, hhi, _root_.decide_false]
          wp_auto
          wp_join_done
          simp only [fieldFoldByte, hlo, hhi, and_false, ite_false]
          iframe
      · simp only [show uint.Z (W8 65) = 65 by decide, hlo, _root_.decide_false]
        wp_auto
        wp_join_done
        simp only [fieldFoldByte, hlo, false_and, ite_false]
        iframe
    by_cases heq : fieldFoldByte a = fieldFoldByte b
    · simp only [heq, _root_.decide_true]
      wp_auto
      wp_for_post
      have hinc : W64 n + W64 1 = W64 (n + 1) := by word
      isimp only [hinc] at Hrange
      iframe
      iexists (n + 1), (W64 n)
      iframe
      ipureintro
      refine ⟨by omega, ?_⟩
      intro j hj
      by_cases hjn : j < n
      · exact Hprefix j hjn
      · have hjn : j = n := by omega
        subst j
        simp only [List.getElem?_eq_getElem (show n < xs.length by omega),
          List.getElem?_eq_getElem hmore, Option.map_some]
        exact congrArg some heq
    · simp only [heq, _root_.decide_false]
      wp_auto
      wp_for_post
      iapply HΦ
      iframe Hs
      ipureintro
      constructor
      · intro h; contradiction
      · intro h
        have hget := congrArg (fun ys : List w8 => ys[n]?) h
        simp only [List.getElem?_map, List.getElem?_eq_getElem (show n < xs.length by omega),
          List.getElem?_eq_getElem hmore, Option.map_some] at hget
        exact False.elim (heq (Option.some.inj hget))
  · have hcond : ¬ sint.Z (W64 n) < sint.Z (W64 name.length) := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    iapply HΦ
    iframe Hs
    ipureintro
    constructor
    · intro _
      apply List.ext_getElem?
      intro j
      simp only [List.getElem?_map]
      by_cases hj : j < name.length
      · exact Hprefix j (by omega)
      · rw [List.getElem?_eq_none (by omega), List.getElem?_eq_none (by omega)]
    · intro _; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_equalFoldASCII
