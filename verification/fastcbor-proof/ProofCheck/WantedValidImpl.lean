import ProofCheck.WantedSelected
import ProofCheck.WantedSeen
import ProofCheck.SkipValid

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 12800000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def wantedCode : WantedResult → w64
  | .done => W64 (-1)
  | .field index _ => W64 index

def wantedStruct (s : cursor.Struct) : WantedResult → cursor.Struct
  | .done => {s with left' := W64 0, open' := false}
  | .field index left => {s with left' := W64 left, seen' := s.seen' ||| (W64 1 <<< W64 index)}

def wantedCursor (v : cursor.Cursor) (openMap : Bool) (pos : w64) : WantedResult → cursor.Cursor
  | .done => {v with pos' := pos, depth' := if openMap then v.depth' - W64 1 else v.depth'}
  | .field _ _ => {v with pos' := pos}

/-- Every successful independent scan is realized by the actual NextWanted
    loop, including arbitrary ignored/duplicate prefixes and full exhaustion.
    The selected value stays unread; exhaustion alone closes the container. -/
theorem wp_nextWanted_valid (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (seen : List Nat) (xs item rem : List w8) (qf qn dq : DFrac)
    (d n : Nat) (result : WantedResult)
    (hfields : s.fields' = fields) (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (small : names.length ≤ 64) (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (allMask : f.all' = (W64 1 <<< W64 names.length) - W64 1) (hs : WantedSeen names.length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32) (valid : WantedValid d names seen n item result)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ RET (PairV #(wantedCode result) #interface.nil);
       c ↦ wantedCursor v s.open' (v.pos' + W64 item.length) result ∗ st ↦ wantedStruct s result ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hst, Hfields, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Htotal := input_consumption xs item rem v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  ihave HI : (∃ (remaining : Nat) (todo : List w8) (pos : w64),
      "Hc" ∷ c ↦ {v with pos' := pos} ∗ "Hst" ∷ st ↦ {s with left' := W64 remaining} ∗
      "%Hvalid" ∷ ⌜WantedValid d names seen remaining todo result⌝ ∗ "%Hcount" ∷ ⌜remaining ≤ n⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + item.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF) $$ [Hc Hst]
  · iexists n, item, v.pos'
    have initial : {s with left' := W64 n} = s := by rw [← hleft]
    rw [initial]
    iframe Hc Hst
    ipureintro; exact ⟨valid, Nat.le_refl _, hp, rfl, hbytes⟩
  wp_for HI
  cases Hvalid with
  | done =>
    have hfalse : ¬ sint.Z (W64 0) > sint.Z (W64 0) := by omega
    simp only [Int.natCast_zero, hfalse, _root_.decide_false, Bool.false_eq_true, ite_false]
    wp_auto
    have hfinal : pos = v.pos' + W64 item.length := by simp only [List.length_nil] at Hposition; word
    cases openEq : s.open' with
    | false =>
      wp_auto
      isimp only [wantedCode] at HΦ
      iapply HΦ
      isimp only [wantedCode, wantedCursor, wantedStruct, openEq, Bool.false_eq_true, ite_false]
      isimp only [hfinal] at Hc
      iframe Hc Hst Hfields Hnames Hs
      ipureintro; exact Htotal
    | true =>
      wp_auto
      wp_method_call
      wp_call
      wp_auto
      isimp only [wantedCode] at HΦ
      iapply HΦ
      isimp only [wantedCode, wantedCursor, wantedStruct, openEq, ite_true]
      isimp only [hfinal] at Hc
      iframe Hc Hst Hfields Hnames Hs
      ipureintro; exact Htotal
  | @selected remaining index keyBytes choice keyValid selected =>
    have hmore : 0 < sint.Z (W64 (remaining + 1)) := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz, hmore, _root_.decide_true, ite_true]
    wp_auto
    obtain ⟨choiceEq, fresh⟩ := wantedSelect_fresh seen choice index selected
    subst choice
    obtain ⟨decoded, keyValid, meaning⟩ := keyValid
    cases keyValid with
    | integer enc major bound => simp at meaning
    | @text head payload key argument enc major count utf8 =>
      have argEq : argument = W64 key.length := by word
      subst argument
      wp_apply wp_Key_text validator c {v with pos' := pos} xs payload key rem head dq enc major utf8 Hpos Hremaining
        $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      obtain ⟨name, lookup, folded⟩ := meaning
      have inside := (List.getElem?_eq_some_iff.mp lookup).choose
      have incomplete : s.seen' ≠ f.all' := by
        rw [allMask]
        exact wantedSeen_fresh_not_all names.length index s.seen' seen small inside hs fresh
      have freshBit := (hs index inside).mpr fresh
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
      wp_auto
      simp only [incomplete, _root_.decide_false]
      wp_auto
      ihave Hfields := Hrestore $$ Hall
      have htotal : sint.Z pos + 1 + payload.length + key.length ≤ sint.Z v.buf'.len := by dsimp only at Hkey; omega
      have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
          sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length + key.length)) ∧
          sint.Z (W64 (1 + payload.length + key.length)) ≤ sint.Z (v.buf'.len - pos) := by word
      have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
      have hhi : sint.nat (W64 (1 + payload.length + key.length)) = 1 + payload.length + key.length := by word
      have hkey : subslice (sint.nat (W64 (1 + payload.length))) (sint.nat (W64 (1 + payload.length + key.length)))
          (subslice (sint.nat pos) (sint.nat v.buf'.len) xs) = key := by
        rw [Hremaining, hlo, hhi]
        simpa only [List.append_assoc] using (list_slice3 (head :: payload) key rem (1 + payload.length) (1 + payload.length + key.length)
          (by simp; omega) (by omega) (by omega)).2.1
      simp only [hfields]
      wp_apply wp_fields_match_nested_range fields f names v.buf' xs key pos
        (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq Hpos hbounds hkey
        $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
      · iframe Hfields Hnames Hs
      cases ok with
      | false => exact False.elim (Hmatch.2 name (List.mem_of_getElem? lookup) folded)
      | true =>
        have same : field = W64 index := wantedMatch_known names key index field distinct small ⟨name, lookup, folded⟩ Hmatch
        subst field
        have hi : 0 ≤ sint.Z (W64 index) := by word
        wp_auto
        wp_apply wp_shiftCount_int
        · ipureintro; exact hi
        simp only [freshBit]
        wp_auto
        wp_apply wp_shiftCount_int
        · ipureintro; exact hi
        wp_for_post
        have hfinal : pos + W64 (1 + payload.length + key.length) = v.pos' + W64 item.length := by
          simp only [List.length_append, List.length_cons] at Hposition
          dsimp only at Hkey
          word
        have hdecrement : W64 (remaining + 1) - W64 1 = W64 remaining := by word
        isimp only [wantedCode] at HΦ
        iapply HΦ
        isimp only [wantedCode, wantedCursor, wantedStruct]
        isimp only [hfinal] at Hc
        isimp only [hdecrement] at Hst
        isimp only [hfields]
        iframe Hc Hst Hfields Hnames Hs
        ipureintro; exact Htotal
  | @skipped remaining keyBytes value tail choice target keyValid ignored valueValid restValid =>
    have hmore : 0 < sint.Z (W64 (remaining + 1)) := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    have hdecrement : W64 (remaining + 1) - W64 1 = W64 remaining := by word
    simp only [Int.natCast_add, Int.natCast_one, hz, hmore, _root_.decide_true, ite_true]
    wp_auto
    obtain ⟨decoded, keyValid, meaning⟩ := keyValid
    cases keyValid with
    | @integer head payload argument enc major bound =>
      wp_apply wp_Key_integer c {v with pos' := pos} xs payload (value ++ tail ++ rem) head argument dq enc major bound Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length)} xs value (tail ++ rem) dq d hd depth valueValid
        ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
        $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
      · iframe Hc Hs
      wp_for_post
      isimp only [hdecrement] at Hst
      iframe
      isplitr
      · ipureintro; with_unfolding_all rfl
      iexists remaining, tail, (pos + W64 (1 + payload.length) + W64 value.length)
      iframe Hc Hst
      ipureintro
      dsimp only at Hkey Hvalue
      refine ⟨restValid, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, Hvalue.2.2⟩
      simp only [List.length_append, List.length_cons] at Hposition
      omega
    | @text head payload key argument enc major count utf8 =>
      have argEq : argument = W64 key.length := by word
      subst argument
      wp_apply wp_Key_text validator c {v with pos' := pos} xs payload key (value ++ tail ++ rem) head dq enc major utf8 Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
      wp_auto
      by_cases allSeen : s.seen' = f.all'
      · simp only [allSeen, _root_.decide_true]
        wp_auto
        ihave Hfields := Hrestore $$ Hall
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value (tail ++ rem) dq d hd depth valueValid
          ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hdecrement] at Hst
        iframe
        isplitr
        · ipureintro; with_unfolding_all rfl
        iexists remaining, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
        iframe Hc Hst
        ipureintro
        dsimp only at Hkey Hvalue
        refine ⟨restValid, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, Hvalue.2.2⟩
        simp only [List.length_append, List.length_cons] at Hposition
        omega
      · simp only [allSeen, _root_.decide_false]
        wp_auto
        ihave Hfields := Hrestore $$ Hall
        have htotal : sint.Z pos + 1 + payload.length + key.length ≤ sint.Z v.buf'.len := by dsimp only at Hkey; omega
        have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
            sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length + key.length)) ∧
            sint.Z (W64 (1 + payload.length + key.length)) ≤ sint.Z (v.buf'.len - pos) := by word
        have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
        have hhi : sint.nat (W64 (1 + payload.length + key.length)) = 1 + payload.length + key.length := by word
        have hkey : subslice (sint.nat (W64 (1 + payload.length))) (sint.nat (W64 (1 + payload.length + key.length)))
            (subslice (sint.nat pos) (sint.nat v.buf'.len) xs) = key := by
          rw [Hremaining, hlo, hhi]
          simpa only [List.append_assoc] using (list_slice3 (head :: payload) key (value ++ tail ++ rem) (1 + payload.length) (1 + payload.length + key.length)
            (by simp; omega) (by omega) (by omega)).2.1
        simp only [hfields]
        wp_apply wp_fields_match_nested_range fields f names v.buf' xs key pos
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq Hpos hbounds hkey
          $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
        · iframe Hfields Hnames Hs
        cases ok with
        | false =>
          wp_auto
          wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value (tail ++ rem) dq d hd depth valueValid
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hdecrement] at Hst
          iframe
          isplitr
          · ipureintro; with_unfolding_all rfl
          iexists remaining, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
          iframe Hc Hst
          ipureintro
          dsimp only at Hkey Hvalue
          refine ⟨restValid, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, Hvalue.2.2⟩
          simp only [List.length_append, List.length_cons] at Hposition
          omega
        | true =>
          have duplicate := wantedIgnored_mask names key choice seen s.seen' field small distinct meaning ignored hs Hmatch
          wp_auto
          wp_apply wp_shiftCount_int
          · ipureintro; exact Hmatch.1
          simp only [duplicate, _root_.decide_false]
          wp_auto
          wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value (tail ++ rem) dq d hd depth valueValid
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hdecrement] at Hst
          iframe
          isplitr
          · ipureintro; with_unfolding_all rfl
          iexists remaining, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
          iframe Hc Hst
          ipureintro
          dsimp only at Hkey Hvalue
          refine ⟨restValid, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, Hvalue.2.2⟩
          simp only [List.length_append, List.length_cons] at Hposition
          omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_valid
