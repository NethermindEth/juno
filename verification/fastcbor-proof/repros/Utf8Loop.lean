-- Work in progress: general production validator proof. This file is outside
-- the audited target and is expected to fail until every branch is discharged.
-- There are no admitted helpers or changes to the pinned semantics.
-- The remaining goal is the generic ASCII word helper's first load, where the
-- pinned Index rule reverses the (slice, index) operands of IndexRef. See the
-- README approval proposal; this file is not a checked whole-function theorem.
import ProofCheck.Utf8Scan

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

theorem wp_utf8_valid (data : GoSlice) (xs : List w8) (dq : DFrac) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.unicode.utf8 ∗ data ↦*{dq} xs }}
      (App (Val (@! unicode.utf8.Valid)) (Val #data))
    {{ (ok : Bool), RET #ok; data ↦*{dq} xs ∗ ⌜ok = true ↔ Utf8Valid xs⌝ }} := by
  wp_start as Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave Htables := utf8_initialized_tables (GF := GF) $$ []
  · solve_pkg_init
  isimp only [Utf8Initialized] at Htables
  icases Htables with ⟨#Hfirst, #Haccept⟩
  wp_auto
  have hbounds : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) ≤ sint.Z data.len ∧
      sint.Z data.len ≤ sint.Z data.len ∧ sint.Z data.len ≤ sint.Z data.cap :=
    ⟨by decide, Hwf.1, Int.le_refl _, Hwf.2⟩
  rw [ite_eq_left_of_eq_true _ _ (eq_true hbounds)]
  wp_auto
  ihave HI : (∃ off : Nat,
      "Hp" ∷ p_ptr ↦ utf8Window data (W64 off) ∗
      "%Hstate" ∷ ⌜Utf8ScanState xs off⌝ : IProp GF) $$ [p]
  · iexists 0
    isimp only [utf8Window]
    iframe p
    ipureintro
    exact utf8Scan_start xs
  wp_for HI
  have Hoff := Hstate.1
  by_cases more : off < xs.length
  · have hcond : sint.Z (W64 0) < sint.Z (data.len - W64 off) := by
      change sint.Z (W64 0) < sint.Z (data.len - W64 off)
      word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    have hindex : 0 ≤ sint.Z (W64 0) ∧ sint.Z (W64 0) < sint.Z (data.len - W64 off) := ⟨by decide, hcond⟩
    rw [ite_eq_left_of_eq_true _ _ (eq_true hindex), utf8Window_index]
    have href : sint.Z (W64 off) + sint.Z (W64 0) = (off : Int) := by word
    rw [href]
    let b := xs[off]'more
    have lookup : xs[off]? = some b := List.getElem?_eq_getElem more
    wp_apply wp_load_slice_index (t := go.byte) data (off : Int) xs dq b (by omega)
      $$ [Hs] with Hs
    · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup
    have htail : xs.drop off = b :: xs.drop (off + 1) := List.drop_eq_getElem_cons more
    by_cases ascii : uint.Z b < 128
    · simp only [show uint.Z (W8 128) = 128 from rfl, ascii, _root_.decide_true, ite_true]
      wp_auto
      have hs : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) ≤ sint.Z (data.len - W64 off) ∧
          sint.Z (data.len - W64 off) ≤ sint.Z (data.len - W64 off) := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true hs)]
      rw [utf8Window_advance data (W64 off) (W64 1) (by word)]
      rw [show W64 off + W64 1 = W64 ((off + 1 : Nat) : Int) by word]
      wp_auto
      have nextState : Utf8ScanState xs (off + 1) := by
        apply utf8Scan_ascii xs off 1 Hstate (by omega)
        rw [htail]
        simpa [ASCIIBytes] using ascii
      by_cases wide : off + 9 < xs.length
      · have hwide : sint.Z (W64 8) < sint.Z (data.len - W64 ((off + 1 : Nat) : Int)) := by word
        simp only [hwide, _root_.decide_true, ite_true]
        wp_auto
        wp_func_call
        wp_call
        wp_auto
        -- Expected open goal: IndexRef receives (0, window), not (window, 0).
        skip
      · have hshort : ¬ sint.Z (W64 8) < sint.Z (data.len - W64 ((off + 1 : Nat) : Int)) := by word
        simp only [hshort, _root_.decide_false, ite_false]
        wp_auto
        wp_for_post
        iframe
        iexists (off + 1)
        iframe Hp
        ipureintro
        exact nextState
    · simp only [show uint.Z (W8 128) = 128 from rfl, ascii, _root_.decide_false, ite_false]
      wp_auto
      have hbyte : 0 ≤ sint.Z (W64 (uint.Z b)) ∧ sint.Z (W64 (uint.Z b)) < 256 := by word
      have hcast : sint.Z (W64 (uint.Z b)) = uint.Z b := by word
      rw [ite_eq_left_of_eq_true _ _ (eq_true hbyte), hcast]
      have classes := utf8Tables_classify b
      cases info : utf8LeadInfo b with
      | none =>
        simp only [info] at classes
        have bad := classes.resolve_left ascii
        icases array_acc (globalAddr unicode.utf8.first) (uint.Z b) DFrac.discard 256
          utf8FirstTable (W8 241) (by word) (utf8FirstTable_bad_lead b bad) $$ Hfirst
          with ⟨#Hclass, _⟩
        icases array_acc (globalAddr unicode.utf8.acceptRanges) (15 : Int) DFrac.discard 16
          utf8AcceptTable (unicode.utf8.acceptRange.mk (W8 0) (W8 0)) (by decide)
          utf8AcceptTable_invalid $$ Haccept with ⟨#Hrange, _⟩
        wp_auto
        rw [show sint.Z (W64 (uint.Z (W8 241 >>> W64 4))) = 15 by decide]
        wp_auto
        wp_for_post
        have invalid : ¬ Utf8Valid xs := by
          intro valid
          have hv := Hstate.2.mp valid
          rw [htail] at hv
          exact utf8Valid_bad_lead b (xs.drop (off + 1)) bad hv
        have result : false = true ↔ Utf8Valid xs := by simp [invalid]
        wp_end
      | some value =>
        obtain ⟨size, lo, hi⟩ := value
        simp only [info] at classes
        obtain ⟨hfirst, hsize, hmin, hmax, hslot, hlo, hlh, hhi, haccept⟩ := classes
        let code := (utf8FirstTable.arr[(uint.Z b).toNat]?).getD (W8 0)
        icases array_acc (globalAddr unicode.utf8.first) (uint.Z b) DFrac.discard 256
          utf8FirstTable code (by word) hfirst $$ Hfirst with ⟨#Hclass, _⟩
        icases array_acc (globalAddr unicode.utf8.acceptRanges) (uint.Z (code >>> W64 4)) DFrac.discard 16
          utf8AcceptTable (unicode.utf8.acceptRange.mk (W8 lo) (W8 hi)) (by word) haccept
          $$ Haccept with ⟨#Hrange, _⟩
        wp_auto
        have hslotbound : 0 ≤ sint.Z (W64 (uint.Z (code >>> W64 4))) ∧
            sint.Z (W64 (uint.Z (code >>> W64 4))) < 16 := by word
        rw [ite_eq_left_of_eq_true _ _ (eq_true hslotbound)]
        rw [show sint.Z (W64 (uint.Z (code >>> W64 4))) = uint.Z (code >>> W64 4) by word]
        wp_auto
        have hsizeW : W64 (uint.Z (code &&& W8 7)) = W64 size := congrArg W64 hsize
        rw [hsizeW]
        have sizes : size = 2 ∨ size = 3 ∨ size = 4 := by omega
        rcases sizes with rfl | rfl | rfl
        · wp_auto
          by_cases enough : off + 2 ≤ xs.length
          · have hlong : ¬ sint.Z (data.len - W64 off) < sint.Z (W64 2) := by word
            simp only [hlong, _root_.decide_false, ite_false]
            wp_auto
            let b1 := xs[off + 1]'(by omega)
            have lookup1 : xs[off + 1]? = some b1 := List.getElem?_eq_getElem (by omega)
            have hdrop1 : xs.drop (off + 1) = b1 :: xs.drop (off + 2) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 1 < xs.length))
            have htailAll : xs.drop off = b :: b1 :: xs.drop (off + 2) := by
              rw [htail, hdrop1]
            have hi1lo : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
            rw [ite_eq_left_of_eq_true _ _ (eq_true hi1lo), utf8Window_index]
            have href1lo : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
            rw [href1lo]
            wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
              $$ [Hs] with Hs
            · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
            by_cases low1 : uint.Z b1 < uint.Z (W8 lo)
            · simp only [low1, _root_.decide_true, ite_true]
              wp_auto
              wp_for_post
              have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int)) := by word
              have failure : utf8Next (xs.drop off) = none := by
                rw [htailAll, utf8Next_by_info b _ 2 lo hi info ascii]
                simp only [utf8NextInfo, rejected, ite_false]
              have invalid := utf8Scan_reject xs off Hstate more failure
              have result : false = true ↔ Utf8Valid xs := by simp [invalid]
              wp_end
            · simp only [low1, _root_.decide_false, ite_false]
              wp_auto
              have hi1hi : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
              rw [ite_eq_left_of_eq_true _ _ (eq_true hi1hi), utf8Window_index]
              have href1hi : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
              rw [href1hi]
              wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
                $$ [Hs] with Hs
              · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
              by_cases high1 : uint.Z (W8 hi) < uint.Z b1
              · simp only [high1, _root_.decide_true, ite_true]
                wp_auto
                wp_for_post
                have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int)) := by word
                have failure : utf8Next (xs.drop off) = none := by
                  rw [htailAll, utf8Next_by_info b _ 2 lo hi info ascii]
                  simp only [utf8NextInfo, rejected, ite_false]
                have invalid := utf8Scan_reject xs off Hstate more failure
                have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                wp_end
              · simp only [high1, _root_.decide_false, ite_false]
                wp_auto
                have hs : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) ≤ sint.Z (data.len - W64 off) ∧
                    sint.Z (data.len - W64 off) ≤ sint.Z (data.len - W64 off) := by word
                rw [ite_eq_left_of_eq_true _ _ (eq_true hs)]
                rw [utf8Window_advance data (W64 off) (W64 2) (by word)]
                rw [show W64 off + W64 2 = W64 ((off + 2 : Nat) : Int) by word]
                wp_auto
                wp_for_post
                have accepted : (lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) := by word
                have step : utf8Next (xs.drop off) = some (xs.drop (off + 2)) := by
                  rw [htailAll, utf8Next_by_info b _ 2 lo hi info ascii]
                  simp [utf8NextInfo, accepted]
                have consumed : (xs.drop off).length - (xs.drop (off + 2)).length = 2 := by
                  simp only [List.length_drop]
                  omega
                have nextState : Utf8ScanState xs (off + 2) := by
                  simpa only [consumed] using utf8Scan_character xs (xs.drop (off + 2)) off Hstate step
                iframe
                iexists (off + 2)
                iframe Hp
                ipureintro
                exact nextState
          · have hshort : sint.Z (data.len - W64 off) < sint.Z (W64 2) := by word
            simp only [hshort, _root_.decide_true, ite_true]
            wp_auto
            wp_for_post
            have short : (xs.drop (off + 1)).length + 1 < 2 := by
              simp only [List.length_drop]
              omega
            have failure : utf8Next (xs.drop off) = none := by
              rw [htail, utf8Next_by_info b _ 2 lo hi info ascii]
              exact utf8NextInfo_truncated 2 lo hi _ short
            have invalid := utf8Scan_reject xs off Hstate more failure
            have result : false = true ↔ Utf8Valid xs := by simp [invalid]
            wp_end
        · wp_auto
          by_cases enough : off + 3 ≤ xs.length
          · have hlong : ¬ sint.Z (data.len - W64 off) < sint.Z (W64 3) := by word
            simp only [hlong, _root_.decide_false, ite_false]
            wp_auto
            let b1 := xs[off + 1]'(by omega)
            have lookup1 : xs[off + 1]? = some b1 := List.getElem?_eq_getElem (by omega)
            have hdrop1 : xs.drop (off + 1) = b1 :: xs.drop (off + 2) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 1 < xs.length))
            let b2 := xs[off + 2]'(by omega)
            have lookup2 : xs[off + 2]? = some b2 := List.getElem?_eq_getElem (by omega)
            have hdrop2 : xs.drop (off + 2) = b2 :: xs.drop (off + 3) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 2 < xs.length))
            have htailAll : xs.drop off = b :: b1 :: b2 :: xs.drop (off + 3) := by
              rw [htail, hdrop1, hdrop2]
            have hi1lo : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
            rw [ite_eq_left_of_eq_true _ _ (eq_true hi1lo), utf8Window_index]
            have href1lo : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
            rw [href1lo]
            wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
              $$ [Hs] with Hs
            · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
            by_cases low1 : uint.Z b1 < uint.Z (W8 lo)
            · simp only [low1, _root_.decide_true, ite_true]
              wp_auto
              wp_for_post
              have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191)) := by word
              have failure : utf8Next (xs.drop off) = none := by
                rw [htailAll, utf8Next_by_info b _ 3 lo hi info ascii]
                simp only [utf8NextInfo, rejected, ite_false]
              have invalid := utf8Scan_reject xs off Hstate more failure
              have result : false = true ↔ Utf8Valid xs := by simp [invalid]
              wp_end
            · simp only [low1, _root_.decide_false, ite_false]
              wp_auto
              have hi1hi : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
              rw [ite_eq_left_of_eq_true _ _ (eq_true hi1hi), utf8Window_index]
              have href1hi : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
              rw [href1hi]
              wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
                $$ [Hs] with Hs
              · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
              by_cases high1 : uint.Z (W8 hi) < uint.Z b1
              · simp only [high1, _root_.decide_true, ite_true]
                wp_auto
                wp_for_post
                have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191)) := by word
                have failure : utf8Next (xs.drop off) = none := by
                  rw [htailAll, utf8Next_by_info b _ 3 lo hi info ascii]
                  simp only [utf8NextInfo, rejected, ite_false]
                have invalid := utf8Scan_reject xs off Hstate more failure
                have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                wp_end
              · simp only [high1, _root_.decide_false, ite_false]
                wp_auto
                have hi2lo : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) < sint.Z (data.len - W64 off) := by word
                rw [ite_eq_left_of_eq_true _ _ (eq_true hi2lo), utf8Window_index]
                have href2lo : sint.Z (W64 off) + sint.Z (W64 2) = ((off + 2 : Nat) : Int) := by word
                rw [href2lo]
                wp_apply wp_load_slice_index (t := go.byte) data ((off + 2 : Nat) : Int) xs dq b2 (by omega)
                  $$ [Hs] with Hs
                · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup2
                by_cases low2 : uint.Z b2 < uint.Z (W8 128)
                · simp only [low2, _root_.decide_true, ite_true]
                  wp_auto
                  wp_for_post
                  have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191)) := by word
                  have failure : utf8Next (xs.drop off) = none := by
                    rw [htailAll, utf8Next_by_info b _ 3 lo hi info ascii]
                    simp only [utf8NextInfo, rejected, ite_false]
                  have invalid := utf8Scan_reject xs off Hstate more failure
                  have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                  wp_end
                · simp only [low2, _root_.decide_false, ite_false]
                  wp_auto
                  have hi2hi : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) < sint.Z (data.len - W64 off) := by word
                  rw [ite_eq_left_of_eq_true _ _ (eq_true hi2hi), utf8Window_index]
                  have href2hi : sint.Z (W64 off) + sint.Z (W64 2) = ((off + 2 : Nat) : Int) := by word
                  rw [href2hi]
                  wp_apply wp_load_slice_index (t := go.byte) data ((off + 2 : Nat) : Int) xs dq b2 (by omega)
                    $$ [Hs] with Hs
                  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup2
                  by_cases high2 : uint.Z (W8 191) < uint.Z b2
                  · simp only [high2, _root_.decide_true, ite_true]
                    wp_auto
                    wp_for_post
                    have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191)) := by word
                    have failure : utf8Next (xs.drop off) = none := by
                      rw [htailAll, utf8Next_by_info b _ 3 lo hi info ascii]
                      simp only [utf8NextInfo, rejected, ite_false]
                    have invalid := utf8Scan_reject xs off Hstate more failure
                    have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                    wp_end
                  · simp only [high2, _root_.decide_false, ite_false]
                    wp_auto
                    have hs : 0 ≤ sint.Z (W64 3) ∧ sint.Z (W64 3) ≤ sint.Z (data.len - W64 off) ∧
                        sint.Z (data.len - W64 off) ≤ sint.Z (data.len - W64 off) := by word
                    rw [ite_eq_left_of_eq_true _ _ (eq_true hs)]
                    rw [utf8Window_advance data (W64 off) (W64 3) (by word)]
                    rw [show W64 off + W64 3 = W64 ((off + 3 : Nat) : Int) by word]
                    wp_auto
                    wp_for_post
                    have accepted : (lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) := by word
                    have step : utf8Next (xs.drop off) = some (xs.drop (off + 3)) := by
                      rw [htailAll, utf8Next_by_info b _ 3 lo hi info ascii]
                      simp [utf8NextInfo, accepted]
                    have consumed : (xs.drop off).length - (xs.drop (off + 3)).length = 3 := by
                      simp only [List.length_drop]
                      omega
                    have nextState : Utf8ScanState xs (off + 3) := by
                      simpa only [consumed] using utf8Scan_character xs (xs.drop (off + 3)) off Hstate step
                    iframe
                    iexists (off + 3)
                    iframe Hp
                    ipureintro
                    exact nextState
          · have hshort : sint.Z (data.len - W64 off) < sint.Z (W64 3) := by word
            simp only [hshort, _root_.decide_true, ite_true]
            wp_auto
            wp_for_post
            have short : (xs.drop (off + 1)).length + 1 < 3 := by
              simp only [List.length_drop]
              omega
            have failure : utf8Next (xs.drop off) = none := by
              rw [htail, utf8Next_by_info b _ 3 lo hi info ascii]
              exact utf8NextInfo_truncated 3 lo hi _ short
            have invalid := utf8Scan_reject xs off Hstate more failure
            have result : false = true ↔ Utf8Valid xs := by simp [invalid]
            wp_end
        · wp_auto
          by_cases enough : off + 4 ≤ xs.length
          · have hlong : ¬ sint.Z (data.len - W64 off) < sint.Z (W64 4) := by word
            simp only [hlong, _root_.decide_false, ite_false]
            wp_auto
            let b1 := xs[off + 1]'(by omega)
            have lookup1 : xs[off + 1]? = some b1 := List.getElem?_eq_getElem (by omega)
            have hdrop1 : xs.drop (off + 1) = b1 :: xs.drop (off + 2) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 1 < xs.length))
            let b2 := xs[off + 2]'(by omega)
            have lookup2 : xs[off + 2]? = some b2 := List.getElem?_eq_getElem (by omega)
            have hdrop2 : xs.drop (off + 2) = b2 :: xs.drop (off + 3) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 2 < xs.length))
            let b3 := xs[off + 3]'(by omega)
            have lookup3 : xs[off + 3]? = some b3 := List.getElem?_eq_getElem (by omega)
            have hdrop3 : xs.drop (off + 3) = b3 :: xs.drop (off + 4) := by
              simpa only [Nat.add_assoc] using (List.drop_eq_getElem_cons (by omega : off + 3 < xs.length))
            have htailAll : xs.drop off = b :: b1 :: b2 :: b3 :: xs.drop (off + 4) := by
              rw [htail, hdrop1, hdrop2, hdrop3]
            have hi1lo : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
            rw [ite_eq_left_of_eq_true _ _ (eq_true hi1lo), utf8Window_index]
            have href1lo : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
            rw [href1lo]
            wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
              $$ [Hs] with Hs
            · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
            by_cases low1 : uint.Z b1 < uint.Z (W8 lo)
            · simp only [low1, _root_.decide_true, ite_true]
              wp_auto
              wp_for_post
              have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
              have failure : utf8Next (xs.drop off) = none := by
                rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                simp only [utf8NextInfo, rejected, ite_false]
              have invalid := utf8Scan_reject xs off Hstate more failure
              have result : false = true ↔ Utf8Valid xs := by simp [invalid]
              wp_end
            · simp only [low1, _root_.decide_false, ite_false]
              wp_auto
              have hi1hi : 0 ≤ sint.Z (W64 1) ∧ sint.Z (W64 1) < sint.Z (data.len - W64 off) := by word
              rw [ite_eq_left_of_eq_true _ _ (eq_true hi1hi), utf8Window_index]
              have href1hi : sint.Z (W64 off) + sint.Z (W64 1) = ((off + 1 : Nat) : Int) := by word
              rw [href1hi]
              wp_apply wp_load_slice_index (t := go.byte) data ((off + 1 : Nat) : Int) xs dq b1 (by omega)
                $$ [Hs] with Hs
              · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup1
              by_cases high1 : uint.Z (W8 hi) < uint.Z b1
              · simp only [high1, _root_.decide_true, ite_true]
                wp_auto
                wp_for_post
                have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
                have failure : utf8Next (xs.drop off) = none := by
                  rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                  simp only [utf8NextInfo, rejected, ite_false]
                have invalid := utf8Scan_reject xs off Hstate more failure
                have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                wp_end
              · simp only [high1, _root_.decide_false, ite_false]
                wp_auto
                have hi2lo : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) < sint.Z (data.len - W64 off) := by word
                rw [ite_eq_left_of_eq_true _ _ (eq_true hi2lo), utf8Window_index]
                have href2lo : sint.Z (W64 off) + sint.Z (W64 2) = ((off + 2 : Nat) : Int) := by word
                rw [href2lo]
                wp_apply wp_load_slice_index (t := go.byte) data ((off + 2 : Nat) : Int) xs dq b2 (by omega)
                  $$ [Hs] with Hs
                · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup2
                by_cases low2 : uint.Z b2 < uint.Z (W8 128)
                · simp only [low2, _root_.decide_true, ite_true]
                  wp_auto
                  wp_for_post
                  have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
                  have failure : utf8Next (xs.drop off) = none := by
                    rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                    simp only [utf8NextInfo, rejected, ite_false]
                  have invalid := utf8Scan_reject xs off Hstate more failure
                  have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                  wp_end
                · simp only [low2, _root_.decide_false, ite_false]
                  wp_auto
                  have hi2hi : 0 ≤ sint.Z (W64 2) ∧ sint.Z (W64 2) < sint.Z (data.len - W64 off) := by word
                  rw [ite_eq_left_of_eq_true _ _ (eq_true hi2hi), utf8Window_index]
                  have href2hi : sint.Z (W64 off) + sint.Z (W64 2) = ((off + 2 : Nat) : Int) := by word
                  rw [href2hi]
                  wp_apply wp_load_slice_index (t := go.byte) data ((off + 2 : Nat) : Int) xs dq b2 (by omega)
                    $$ [Hs] with Hs
                  · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup2
                  by_cases high2 : uint.Z (W8 191) < uint.Z b2
                  · simp only [high2, _root_.decide_true, ite_true]
                    wp_auto
                    wp_for_post
                    have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
                    have failure : utf8Next (xs.drop off) = none := by
                      rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                      simp only [utf8NextInfo, rejected, ite_false]
                    have invalid := utf8Scan_reject xs off Hstate more failure
                    have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                    wp_end
                  · simp only [high2, _root_.decide_false, ite_false]
                    wp_auto
                    have hi3lo : 0 ≤ sint.Z (W64 3) ∧ sint.Z (W64 3) < sint.Z (data.len - W64 off) := by word
                    rw [ite_eq_left_of_eq_true _ _ (eq_true hi3lo), utf8Window_index]
                    have href3lo : sint.Z (W64 off) + sint.Z (W64 3) = ((off + 3 : Nat) : Int) := by word
                    rw [href3lo]
                    wp_apply wp_load_slice_index (t := go.byte) data ((off + 3 : Nat) : Int) xs dq b3 (by omega)
                      $$ [Hs] with Hs
                    · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup3
                    by_cases low3 : uint.Z b3 < uint.Z (W8 128)
                    · simp only [low3, _root_.decide_true, ite_true]
                      wp_auto
                      wp_for_post
                      have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
                      have failure : utf8Next (xs.drop off) = none := by
                        rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                        simp only [utf8NextInfo, rejected, ite_false]
                      have invalid := utf8Scan_reject xs off Hstate more failure
                      have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                      wp_end
                    · simp only [low3, _root_.decide_false, ite_false]
                      wp_auto
                      have hi3hi : 0 ≤ sint.Z (W64 3) ∧ sint.Z (W64 3) < sint.Z (data.len - W64 off) := by word
                      rw [ite_eq_left_of_eq_true _ _ (eq_true hi3hi), utf8Window_index]
                      have href3hi : sint.Z (W64 off) + sint.Z (W64 3) = ((off + 3 : Nat) : Int) := by word
                      rw [href3hi]
                      wp_apply wp_load_slice_index (t := go.byte) data ((off + 3 : Nat) : Int) xs dq b3 (by omega)
                        $$ [Hs] with Hs
                      · iframe Hs; ipureintro; simpa only [Int.toNat_natCast] using lookup3
                      by_cases high3 : uint.Z (W8 191) < uint.Z b3
                      · simp only [high3, _root_.decide_true, ite_true]
                        wp_auto
                        wp_for_post
                        have rejected : ¬ ((lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191)) := by word
                        have failure : utf8Next (xs.drop off) = none := by
                          rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                          simp only [utf8NextInfo, rejected, ite_false]
                        have invalid := utf8Scan_reject xs off Hstate more failure
                        have result : false = true ↔ Utf8Valid xs := by simp [invalid]
                        wp_end
                      · simp only [high3, _root_.decide_false, ite_false]
                        wp_auto
                        have hs : 0 ≤ sint.Z (W64 4) ∧ sint.Z (W64 4) ≤ sint.Z (data.len - W64 off) ∧
                            sint.Z (data.len - W64 off) ≤ sint.Z (data.len - W64 off) := by word
                        rw [ite_eq_left_of_eq_true _ _ (eq_true hs)]
                        rw [utf8Window_advance data (W64 off) (W64 4) (by word)]
                        rw [show W64 off + W64 4 = W64 ((off + 4 : Nat) : Int) by word]
                        wp_auto
                        wp_for_post
                        have accepted : (lo : Int) ≤ uint.Z b1 ∧ uint.Z b1 ≤ (hi : Int) ∧ (128 ≤ uint.Z b2 ∧ uint.Z b2 ≤ 191) ∧ (128 ≤ uint.Z b3 ∧ uint.Z b3 ≤ 191) := by word
                        have step : utf8Next (xs.drop off) = some (xs.drop (off + 4)) := by
                          rw [htailAll, utf8Next_by_info b _ 4 lo hi info ascii]
                          simp [utf8NextInfo, accepted]
                        have consumed : (xs.drop off).length - (xs.drop (off + 4)).length = 4 := by
                          simp only [List.length_drop]
                          omega
                        have nextState : Utf8ScanState xs (off + 4) := by
                          simpa only [consumed] using utf8Scan_character xs (xs.drop (off + 4)) off Hstate step
                        iframe
                        iexists (off + 4)
                        iframe Hp
                        ipureintro
                        exact nextState
          · have hshort : sint.Z (data.len - W64 off) < sint.Z (W64 4) := by word
            simp only [hshort, _root_.decide_true, ite_true]
            wp_auto
            wp_for_post
            have short : (xs.drop (off + 1)).length + 1 < 4 := by
              simp only [List.length_drop]
              omega
            have failure : utf8Next (xs.drop off) = none := by
              rw [htail, utf8Next_by_info b _ 4 lo hi info ascii]
              exact utf8NextInfo_truncated 4 lo hi _ short
            have invalid := utf8Scan_reject xs off Hstate more failure
            have result : false = true ↔ Utf8Valid xs := by simp [invalid]
            wp_end
  · have hcond : ¬ sint.Z (W64 0) < sint.Z (data.len - W64 off) := by
      change ¬ sint.Z (W64 0) < sint.Z (data.len - W64 off)
      word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    iapply HΦ
    iframe Hs
    ipureintro
    exact ⟨fun _ => utf8Scan_done xs off Hstate (by omega), fun _ => rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
