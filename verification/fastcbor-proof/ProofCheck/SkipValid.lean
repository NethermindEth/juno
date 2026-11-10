import ProofCheck.SkipTag

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]


/-- A child cannot occupy more bytes than the complete sequence containing it. -/
theorem skipItem_length_bound (items : List (List w8)) (item : List w8) (hm : item ∈ items) :
    item.length ≤ items.flatten.length := by
  induction items with
  | nil => simp at hm
  | cons x xs ih =>
    simp only [List.mem_cons] at hm
    simp only [List.flatten_cons, List.length_append]
    rcases hm with rfl | hm
    · omega
    · have := ih hm; omega

/-- Every encoding in the recursive acceptance relation is consumed by the actual skip worker.
All child obligations are discharged recursively on strictly shorter encoded values. -/
theorem wp_skip_valid (c : Loc) (v : cursor.Cursor) (xs wire rem : List w8) (dq : DFrac)
    (depth : w64) (d : Nat) (hd : sint.Z depth = d) (hdepth : d ≤ 32)
    (valid : SkipValid d wire)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #depth))
    {{ RET #interface.nil; c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs }} := by
  by_cases shape : SkipWideShape (wire ++ rem)
  · iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hsize := skipValid_wide_length d wire rem valid shape
    have hroom := skipValid_wide_depth d wire rem valid shape
    have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
    have hb : sint.Z v.pos' + 37 ≤ sint.Z v.buf'.len := by rw [hsize] at Hnext; omega
    wp_apply wp_skip_wide_shape c v depth xs (wire ++ rem) dq hp.1 (by omega) hb hbytes shape
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    isimp only [hsize] at HΦ
    iapply HΦ
    iframe Hc Hs
  · cases valid with
    | @atom d head payload body argument enc atom =>
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1] using
        wp_skip_atom c v depth xs payload body rem head argument dq enc atom hp hbytes
    | @array d head payload argument items enc hm hcount bound children =>
      have hspec : ∀ item ∈ items, ∀ (state : cursor.Cursor) (rest : List w8),
          0 ≤ sint.Z state.pos' ∧ sint.Z state.pos' ≤ sint.Z state.buf'.len →
          subslice (sint.nat state.pos') (sint.nat state.buf'.len) xs = item ++ rest →
          {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
             c ↦ state ∗ state.buf' ↦*{dq} xs }}
            (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(depth + W64 1)))
          {{ RET #interface.nil; c ↦ {state with pos' := state.pos' + W64 item.length} ∗ state.buf' ↦*{dq} xs }} := by
        intro item hmem state rest hpos hb
        have hsmaller := skipItem_length_bound items item hmem
        exact wp_skip_valid c state xs item rest dq (depth + W64 1) (d + 1)
          (by word) (by omega) (children item hmem) hpos hb
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1] using
        wp_skip_container c v xs payload rem head argument (W64 1) depth items dq d enc
          (Or.inl ⟨hm, rfl⟩) hd bound (by change uint.Z argument * 1 = items.length; omega) children hspec hp hbytes shape
    | @map d head payload argument items enc hm hcount bound children =>
      have hspec : ∀ item ∈ items, ∀ (state : cursor.Cursor) (rest : List w8),
          0 ≤ sint.Z state.pos' ∧ sint.Z state.pos' ≤ sint.Z state.buf'.len →
          subslice (sint.nat state.pos') (sint.nat state.buf'.len) xs = item ++ rest →
          {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
             c ↦ state ∗ state.buf' ↦*{dq} xs }}
            (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(depth + W64 1)))
          {{ RET #interface.nil; c ↦ {state with pos' := state.pos' + W64 item.length} ∗ state.buf' ↦*{dq} xs }} := by
        intro item hmem state rest hpos hb
        have hsmaller := skipItem_length_bound items item hmem
        exact wp_skip_valid c state xs item rest dq (depth + W64 1) (d + 1)
          (by word) (by omega) (children item hmem) hpos hb
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1] using
        wp_skip_container c v xs payload rem head argument (W64 2) depth items dq d enc
          (Or.inr ⟨hm, rfl⟩) hd bound (by change uint.Z argument * 2 = items.length; omega) children hspec hp hbytes shape
    | @tag d head next payload tail argument enc hm number bound child =>
      have hspec : ∀ (state : cursor.Cursor),
          0 ≤ sint.Z state.pos' ∧ sint.Z state.pos' ≤ sint.Z state.buf'.len →
          subslice (sint.nat state.pos') (sint.nat state.buf'.len) xs = (next :: tail) ++ rem →
          {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
             c ↦ state ∗ state.buf' ↦*{dq} xs }}
            (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip"))
              (Val #(depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0))))
          {{ RET #interface.nil; c ↦ {state with pos' := state.pos' + W64 (next :: tail).length} ∗ state.buf' ↦*{dq} xs }} := by
        intro state hpos hb
        have hdc : sint.Z (depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0)) =
            (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) := by
          by_cases ht : next &&& W8 224 = W8 192 <;> simp only [ht, ite_true, ite_false] <;> word
        have hbd : (d + if next &&& W8 224 = W8 192 then 1 else 0 : Nat) ≤ 32 := by
          by_cases ht : next &&& W8 224 = W8 192
          · have := bound ht; simp only [ht, ite_true]; omega
          · simp only [ht, ite_false, Nat.add_zero]; exact hdepth
        exact wp_skip_valid c state xs (next :: tail) rem dq
          (depth + W64 (if next &&& W8 224 = W8 192 then 1 else 0))
          (d + if next &&& W8 224 = W8 192 then 1 else 0) hdc hbd child hpos hb
      simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1] using
        wp_skip_tag c v xs payload tail rem head next argument depth dq enc hm number
          ⟨by omega, by omega⟩ (by intro ht; have := bound ht; omega) hspec hp hbytes
termination_by wire.length
decreasing_by
  all_goals try simp_wf
  all_goals subst wire
  all_goals try simp only [List.length_append, List.length_cons]
  all_goals omega

/-- Public Skip accepts every recursively specified ignored value and leaves the exact suffix. -/
theorem wp_Skip_valid (c : Loc) (v : cursor.Cursor) (xs wire rem : List w8) (dq : DFrac)
    (d : Nat) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : SkipValid d wire)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Skip")) (Val #()))
    {{ RET #interface.nil; c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  wp_apply wp_skip_valid c v xs wire rem dq v.depth' d hd hdepth valid hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.skipItem_length_bound
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skip_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Skip_valid
