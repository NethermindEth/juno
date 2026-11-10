import ProofCheck.SkipContainerBoundary

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


/-- A container loop consumes an arbitrary valid prefix then propagates a proved
child failure. Its own wide shortcut is proved for valid children and excluded
for the failing input; no temporary cursor progress is hidden. -/
theorem wp_skipElements_prefix_reject (c : Loc) (v : cursor.Cursor) (xs rem : List w8)
    (items : List (List w8)) (dq : DFrac) (count limit perCount depth out : w64) (d : Nat) (R : IProp GF) (E : Loc → IProp GF)
    (hd : sint.Z depth = d) (hdepth : d < 32)
    (hper : perCount = W64 1 ∨ perCount = W64 2)
    (hcount : (items.length : Int) < uint.Z count * uint.Z perCount)
    (htotal : uint.Z count * uint.Z perCount ≤ sint.Z v.buf'.len) (hlimit : uint.Z count ≤ uint.Z limit)
    (valid : ∀ item ∈ items, SkipValid (d + 1) item)
    (shape : d + 1 < 32 → 37 ≤ rem.length → ¬ SkipWideShape rem)
    (hbad :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ {v with pos' := v.pos' + W64 items.flatten.length} ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skip")) (Val #(depth + W64 1)))
      {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
         c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }})
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = items.flatten ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (App (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"skipElements"))
        (Val #count)) (Val #limit)) (Val #perCount)) (Val #depth))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       c ↦ {v with pos' := out} ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  wp_start as ⟨HR, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  have hitems := skipValid_items_bound (d + 1) items valid
  have hwhole := input_consumption xs items.flatten rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hdep : ¬ sint.Z (W64 32) < sint.Z (depth + W64 1) := by word
  have hlim : ¬ uint.Z limit < uint.Z count := by omega
  wp_auto
  simp only [hdep, _root_.decide_false]
  wp_auto
  simp only [hlim, _root_.decide_false]
  wp_auto
  ihave HI : (∃ (done todo : List (List w8)) (pos : w64),
      "Hn" ∷ n_ptr ↦ (count * perCount - W64 done.length) ∗
      "Hc" ∷ c ↦ {v with pos' := pos} ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + done.flatten.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatten ++ rem⌝ : IProp GF)
      $$ [n Hc]
  · iexists [], items, v.pos'
    have hz : count * perCount - W64 ([] : List (List w8)).length = count * perCount := by
      simp only [List.length_nil]; word
    simp only [hz]
    iframe
    ipureintro
    exact ⟨rfl, hp, by simp, hbytes⟩
  have hf : (#false : val) ≠ #true := fun h => Bool.noConfusion (go.intoVal_inj h)
  wp_for HI
  have htlen : todo.length ≤ items.length := by rw [Hsplit, List.length_append]; omega
  have hdlen : done.length ≤ items.length := by rw [Hsplit, List.length_append]; omega
  cases todo with
  | nil =>
    have hcond : uint.Z (W64 0) < uint.Z (count * perCount - W64 done.length) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨Hpos.1, Hpos.2, Hwf.2⟩)]
    wp_auto
    have hdone : done = items := by simpa using Hsplit.symm
    have hfinal : pos = v.pos' + W64 items.flatten.length := by rw [hdone] at Hposition; word
    have hremaining : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = rem := by
      simpa only [List.flatten_nil, List.nil_append] using Hremaining
    icases (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
    isimp only [hremaining] at Hbuf
    ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
    by_cases hdep : sint.Z (depth + W64 1) < sint.Z (W64 32) <;>
      simp only [hdep, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        by_cases hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - pos) <;>
          simp only [hl, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - pos); word⟩)]
        have hblen : 37 ≤ rem.length := by
          have heq := Hbuflen.1
          change rem.length = (sint.Z (v.buf'.len - pos)).toNat at heq
          change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
          omega
        list_elem rem 0 as byte0
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
          rem dq byte0 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte0_lookup
        by_cases heq0 : byte0 = W8 132 <;> simp only [heq0, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - pos); word⟩)]
        have hblen : 37 ≤ rem.length := by
          have heq := Hbuflen.1
          change rem.length = (sint.Z (v.buf'.len - pos)).toNat at heq
          change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
          omega
        list_elem rem 1 as byte1
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
          rem dq byte1 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte1_lookup
        by_cases heq1 : byte1 = W8 27 <;> simp only [heq1, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - pos); word⟩)]
        have hblen : 37 ≤ rem.length := by
          have heq := Hbuflen.1
          change rem.length = (sint.Z (v.buf'.len - pos)).toNat at heq
          change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
          omega
        list_elem rem 10 as byte10
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
          rem dq byte10 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte10_lookup
        by_cases heq10 : byte10 = W8 27 <;> simp only [heq10, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - pos); word⟩)]
        have hblen : 37 ≤ rem.length := by
          have heq := Hbuflen.1
          change rem.length = (sint.Z (v.buf'.len - pos)).toNat at heq
          change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
          omega
        list_elem rem 19 as byte19
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
          rem dq byte19 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte19_lookup
        by_cases heq19 : byte19 = W8 27 <;> simp only [heq19, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - pos); word⟩)]
        have hblen : 37 ≤ rem.length := by
          have heq := Hbuflen.1
          change rem.length = (sint.Z (v.buf'.len - pos)).toNat at heq
          change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
          omega
        list_elem rem 28 as byte28
        wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
          rem dq byte28 (by decide) $$ [Hbuf] with Hbuf
        · iframe Hbuf; ipureintro; exact Hbyte28_lookup
        by_cases heq28 : byte28 = W8 27 <;> simp only [heq28, _root_.decide_true, _root_.decide_false] <;> wp_auto
    all_goals
      try
        exfalso
        apply shape (by word) hblen
        exact ⟨by simpa only [heq0] using Hbyte0_lookup,
          by simpa only [heq1] using Hbyte1_lookup, by simpa only [heq10] using Hbyte10_lookup,
          by simpa only [heq19] using Hbyte19_lookup, by simpa only [heq28] using Hbyte28_lookup⟩
    all_goals
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
        rw [hremaining]
        iframe Hpre Hbuf Hpost
      isimp only [hfinal] at Hc
      wp_apply hbad $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe HR Hc Hs
      wp_for_post
      wp_end
  | cons item todo =>
    have hcond : uint.Z (W64 0) < uint.Z (count * perCount - W64 done.length) := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨Hpos.1, Hpos.2, Hwf.2⟩)]
    wp_auto
    have hmember : item ∈ items := by rw [Hsplit]; simp
    have hvalid := valid item hmember
    have hitembytes : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ (todo.flatten ++ rem) := by
      simpa only [List.flatten_cons, List.append_assoc] using Hremaining
    have Hnext := input_consumption xs item (todo.flatten ++ rem) pos v.buf'.len Hpos Hlen.1 hitembytes
    icases (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).1 $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
    isimp only [hitembytes] at Hbuf
    ihave %Hbuflen := ownSlice_len _ _ _ $$ Hbuf
    by_cases itemShape : SkipWideShape (item ++ (todo.flatten ++ rem))
    · have hsize := skipValid_wide_length (d + 1) item (todo.flatten ++ rem) hvalid itemShape
      have hroom := skipValid_wide_depth (d + 1) item (todo.flatten ++ rem) hvalid itemShape
      have hdep : sint.Z (depth + W64 1) < sint.Z (W64 32) := by word
      have hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - pos) := by rw [hsize] at Hnext; word
      simp only [hdep, _root_.decide_true]
      wp_auto
      simp only [hl, _root_.decide_true]
      wp_auto
      obtain ⟨h0, h1, h10, h19, h28⟩ := itemShape
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - pos); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
        (item ++ (todo.flatten ++ rem)) dq (W8 132) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact h0
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - pos); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
        (item ++ (todo.flatten ++ rem)) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact h1
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - pos); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
        (item ++ (todo.flatten ++ rem)) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact h10
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - pos); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
        (item ++ (todo.flatten ++ rem)) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact h19
      rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - pos); word⟩)]
      wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
        (item ++ (todo.flatten ++ rem)) dq (W8 27) (by decide) $$ [Hbuf] with Hbuf
      · iframe Hbuf; ipureintro; exact h28
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
        rw [hitembytes]
        iframe Hpre Hbuf Hpost
      have hsizeInt : (item.length : Int) = 37 := by omega
      isimp only [← hsizeInt] at Hc
      have hdec : (count * perCount - W64 done.length) - W64 1 = count * perCount - W64 (done ++ [item]).length := by simp only [List.length_append, List.length_cons, List.length_nil]; word
      have hdonebytes : (done ++ [item]).flatten.length = done.flatten.length + item.length := by simp
      wp_for_post
      isimp only [hdec] at Hn
      iframe
      iexists (done ++ [item]), todo, (pos + W64 item.length)
      iframe
      ipureintro
      refine ⟨?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · rw [Hnext.1]; omega
      · rw [Hnext.1, hdonebytes]; omega
    · by_cases hdep : sint.Z (depth + W64 1) < sint.Z (W64 32) <;>
        simp only [hdep, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          by_cases hl : sint.Z (W64 37) ≤ sint.Z (v.buf'.len - pos) <;>
            simp only [hl, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 0) < sint.Z (v.buf'.len - pos); word⟩)]
          have hblen : 37 ≤ (item ++ (todo.flatten ++ rem)).length := by
            have heq := Hbuflen.1
            change (item ++ (todo.flatten ++ rem)).length = (sint.Z (v.buf'.len - pos)).toNat at heq
            change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
            omega
          list_elem (item ++ (todo.flatten ++ rem)) 0 as byte0
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
            (item ++ (todo.flatten ++ rem)) dq byte0 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte0_lookup
          by_cases heq0 : byte0 = W8 132 <;> simp only [heq0, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 1) < sint.Z (v.buf'.len - pos); word⟩)]
          have hblen : 37 ≤ (item ++ (todo.flatten ++ rem)).length := by
            have heq := Hbuflen.1
            change (item ++ (todo.flatten ++ rem)).length = (sint.Z (v.buf'.len - pos)).toNat at heq
            change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
            omega
          list_elem (item ++ (todo.flatten ++ rem)) 1 as byte1
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
            (item ++ (todo.flatten ++ rem)) dq byte1 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte1_lookup
          by_cases heq1 : byte1 = W8 27 <;> simp only [heq1, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 10) < sint.Z (v.buf'.len - pos); word⟩)]
          have hblen : 37 ≤ (item ++ (todo.flatten ++ rem)).length := by
            have heq := Hbuflen.1
            change (item ++ (todo.flatten ++ rem)).length = (sint.Z (v.buf'.len - pos)).toNat at heq
            change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
            omega
          list_elem (item ++ (todo.flatten ++ rem)) 10 as byte10
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
            (item ++ (todo.flatten ++ rem)) dq byte10 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte10_lookup
          by_cases heq10 : byte10 = W8 27 <;> simp only [heq10, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 19) < sint.Z (v.buf'.len - pos); word⟩)]
          have hblen : 37 ≤ (item ++ (todo.flatten ++ rem)).length := by
            have heq := Hbuflen.1
            change (item ++ (todo.flatten ++ rem)).length = (sint.Z (v.buf'.len - pos)).toNat at heq
            change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
            omega
          list_elem (item ++ (todo.flatten ++ rem)) 19 as byte19
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
            (item ++ (todo.flatten ++ rem)) dq byte19 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte19_lookup
          by_cases heq19 : byte19 = W8 27 <;> simp only [heq19, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨by decide, by change sint.Z (W64 28) < sint.Z (v.buf'.len - pos); word⟩)]
          have hblen : 37 ≤ (item ++ (todo.flatten ++ rem)).length := by
            have heq := Hbuflen.1
            change (item ++ (todo.flatten ++ rem)).length = (sint.Z (v.buf'.len - pos)).toNat at heq
            change (37 : Int) ≤ sint.Z (v.buf'.len - pos) at hl
            omega
          list_elem (item ++ (todo.flatten ++ rem)) 28 as byte28
          wp_apply wp_load_slice_index (slice.slice v.buf' w8 pos v.buf'.len) _
            (item ++ (todo.flatten ++ rem)) dq byte28 (by decide) $$ [Hbuf] with Hbuf
          · iframe Hbuf; ipureintro; exact Hbyte28_lookup
          by_cases heq28 : byte28 = W8 27 <;> simp only [heq28, _root_.decide_true, _root_.decide_false] <;> wp_auto
      all_goals
        try
          exfalso
          apply itemShape
          exact ⟨by simpa only [heq0] using Hbyte0_lookup,
            by simpa only [heq1] using Hbyte1_lookup, by simpa only [heq10] using Hbyte10_lookup,
            by simpa only [heq19] using Hbyte19_lookup, by simpa only [heq28] using Hbyte28_lookup⟩
      all_goals
        ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
        · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
          rw [hitembytes]
          iframe Hpre Hbuf Hpost
        wp_apply wp_skip_valid c {v with pos' := pos} xs item (todo.flatten ++ rem) dq
          (depth + W64 1) (d + 1) (by word) (by omega) hvalid Hpos hitembytes
          $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        ieval (change (c ↦ {v with pos' := pos + W64 item.length} : IProp GF) ⊢ _) at Hc
        have hdec : (count * perCount - W64 done.length) - W64 1 = count * perCount - W64 (done ++ [item]).length := by simp only [List.length_append, List.length_cons, List.length_nil]; word
        have hdonebytes : (done ++ [item]).flatten.length = done.flatten.length + item.length := by simp
        wp_for_post
        isimp only [hdec] at Hn
        iframe
        iexists (done ++ [item]), todo, (pos + W64 item.length)
        iframe
        ipureintro
        refine ⟨?_, ⟨?_, Hnext.2.1⟩, ?_, Hnext.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · rw [Hnext.1]; omega
        · rw [Hnext.1, hdonebytes]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_skipElements_prefix_reject
