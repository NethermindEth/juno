import ProofCheck.ReadLimbsValid
import ProofCheck.ReadLimbsFailure

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

/-- The actual loop retains exactly its decoded prefix on failure, preserving
    all unwritten slots and the exact diagnostic after mixed fast/slow steps. -/
theorem wp_readLimbs_prefix_failure (c : Loc) (out : GoSlice) (old : List (GoArray w64 4))
    (v : cursor.Cursor) (xs rem : List w8) (dq : DFrac) (depth offset : Nat)
    (reason : FeltFailureReason) (kind : GoInterface) (failure : LimbsFailure depth rem offset reason)
    (items : List (LimbValueWire depth)) (count : items.length < old.length)
    (hd : sint.Z v.depth' = depth) (bound : depth < 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = items.flatMap LimbValueWire.bytes ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       feltFailureAddr (GF := GF) reason ↦□ kind ∗ out ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"readLimbs")) (Val #out))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       out ↦* (items.map LimbValueWire.value ++ old.drop items.length) ∗
       c ↦ {v with pos' := v.pos' + W64 (items.flatMap LimbValueWire.bytes).length} ∗
       v.buf' ↦*{dq} xs ∗
       FeltFailureError err (v.pos' + W64 (items.flatMap LimbValueWire.bytes).length + W64 offset) reason kind }} := by
  wp_start as ⟨#Hkind, Hout, Hc, Hs⟩
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Houtlen := ownSlice_len _ _ _ $$ Hout
  have houtlen : sint.Z out.len = old.length := by simp only [sint.nat, sint.Z] at Houtlen ⊢; omega
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
      "%Hsplit" ∷ ⌜items = done ++ todo⌝ ∗ "%Htail" ∷ ⌜unwritten = old.drop done.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + (done.flatMap LimbValueWire.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap LimbValueWire.bytes ++ rem⌝ : IProp GF)
      $$ [Hrange Hi Hout Hc Hbufptr]
  · iexists [], items, old, v.pos', v.pos', (W64 0)
    isimp only [List.map_nil, List.nil_append]
    iframe
    ipureintro
    exact ⟨trivial, by simp, hp, by simp, hbytes⟩
  wp_for HI
  have hlength : items.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  have hdone : done.length ≤ items.length := by omega
  have hslots : unwritten.length = old.length - done.length := by rw [Htail, List.length_drop]
  by_cases more : done.length < items.length
  · have hcond : sint.Z (W64 done.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    cases todo with
    | nil => simp only [List.length_nil] at hlength; omega
    | cons item todo =>
      cases unwritten with
      | nil => simp only [List.length_nil] at hslots; omega
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
          · simpa [List.drop_drop, Nat.add_comm] using congrArg (List.drop 1) Htail
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
          · simpa [List.drop_drop, Nat.add_comm] using congrArg (List.drop 1) Htail
          · rw [hstep.1]; omega
          · simpa only [itemLen, show ((37 : Nat) : Int) = 37 from rfl] using hstep.2.1
          · simp only [List.flatMap_append, List.flatMap_singleton, List.length_append, Int.natCast_add]
            simp only [itemLen, show ((37 : Nat) : Int) = 37 from rfl]
            rw [hstep.1, Hposition]
            omega
          · simpa only [itemLen, show ((37 : Nat) : Int) = 37 from rfl] using hstep.2.2
  · have htodo : todo = [] := List.eq_nil_of_length_eq_zero (by omega)
    subst todo
    have hdoneEq : items = done := by simpa using Hsplit
    subst done
    have hcond : sint.Z (W64 items.length) < sint.Z out.len := by word
    simp only [hcond, _root_.decide_true, ite_true]
    wp_auto
    cases unwritten with
    | nil => simp only [List.length_nil] at hslots; omega
    | cons previous unwritten =>
      have hindex : 0 ≤ sint.Z (W64 items.length) ∧ sint.Z (W64 items.length) < sint.Z out.len := by word
      have hidxnat : (sint.Z (W64 items.length)).toNat = items.length := by word
      have hlookup : (items.map LimbValueWire.value ++ previous :: unwritten)[(sint.Z (W64 items.length)).toNat]? = some previous := by
        rw [hidxnat]
        simp [List.getElem?_append]
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_apply +noauto wp_load_slice_index out _ (items.map LimbValueWire.value ++ previous :: unwritten)
        (DFrac.own 1) previous hindex.1 $$ [Hout] with Hout
      · iframe Hout; ipureintro; exact hlookup
      wp_pures
      wp_load
      wp_pures
      wp_store
      wp_pures
      icases (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).1
        $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
      have hrem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = rem := by simpa using Hremaining
      isimp only [hrem] at Hbuf
      have hguard := wp_limbWideGuard (GF := GF) buf_ptr (slice.slice v.buf' w8 pos v.buf'.len) rem dq
      unfold limbWideGuardExpr at hguard
      wp_apply hguard $$ [Hbufptr Hbuf] as %ok ⟨Hbufptr, Hbuf, %Hguard⟩
      · iframe Hbufptr Hbuf
      have hfalse : ok = false := by
        cases ok with
        | false => rfl
        | true => exact False.elim ((limbWideGuard_not_failure depth offset rem reason bound failure) (Hguard.mp rfl))
      subst ok
      wp_auto
      ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
      · iapply (ownSlice_slice pos v.buf'.len v.buf' dq xs ⟨Hpos.1, Hpos.2, Int.le_refl _⟩).2
        rw [hrem]
        iframe Hpre Hbuf Hpost
      have hreset (x : w64) : v.buf'.len - (v.buf'.len - x) = x := by word
      isimp only [hreset] at Hc
      rw [ite_eq_left_of_eq_true _ _ (eq_true hindex)]
      wp_auto
      wp_apply wp_limbs_slice_failure c out (items.map LimbValueWire.value ++ previous :: unwritten)
        (sint.Z (W64 items.length)) previous {v with pos' := pos} xs rem dq depth offset reason kind
        hindex.1 hlookup failure hd (by omega) Hpos hrem $$ [Hout Hc Hs] as %err ⟨Hout, Hc, Hs, Herr⟩
      · iframe Hkind Hout Hc Hs
      wp_for_post
      have hfinal : pos = v.pos' + W64 (items.flatMap LimbValueWire.bytes).length := by word
      isimp only [hfinal] at Hc
      isimp only [hfinal] at Herr
      isimp only [Htail] at Hout
      wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readLimbs_prefix_failure
