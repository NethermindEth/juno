import ProofCheck.Utf8Classify
import ProofCheck.Utf8Tables

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

/-- Sequence length and permitted second-byte interval, derived from the
    independent UTF-8 grammar rather than Go's packed class bytes. -/
def utf8LeadInfo (a : w8) : Option (Nat × Nat × Nat) :=
  if 194 ≤ uint.Z a ∧ uint.Z a ≤ 223 then some (2, 128, 191)
  else if a = W8 224 then some (3, 160, 191)
  else if (225 ≤ uint.Z a ∧ uint.Z a ≤ 236) ∨ (238 ≤ uint.Z a ∧ uint.Z a ≤ 239) then some (3, 128, 191)
  else if a = W8 237 then some (3, 128, 159)
  else if a = W8 240 then some (4, 144, 191)
  else if 241 ≤ uint.Z a ∧ uint.Z a ≤ 243 then some (4, 128, 191)
  else if a = W8 244 then some (4, 128, 143)
  else none

/-- The descriptor's accepted second bytes agree with all three multibyte
    grammar forms. Subsequent continuation bytes have the ordinary tail range. -/
theorem utf8LeadInfo_grammar (a b c d : w8) :
    (Utf8Two a b ↔ (match utf8LeadInfo a with
      | some (2, lo, hi) => lo ≤ uint.Z b ∧ uint.Z b ≤ hi
      | _ => False)) ∧
    (Utf8Three a b c ↔ (match utf8LeadInfo a with
      | some (3, lo, hi) => lo ≤ uint.Z b ∧ uint.Z b ≤ hi ∧ Utf8Tail c
      | _ => False)) ∧
    (Utf8Four a b c d ↔ (match utf8LeadInfo a with
      | some (4, lo, hi) => lo ≤ uint.Z b ∧ uint.Z b ≤ hi ∧ Utf8Tail c ∧ Utf8Tail d
      | _ => False)) := by
  generalize hinfo : utf8LeadInfo a = info
  unfold utf8LeadInfo at hinfo
  repeat' split at hinfo
  all_goals cases hinfo
  all_goals simp_all [Utf8Two, Utf8Three, Utf8Four, Utf8Tail]
  all_goals word

/-- A plain byte-pair view lets finite table checks avoid any abstract Go context. -/
def utf8AcceptBytes : List (w8 × w8) :=
  [(W8 128, W8 191), (W8 160, W8 191), (W8 128, W8 159),
   (W8 144, W8 191), (W8 128, W8 143)] ++ List.replicate 11 (W8 0, W8 0)

variable [GooseExts]

theorem utf8AcceptTable_bytes : utf8AcceptTable.arr =
    utf8AcceptBytes.map (fun p => unicode.utf8.acceptRange.mk p.1 p.2) := by
  rfl

/-- For all 256 starters, the actual initialized tables supply exactly the
    independent descriptor's size and bounds. ASCII and illegal starters are
    classified separately. This is a kernel-checked finite computation. -/
theorem utf8Tables_classify (a : w8) :
    match utf8LeadInfo a with
    | none => uint.Z a < 128 ∨ Utf8BadLead a
    | some (size, lo, hi) =>
      let x := (utf8FirstTable.arr[(uint.Z a).toNat]?).getD (W8 0)
      utf8FirstTable.arr[(uint.Z a).toNat]? = some x ∧
      uint.Z (x &&& W8 7) = size ∧ 2 ≤ size ∧ size ≤ 4 ∧
      uint.Z (x >>> W64 4) < 16 ∧ 128 ≤ lo ∧ lo ≤ hi ∧ hi ≤ 191 ∧
      utf8AcceptTable.arr[(uint.Z (x >>> W64 4)).toNat]? =
        some (unicode.utf8.acceptRange.mk (W8 lo) (W8 hi)) := by
  have h : ∀ a : w8,
      (match utf8LeadInfo a with
      | none => decide (uint.Z a < 128 ∨ (128 ≤ uint.Z a ∧ uint.Z a < 194) ∨ 245 ≤ uint.Z a)
      | some (size, lo, hi) => decide (
        let x := (utf8FirstTable.arr[(uint.Z a).toNat]?).getD (W8 0)
        utf8FirstTable.arr[(uint.Z a).toNat]? = some x ∧
        uint.Z (x &&& W8 7) = size ∧ 2 ≤ size ∧ size ≤ 4 ∧
        uint.Z (x >>> W64 4) < 16 ∧ 128 ≤ lo ∧ lo ≤ hi ∧ hi ≤ 191 ∧
        utf8AcceptBytes[(uint.Z (x >>> W64 4)).toNat]? = some (W8 lo, W8 hi))) = true := by
    unfold utf8LeadInfo
    decide
  have ha := h a
  cases info : utf8LeadInfo a with
  | none => simpa only [info, decide_eq_true_eq, Utf8BadLead] using ha
  | some value =>
    obtain ⟨size, lo, hi⟩ := value
    simp only [info, decide_eq_true_eq] at ha ⊢
    obtain ⟨hfirst, hsize, hmin, hmax, hindex, hlo, hlh, hhi, haccept⟩ := ha
    refine ⟨hfirst, hsize, hmin, hmax, hindex, hlo, hlh, hhi, ?_⟩
    rw [utf8AcceptTable_bytes, List.getElem?_map, haccept]
    rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8LeadInfo_grammar
#print axioms Perennial.github_com.NethermindEth.juno.core.utf8Tables_classify

#print axioms Perennial.github_com.NethermindEth.juno.core.utf8AcceptTable_bytes
