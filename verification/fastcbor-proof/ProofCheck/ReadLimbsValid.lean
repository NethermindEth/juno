import ProofCheck.LimbItemsWire

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 12800000
set_option maxRecDepth 4096

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual loop decodes arbitrary sequences of valid nonnullable felts,
    including mixtures of fixed-width and slow encodings, into owned storage. -/
theorem wp_readLimbs_items (c : Loc) (out : GoSlice) (old : List (GoArray w64 4))
    (v : cursor.Cursor) (xs rem : List w8) (dq : DFrac) (depth : Nat)
    (items : List (LimbValueWire depth)) (count : old.length = items.length)
    (hd : sint.Z v.depth' = depth) (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = items.flatMap LimbValueWire.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       out ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ RET #interface.nil; out ↦* items.map LimbValueWire.value ∗
       c ↦ {v with pos' := v.pos' + W64 (items.flatMap LimbValueWire.bytes).length} ∗
       v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hout, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Houtlen := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = items.length := by simp only [count, sint.nat, sint.Z] at Houtlen ⊢; omega
  have hfit : (items.length : Int) < 2^63 := by word
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  rename_i itemIndex
  irename i => Hrange
  irename i => Hi
  irename buf => Hbufptr
  ihave HI : (∃ (done todo : List (LimbValueWire depth)) (unwritten : List (GoArray w64 4))
      (pos lag oldi : w64),
      "Hrange" ∷ i_ptr ↦ W64 done.length ∗ "Hi" ∷ itemIndex ↦ oldi ∗
      "Hout" ∷ out ↦* (done.map LimbValueWire.value ++ unwritten) ∗
      "Hc" ∷ c ↦ {v with pos' := lag} ∗
      "Hbufptr" ∷ buf_ptr ↦ slice.slice v.buf' w8 pos v.buf'.len ∗
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗ "%Hslots" ∷ ⌜unwritten.length = todo.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + (done.flatMap LimbValueWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap LimbValueWire.bytes ++ rem⌝ : IProp GF)
      $$ [Hrange Hi Hout Hc Hbufptr]
  · iexists [], items, old, v.pos', v.pos', (W64 0)
    isimp only [List.map_nil, List.nil_append]
    iframe
    ipureintro
    exact ⟨trivial, count, hp, by simp, hbytes⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  by_cases more : done.length < items.length
  · have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    cases todo with
    | nil => simp only [List.length_nil] at hlength; omega
    | cons item todo =>
      cases unwritten with
      | nil => simp at Hslots
      | cons previous unwritten =>
        have hindex : 0 ≤ sint.Z (W64 done.length) ∧ sint.Z (W64 done.length) < sint.Z out.len := by word
        have hidxnat : (sint.Z (W64 done.length)).toNat = done.length := by word
        have hlookup : (done.map LimbValueWire.value ++ previous :: unwritten)[(sint.Z (W64 done.length)).toNat]? = some previous := by
          rw [hidxnat]
          simp [List.getElem?_append]
        rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
        wp_apply +noauto wp_load_slice_index out _ (done.map LimbValueWire.value ++ previous :: unwritten)
          (DFrac.own 1) previous hindex.1 $$ [Hout] with Hout
        · iframe Hout; ipureintro; exact hlookup
        wp_pures
        wp_load
        wp_pures
        wp_store
        wp_pures
        icases (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).1
          $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
        isimp only [Hremaining, List.flatMap_cons, List.append_assoc] at Hbuf
        have hguard := wp_limbWideGuard (GF := GF) buf_ptr (slice.slice v.buf' w8 pos v.buf'.len)
          (item.bytes ++ (todo.flatMap LimbValueWire.bytes ++ rem)) dq
        unfold limbWideGuardExpr at hguard
        wp_apply +noauto hguard $$ [Hbufptr Hbuf] as %ok ⟨Hbufptr, Hbuf, %Hguard⟩
        · iframe Hbufptr Hbuf
        have hstep := input_consumption xs item.bytes (todo.flatMap LimbValueWire.bytes ++ rem)
          pos v.buf'.len Hpos Hlen.1 (by simpa only [List.flatMap_cons, List.append_assoc] using Hremaining)
        have hinc : W64 done.length + W64 1 = W64 (done ++ [item]).length := by simp; word
        have hset : (done.map LimbValueWire.value ++ previous :: unwritten).set (sint.Z (W64 done.length)).toNat item.value =
            (done ++ [item]).map LimbValueWire.value ++ unwritten := by
          rw [hidxnat, ← List.length_map (f := LimbValueWire.value)]
          simpa using limbValues_prefix_step (done.map LimbValueWire.value) unwritten previous item.value
        cases ok with
        | false =>
          wp_auto
          ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
          · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
            rw [Hremaining]
            simpa only [List.flatMap_cons, List.append_assoc] using (show _ from by iframe Hpre Hbuf Hpost)
          have hreset (x : w64) : v.buf'.len - (v.buf'.len - x) = x := by word
          isimp only [hreset] at Hc
          rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
          wp_auto
          wp_apply wp_limbs_slice_valid c out (done.map LimbValueWire.value ++ previous :: unwritten)
            (sint.Z (W64 done.length)) previous item.value {v with pos' := pos}
            xs item.bytes (todo.flatMap LimbValueWire.bytes ++ rem) dq depth hindex.1 hlookup
            hd (by omega) item.valid Hpos (by simpa only [List.flatMap_cons, List.append_assoc] using Hremaining)
            $$ [Hout Hc Hs] with ⟨Hout, Hc, Hs, %Hadvance⟩
          · iframe Hout Hc Hs
          dsimp only at Hadvance
          have hb : 0 ≤ sint.Z (pos + W64 item.bytes.length) ∧ sint.Z (pos + W64 item.bytes.length) ≤ sint.Z v.buf'.len ∧
              sint.Z v.buf'.len ≤ sint.Z v.buf'.cap := by constructor; rw [Hadvance.1]; omega; exact ⟨Hadvance.2.1, Hwf.2⟩
          rw [ite_eq_left_of_eq_true _ _ (eq_true hb)]
          wp_auto
          wp_for_post
          isimp only [hinc] at Hrange
          isimp only [hset] at Hout
          iframe
          iexists (done ++ [item]), todo, unwritten, (pos + W64 item.bytes.length), (pos + W64 item.bytes.length), (W64 done.length)
          iframe
          ipureintro
          refine ⟨?_, ?_, ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa using Hslots
          · rw [Hadvance.1]; omega
          · simp only [List.flatMap_append, List.flatMap_singleton, List.length_append, Int.natCast_add]
            rw [Hadvance.1, Hposition]
            omega
        | true =>
          have guards := Hguard.mp rfl
          obtain ⟨a, b, cc, d, rest, ha, hb, hc, hd8, shape⟩ := limbWideGuard_decompose _ guards
          have wide := limbWide_valid a b cc d depth ha hb hc hd8 bound
          obtain ⟨itemEq, valueEq, restEq⟩ := feltValid_prefix_unique depth (some item.value) (some (wideFeltValue a b cc d))
            item.bytes (wideFeltBytes a b cc d) (todo.flatMap LimbValueWire.bytes ++ rem) rest item.valid wide shape
          have valEq : item.value = wideFeltValue a b cc d := Option.some.inj valueEq
          have itemLen : item.bytes.length = 37 := by rw [itemEq]; simp [wideFeltBytes, ha, hb, hc, hd8]
          isimp only [shape] at Hbuf
          wp_pures
          have hfast := wp_limbWideStep (GF := GF) buf_ptr felts_ptr itemIndex
            (slice.slice v.buf' w8 pos v.buf'.len) out (done.map LimbValueWire.value ++ previous :: unwritten)
            previous (W64 done.length) a b cc d rest dq ha hb hc hd8 hindex.1 hlookup
          unfold limbWideStepExpr at hfast
          wp_apply hfast $$ [Hbufptr felts Hi Hout Hbuf] with ⟨Hbufptr, Hfelts, Hi, Hout, Hbuf⟩
          · iframe Hbufptr felts Hi Hout Hbuf
          ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
          · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
            rw [Hremaining]
            isimp only [← shape] at Hbuf
            simp only [List.flatMap_cons, List.append_assoc]
            iframe Hpre Hbuf Hpost
          simp only [itemLen, show ((37 : Nat) : Int) = 37 from rfl] at hstep
          have hwindow := limbWindow_advance v.buf' pos (W64 37) (by constructor <;> word)
          isimp only [hwindow] at Hbufptr
          isimp only [← valEq, hset] at Hout
          wp_for_post
          isimp only [hinc] at Hrange
          iframe
          iexists (done ++ [item]), todo, unwritten, (pos + W64 37), lag, (W64 done.length)
          iframe
          ipureintro
          refine ⟨?_, ?_, ⟨?_, ?_⟩, ?_, ?_⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa using Hslots
          · rw [hstep.1]; omega
          · simpa only [itemLen, show ((37 : Nat) : Int) = 37 from rfl] using hstep.2.1
          · simp only [List.flatMap_append, List.flatMap_singleton, List.length_append, Int.natCast_add]
            simp only [itemLen, show ((37 : Nat) : Int) = 37 from rfl]
            rw [hstep.1, Hposition]
            omega
          · simpa only [itemLen, show ((37 : Nat) : Int) = 37 from rfl] using hstep.2.2
  · have hcond : ¬ sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_false, Bool.false_eq_true, ite_false]
    cleanup_bool_decide
    wp_auto
    have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    subst todo
    have hunwritten : unwritten = [] := List.eq_nil_of_length_eq_zero (by simpa using Hslots)
    subst unwritten
    have hdoneEq : items = done := by simpa using Hsplit
    subst done
    have hreset (x : w64) : v.buf'.len - (v.buf'.len - x) = x := by word
    have hfinal : pos = v.pos' + W64 (items.flatMap LimbValueWire.bytes).length := by word
    isimp only [hreset, hfinal] at Hc
    isimp only [List.append_nil] at Hout
    wp_end

/-- The production loop realizes the independent sequence relation for every
    successful encoding, with exact nonwrapping consumption and unread suffix. -/
theorem wp_readLimbs_valid (c : Loc) (out : GoSlice) (old values : List (GoArray w64 4))
    (v : cursor.Cursor) (xs item rest : List w8) (dq : DFrac) (depth count : Nat)
    (size : old.length = count) (valid : LimbItemsValid depth count item values)
    (hd : sint.Z v.depth' = depth) (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       out ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ RET #interface.nil; out ↦* values ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hout, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  obtain ⟨items, length, bytes, vals⟩ := limbItems_witness depth count item values valid
  subst item
  subst values
  wp_apply wp_readLimbs_items c out old v xs rest dq depth items (size.trans length.symm)
    hd bound hp hbytes $$ [Hout Hc Hs] with ⟨Hout, Hc, Hs⟩
  · iframe Hout Hc Hs
  iapply HΦ
  iframe Hout Hc Hs
  ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_items

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_valid
