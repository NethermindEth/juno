import ProofCheck.IndexesOwnership
import ProofCheck.IndexesSeen
import ProofCheck.IndexesError

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

/-- Every specified map failure follows the actual generated loop after any
    valid prefix, retaining exact partial output, cursor/depth and error ownership.
    Unknown text keys retain the explicit UTF-8 validator premise. -/
theorem wp_readIndexes_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (h : BlockTransactionsIndexes) (initial : IndexesValue)
    (v : cursor.Cursor) (xs payload body : List w8) (head : w8) (dq : DFrac)
    (d count consumed offset extraDepth : Nat) (final : IndexesValue) (reason : IndexesRecordFailureReason)
    (k : BigIntErrorKinds) (depthKind : Loc)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : 2 * count ≤ body.length)
    (failure : IndexesPairsRejected (d + 1) count [] initial body final consumed offset extraDepth reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals k ∗ globalAddr cursor.ErrDepth ↦□ bigIntCategory depthKind ∗
       dst ↦ h ∗ IndexesChildren h initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBlockTransactionsIndexes)) (Val #c)) (Val #dst))
    {{ (result : GoInterface) (out : BlockTransactionsIndexes), RET #result;
       dst ↦ out ∗ IndexesChildren out final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1 + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ IndexesRecordError result (v.pos' + W64 (1 + payload.length + offset)) reason k depthKind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32⌝ }} := by
  have hz (x : w64) : x + W64 0 = x := by word
  wp_start as ⟨#Hglobals, #Hdepth, Hdst, Hchildren, Hc, Hs⟩
  haveI : Persistent (SkipErrorGlobals (GF := GF) (indexesSkipKinds k depthKind)) := by
    unfold SkipErrorGlobals
    infer_instance
  ihave #Hskip : (SkipErrorGlobals (indexesSkipKinds k depthKind) : IProp GF) $$ []
  · iapply indexesSkip_globals k depthKind
    iframe Hglobals Hdepth
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hi := input_suffix_head xs (payload ++ body) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro heq; subst head; exact (by decide : W8 246 &&& W8 224 ≠ W8 160) major
  have hu : head ≠ W8 247 := by
    intro heq; subst head; exact (by decide : W8 247 &&& W8 224 ≠ W8 160) major
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ body := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload body head (W64 count) dq
    enc major hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) body v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead; word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  have bounds := indexesPairsRejected_bounds (d + 1) count consumed offset extraDepth [] initial final body reason (by omega) failure
  have hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32 := by
    exact ⟨by word, by word, by word, by word, bounds.2.2⟩
  ihave HI : (∃ (remaining tailConsumed tailOffset : Nat) (seenFields : List Bool) (todo : List w8)
      (current : BlockTransactionsIndexes) (value : IndexesValue) (pos mask : w64),
      "Hcount" ∷ count_ptr ↦ W64 remaining ∗ "Hmask" ∷ seen_ptr ↦ mask ∗
      "Hdst" ∷ dst ↦ current ∗ "Hchildren" ∷ IndexesChildren current value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hfailure" ∷ ⌜IndexesPairsRejected (d + 1) remaining seenFields value todo final tailConsumed tailOffset extraDepth reason⌝ ∗
      "%Hseen" ∷ ⌜IndexesSeen mask seenFields⌝ ∗ "%Hbound" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + 1 + payload.length + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + 1 + payload.length + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝ : IProp GF)
      $$ [count seen Hdst Hchildren Hc]
  · iexists count, consumed, offset, [], body, h, initial, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨failure, indexesSeen_zero, Nat.le_refl _, ⟨by word, by word⟩, by omega, by omega, hremaining⟩
  wp_for HI
  cases Hfailure with
  | @key n keyConsumed seenFields initial wire keyReason rejected =>
    have hleft : 0 < sint.Z (W64 (n + 1)) := by word
    have hz0 : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz0, hleft, _root_.decide_true, ite_true]
    wp_auto
    have hposfinal : pos + W64 tailConsumed = v.pos' + W64 (1 + payload.length + consumed) := by word
    have hoffinal : pos = v.pos' + W64 (1 + payload.length + offset) := by word
    wp_apply wp_readIntegerKey_rejected validator c {v with pos' := pos, depth' := v.depth' + W64 1}
      xs todo dq tailConsumed keyReason k rejected Hpos Hremaining $$ [Hc Hs] as %result ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    ihave %Hshape := integerKeyError_shape result pos keyReason k $$ Herr
    rcases Hshape with heq | ⟨err, heq⟩
    all_goals
      subst result
      try simp only [bigIntCategory]
      wp_auto
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      iapply HΦ
      isimp only [IndexesRecordError, Int.natCast_zero, hz]
      iframe Hdst Hchildren Hc Hs Herr
      ipureintro
      exact hprogress
  | @value n valueConsumed valueOffset valueDepth seenFields decoded initial key tail valueReason keyValid rejected =>
    have hleft : 0 < sint.Z (W64 (n + 1)) := by word
    have hz0 : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz0, hleft, _root_.decide_true, ite_true]
    wp_auto
    have hposfinal : pos + W64 key.length + W64 valueConsumed = v.pos' + W64 (1 + payload.length + consumed) := by
      simp only [Int.natCast_add] at Hconsumed; word
    have hoffinal : pos + W64 key.length + W64 valueOffset = v.pos' + W64 (1 + payload.length + offset) := by
      simp only [Int.natCast_add] at Hoffset; word
    wp_apply wp_readIntegerKey_valid validator c {v with pos' := pos, depth' := v.depth' + W64 1}
      xs key tail decoded dq keyValid Hpos Hremaining $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
    · iframe Hc Hs
    dsimp only at Hkey
    have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
      ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
    generalize hchoice : indexesChoice seenFields decoded = choice at rejected
    cases rejected with
    | @selected field wire valueConsumed valueOffset entered arrayReason arrayFailure =>
      obtain ⟨hdecoded, hbit⟩ := indexesChoice_selected mask seenFields decoded field Hseen hchoice
      subst decoded
      have hread := wp_indexesField_failure (GF := GF) c dst current final field
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs tail dq (d + 1) valueConsumed valueOffset entered arrayReason k depthKind arrayFailure
        (by word) (by omega) hvaluepos Hkey.2.2
      cases field <;> simp only [Bool.false_eq_true, ite_false, ite_true, integerKeyValue, Option.getD, Option.isSome]
      all_goals
        simp only [indexesMask, Bool.false_eq_true, ite_false, ite_true] at hbit
        wp_auto
        simp only [hbit, _root_.decide_true]
        wp_auto
        simp only [indexesFieldRef, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hchildren Hc Hs] as %result ⟨Hdst, Hchildren, Hc, Hs, Herr⟩
        · iframe Hglobals Hdepth Hdst Hchildren Hc Hs
        ihave %Hshape := intsFailureError_shape result (pos + W64 key.length + W64 valueOffset) arrayReason k depthKind $$ Herr
        rcases Hshape with heq | ⟨err, heq⟩
        all_goals
          subst result
          try simp only [bigIntCategory]
          wp_auto
          wp_for_post
          isimp only [hposfinal] at Hc
          isimp only [hoffinal] at Herr
          iapply HΦ
          isimp only [IndexesRecordError]
          iframe Hdst Hchildren Hc Hs Herr
          ipureintro
          exact hprogress
    | @ignored wire valueOffset skipReason skipFailure =>
      ihave #Hkind := skipErrorGlobals_lookup (indexesSkipKinds k depthKind) skipReason $$ Hskip
      have hdispatch := indexesChoice_mask mask seenFields decoded Hseen
      rw [hchoice] at hdispatch
      cases decoded with
      | none =>
        simp only [integerKeyValue, Option.getD, Option.isSome]
        wp_auto
        wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason (indexesSkipKinds k depthKind))
          skipFailure (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe Hkind Hc Hs
        wp_for_post
        simp only [Int.natCast_zero, hz] at hposfinal
        isimp only [hposfinal] at Hc
        isimp only [hoffinal] at Herr
        iapply HΦ
        isimp only [IndexesRecordError, Int.natCast_zero, hz]
        iframe Hdst Hchildren Hc Hs
        isplitl [Herr]
        · iexists err
          iframe Herr
          ipureintro; rfl
        ipureintro
        exact hprogress
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
          wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason (indexesSkipKinds k depthKind))
            skipFailure (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
          · iframe Hkind Hc Hs
          wp_for_post
          simp only [Int.natCast_zero, hz] at hposfinal
          isimp only [hposfinal] at Hc
          isimp only [hoffinal] at Herr
          iapply HΦ
          isimp only [IndexesRecordError, Int.natCast_zero, hz]
          iframe Hdst Hchildren Hc Hs
          isplitl [Herr]
          · iexists err
            iframe Herr
            ipureintro; rfl
          ipureintro
          exact hprogress
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
            wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
              xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason (indexesSkipKinds k depthKind))
              skipFailure (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
            · iframe Hkind Hc Hs
            wp_for_post
            simp only [Int.natCast_zero, hz] at hposfinal
            isimp only [hposfinal] at Hc
            isimp only [hoffinal] at Herr
            iapply HΦ
            isimp only [IndexesRecordError, Int.natCast_zero, hz]
            iframe Hdst Hchildren Hc Hs
            isplitl [Herr]
            · iexists err
              iframe Herr
              ipureintro; rfl
            ipureintro
            exact hprogress
          · simp only [htwo, _root_.decide_false]
            wp_auto
            wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
              xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason (indexesSkipKinds k depthKind))
              skipFailure (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
            · iframe Hkind Hc Hs
            wp_for_post
            simp only [Int.natCast_zero, hz] at hposfinal
            isimp only [hposfinal] at Hc
            isimp only [hoffinal] at Herr
            iapply HΦ
            isimp only [IndexesRecordError, Int.natCast_zero, hz]
            iframe Hdst Hchildren Hc Hs
            isplitl [Herr]
            · iexists err
              iframe Herr
              ipureintro; rfl
            ipureintro
            exact hprogress
  | @next n childConsumed childOffset childDepth seenFields decoded initial middle target key valueBytes tail nextReason keyValid valueValid rest =>
    have hleft : 0 < sint.Z (W64 (n + 1)) := by word
    have hleftstep : W64 (n + 1) - W64 1 = W64 n := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz, hleft, _root_.decide_true, ite_true]
    wp_auto
    have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = (key ++ valueBytes) ++ tail := by
      simpa only [List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs (key ++ valueBytes) tail pos v.buf'.len Hpos Hlen.1 hentry
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = key ++ (valueBytes ++ tail) := by
      simpa only [List.append_assoc] using hentry
    have hbodypos : pos + W64 key.length + W64 valueBytes.length = pos + W64 (key ++ valueBytes).length := by
      simp only [List.length_append]; word
    wp_apply wp_readIntegerKey_valid validator c {v with pos' := pos, depth' := v.depth' + W64 1}
      xs key (valueBytes ++ tail) decoded dq keyValid Hpos hkey $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
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
        xs valueBytes tail arrayValue dq (d + 1) (by word) (by omega) arrayValid hvaluepos Hkey.2.2
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
        iexists n, childConsumed, childOffset, (indexesSeen seenFields choice0), tail, (store out), newValue,
          (pos + W64 (key ++ valueBytes).length), (indexesMark mask choice0)
        isimp only [choice0, store, newValue, indexesMark, indexesMask, Bool.false_eq_true, ite_false, ite_true]
        iframe
        ipureintro
        refine ⟨rest, indexesSeen_mark mask seenFields choice0 Hseen, by omega,
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢
        all_goals omega
    | ignored skipValid =>
      have hdispatch := indexesChoice_mask mask seenFields decoded Hseen
      rw [hchoice] at hdispatch
      cases decoded with
      | none =>
        simp only [integerKeyValue, Option.getD, Option.isSome]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        isimp only [hleftstep] at Hcount
        iframe
        iexists n, childConsumed, childOffset, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
        iframe
        ipureintro
        refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢
        all_goals omega
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
            xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hbodypos] at Hc
          isimp only [hleftstep] at Hcount
          iframe
          iexists n, childConsumed, childOffset, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
          iframe
          ipureintro
          refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢
          all_goals omega
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
              xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
              $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
            · iframe Hc Hs
            wp_for_post
            isimp only [hbodypos] at Hc
            isimp only [hleftstep] at Hcount
            iframe
            iexists n, childConsumed, childOffset, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
            iframe
            ipureintro
            refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
            all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢
            all_goals omega
          · simp only [htwo, _root_.decide_false]
            wp_auto
            wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
              xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
              $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
            · iframe Hc Hs
            wp_for_post
            isimp only [hbodypos] at Hc
            isimp only [hleftstep] at Hcount
            iframe
            iexists n, childConsumed, childOffset, seenFields, tail, current, value, (pos + W64 (key ++ valueBytes).length), mask
            iframe
            ipureintro
            refine ⟨rest, Hseen, by omega, ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
            all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢
            all_goals omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIndexes_map_rejected
