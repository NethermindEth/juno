import ProofCheck.WantedSpec
import ProofCheck.FieldMatch

set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- The machine seen mask represents membership for each schema index. -/
def WantedSeen (count : Nat) (mask : w64) (seen : List Nat) : Prop :=
  ∀ index, index < count → (mask &&& (W64 1 <<< W64 index) = W64 0 ↔ index ∉ seen)

/-- Exhaustive finite bit positions establish separation without a native
    evaluation axiom or an assumption about the generated schema. -/
theorem wantedBits_disjoint (a b : Nat) (ha : a < 64) (hb : b < 64) :
    ((W64 1 <<< W64 a) &&& (W64 1 <<< W64 b) = W64 0) ↔ a ≠ b := by
  have bits : ∀ a b : Fin 64,
      ((W64 1 <<< W64 a.val) &&& (W64 1 <<< W64 b.val) = W64 0) ↔ a.val ≠ b.val := by decide
  exact bits ⟨a, ha⟩ ⟨b, hb⟩

theorem wantedAll_has_bit (count index : Nat) (small : count ≤ 64) (inside : index < count) :
    ((W64 1 <<< W64 count) - W64 1) &&& (W64 1 <<< W64 index) ≠ W64 0 := by
  have bits : ∀ count : Fin 65, ∀ index : Fin 64, index.val < count.val →
      ((W64 1 <<< W64 count.val) - W64 1) &&& (W64 1 <<< W64 index.val) ≠ W64 0 := by decide
  exact bits ⟨count, by omega⟩ ⟨index, by omega⟩ inside

theorem wantedSeen_zero (count : Nat) : WantedSeen count (W64 0) [] := by
  intro index inside
  simp [BitVec.zero_and]

/-- A selected key updates precisely the corresponding membership bit. -/
theorem wantedSeen_mark (count index : Nat) (mask : w64) (seen : List Nat)
    (small : count ≤ 64) (inside : index < count) (hs : WantedSeen count mask seen) :
    WantedSeen count (mask ||| (W64 1 <<< W64 index)) (index :: seen) := by
  intro other bound
  have hor (a b : w64) : a ||| b = W64 0 ↔ a = W64 0 ∧ b = W64 0 := BitVec.or_eq_zero_iff
  rw [BitVec.and_or_distrib_right, hor, hs other bound, wantedBits_disjoint index other (by omega) (by omega)]
  simp [List.mem_cons, and_comm, eq_comm]

/-- A fresh field cannot be suppressed by the all-fields-seen shortcut. -/
theorem wantedSeen_fresh_not_all (count index : Nat) (mask : w64) (seen : List Nat)
    (small : count ≤ 64) (inside : index < count) (hs : WantedSeen count mask seen) (fresh : index ∉ seen) :
    mask ≠ (W64 1 <<< W64 count) - W64 1 := by
  intro same
  have hz := (hs index inside).mpr fresh
  rw [same] at hz
  exact wantedAll_has_bit count index small inside hz

/-- A matching successful machine lookup has the independent schema index. -/
theorem wantedMatch_known (names : List GoString) (key : GoString) (index : Nat) (field : w64)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup) (small : names.length ≤ 64)
    (meaning : WantedKeyMeaning names key (some index))
    (matched : FieldMatchResult names key field true) : field = W64 index := by
  obtain ⟨name, lookup, folded⟩ := meaning
  have inside := (List.getElem?_eq_some_iff.mp lookup).choose
  have hi : sint.Z (W64 index) = index := by word
  apply fieldMatchResult_unique names key field (W64 index) distinct matched
  refine ⟨by word, name, ?_, folded⟩
  simpa only [hi, Int.toNat_natCast] using lookup

/-- Once every field is seen, every supported key is ignored. -/
theorem wantedSelect_all (names : List GoString) (key : GoString) (choice : Option Nat) (seen : List Nat)
    (small : names.length ≤ 64) (meaning : WantedKeyMeaning names key choice)
    (hs : WantedSeen names.length ((W64 1 <<< W64 names.length) - W64 1) seen) :
    wantedSelect seen choice = none := by
  cases choice with
  | none => rfl
  | some index =>
    obtain ⟨name, lookup, folded⟩ := meaning
    have inside := (List.getElem?_eq_some_iff.mp lookup).choose
    have member : index ∈ seen := by
      by_cases member : index ∈ seen
      · exact member
      · exact False.elim (wantedAll_has_bit names.length index small inside ((hs index inside).mpr member))
    simp [wantedSelect, member]

/-- An independently ignored text key cannot pass the machine fresh-bit test. -/
theorem wantedIgnored_mask (names : List GoString) (key : GoString) (choice : Option Nat)
    (seen : List Nat) (mask field : w64) (small : names.length ≤ 64)
    (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (meaning : WantedKeyMeaning names key choice) (ignored : wantedSelect seen choice = none)
    (hs : WantedSeen names.length mask seen) (matched : FieldMatchResult names key field true) :
    mask &&& (W64 1 <<< field) ≠ W64 0 := by
  cases choice with
  | none =>
    obtain ⟨_, name, lookup, folded⟩ := matched
    exact False.elim (meaning name (List.mem_of_getElem? lookup) folded)
  | some index =>
    have same := wantedMatch_known names key index field distinct small meaning matched
    subst field
    obtain ⟨name, lookup, folded⟩ := meaning
    have inside := (List.getElem?_eq_some_iff.mp lookup).choose
    have member : index ∈ seen := by
      by_cases member : index ∈ seen
      · exact member
      · simp [wantedSelect, member] at ignored
    exact fun fresh => (hs index inside).mp fresh member

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedBits_disjoint
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedAll_has_bit
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSeen_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSeen_mark
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSeen_fresh_not_all
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedMatch_known
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedSelect_all
#print axioms Perennial.github_com.NethermindEth.juno.core.wantedIgnored_mask
