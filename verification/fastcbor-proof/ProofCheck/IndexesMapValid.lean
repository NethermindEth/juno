import ProofCheck.IndexesOwnership
import ProofCheck.IndexesSeen
import ProofCheck.IntegerKeyComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The actual generated map loop realizes every successful index-record pair
    sequence, including duplicate and unknown keys and arbitrary signed arrays.
    Text keys retain the explicit production UTF-8 validator premise. -/
theorem wp_readIndexes_map_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (h : BlockTransactionsIndexes) (initial : IndexesValue)
    (v : cursor.Cursor) (xs payload body rem : List w8) (head : w8) (dq : DFrac)
    (d count : Nat) (final : IndexesValue)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (valid : IndexesPairsValid (d + 1) count [] initial body final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ IndexesChildren h initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBlockTransactionsIndexes)) (Val #c)) (Val #dst))
    {{ (out : BlockTransactionsIndexes), RET #interface.nil;
       dst ↦ out ∗ IndexesChildren out final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hchildren, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hentries := indexesPairs_count_bound (d + 1) count [] initial final body valid
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hi := input_suffix_head xs (payload ++ body ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro heq; subst head; exact (by decide : W8 246 &&& W8 224 ≠ W8 160) major
  have hu : head ≠ W8 247 := by
    intro heq; subst head; exact (by decide : W8 247 &&& W8 224 ≠ W8 160) major
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload (body ++ rem) head (W64 count) dq
    enc major hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) (body ++ rem) v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead; word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (remaining : Nat) (seenFields : List Bool) (todo : List w8)
      (current : BlockTransactionsIndexes) (value : IndexesValue) (pos mask : w64),
      "Hcount" ∷ count_ptr ↦ W64 remaining ∗ "Hmask" ∷ seen_ptr ↦ mask ∗
      "Hdst" ∷ dst ↦ current ∗ "Hchildren" ∷ IndexesChildren current value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hvalid" ∷ ⌜IndexesPairsValid (d + 1) remaining seenFields value todo final⌝ ∗
      "%Hseen" ∷ ⌜IndexesSeen mask seenFields⌝ ∗ "%Hbound" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + 1 + payload.length + body.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF)
      $$ [count seen Hdst Hchildren Hc]
  · iexists count, [], body, h, initial, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨valid, indexesSeen_zero, Nat.le_refl _, ⟨by word, by word⟩, by omega, hremaining⟩
  wp_for HI
  cases Hvalid with
  | nil =>
    have hfalse : ¬ sint.Z (W64 0) < sint.Z (W64 0) := by omega
    simp only [Int.natCast_zero, hfalse, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + body.length) := by
      simp only [List.length_nil] at Hposition; word
    wp_apply wp_leave c {v with pos' := pos} $$ [Hc] with Hc
    · iframe Hc
    have hrestoredepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    isimp only [hposfinal, hrestoredepth] at Hc
    iapply HΦ
    iframe Hdst Hchildren Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2
  | @step n seenFields decoded initial middle target key valueBytes tail keyValid valueValid rest =>
    have hleft : 0 < sint.Z (W64 (n + 1)) := by word
    have hleftstep : W64 (n + 1) - W64 1 = W64 n := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz, hleft, _root_.decide_true, ite_true]
    wp_auto
    have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = (key ++ valueBytes) ++ (tail ++ rem) := by
      simpa only [List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs (key ++ valueBytes) (tail ++ rem) pos v.buf'.len Hpos Hlen.1 hentry
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = key ++ (valueBytes ++ (tail ++ rem)) := by
      simpa only [List.append_assoc] using hentry
    have hbodypos : pos + W64 key.length + W64 valueBytes.length = pos + W64 (key ++ valueBytes).length := by
      simp only [List.length_append]; word
    wp_apply wp_readIntegerKey_valid validator c {v with pos' := pos, depth' := v.depth' + W64 1}
      xs key (valueBytes ++ (tail ++ rem)) decoded dq keyValid Hpos hkey $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
    · iframe Hc Hs
    dsimp only at Hkey
    have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
      ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
    generalize hchoice : indexesChoice seenFields decoded = choice at valueValid rest
    cases valueValid with
    | @selected old field wire arrayValue arrayValid =>
      obtain ⟨hdecoded, hbit⟩ := indexesChoice_selected mask seenFields decoded field Hseen hchoice
      subst decoded
      have hread := wp_indexesField_valid (GF := GF) c dst current value field
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) arrayValue dq (d + 1) (by word) (by omega) arrayValid hvaluepos Hkey.2.2
      let choice0 := some field
      let store := indexesStore current field
      let newValue := indexesSet value field arrayValue
      cases field <;> simp only [Bool.false_eq_true, ite_false, ite_true, integerKeyValue, Option.getD, Option.isSome]
      all_goals
        simp only [indexesMask, Bool.false_eq_true, ite_false, ite_true] at hbit
        wp_auto
        simp only [hbit, _root_.decide_true]
        wp_auto
        simp only [indexesFieldRef, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hchildren Hc Hs] as %out ⟨Hdst, Hchildren, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hchildren Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        isimp only [hleftstep] at Hcount
        iframe
        iexists n, (indexesSeen seenFields choice0), tail, (store out), newValue,
          (pos + W64 (key ++ valueBytes).length), (indexesMark mask choice0)
        isimp only [choice0, store, newValue, indexesMark, indexesMask, Bool.false_eq_true, ite_false, ite_true]
        iframe
        ipureintro
        refine ⟨rest, indexesSeen_mark mask seenFields choice0 Hseen, by omega,
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | ignored skipValid =>
      have hdispatch := indexesChoice_mask mask seenFields decoded Hseen
      rw [hchoice] at hdispatch
      cases decoded with
      | none =>
        simp only [integerKeyValue, Option.getD, Option.isSome]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        isimp only [hleftstep] at Hcount
        iframe
        iexists n, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
        iframe
        ipureintro
        refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
      | some number =>
        simp only [integerKeyValue, Option.getD, Option.isSome]
        wp_auto
        by_cases hone : number = W64 1
        · subst number
          have hbit : mask &&& W64 1 ≠ W64 0 := by
            intro hz; simp [hz] at hdispatch; exact hdispatch hz
          simp only [_root_.decide_true]
          wp_auto
          simp only [hbit, _root_.decide_false]
          wp_auto
          wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hbodypos] at Hc
          isimp only [hleftstep] at Hcount
          iframe
          iexists n, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
          iframe
          ipureintro
          refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
          omega
        · simp only [hone, _root_.decide_false]
          wp_auto
          by_cases htwo : number = W64 2
          · subst number
            have hbit : mask &&& W64 2 ≠ W64 0 := by
              intro hz; simp [hz] at hdispatch; exact hdispatch hz
            simp only [_root_.decide_true]
            wp_auto
            simp only [hbit, _root_.decide_false]
            wp_auto
            wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
              xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
              $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
            · iframe Hc Hs
            wp_for_post
            isimp only [hbodypos] at Hc
            isimp only [hleftstep] at Hcount
            iframe
            iexists n, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
            iframe
            ipureintro
            refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
            simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
            omega
          · simp only [htwo, _root_.decide_false]
            wp_auto
            wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
              xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
              $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
            · iframe Hc Hs
            wp_for_post
            isimp only [hbodypos] at Hc
            isimp only [hleftstep] at Hcount
            iframe
            iexists n, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
            iframe
            ipureintro
            refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
            simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
            omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIndexes_map_valid
