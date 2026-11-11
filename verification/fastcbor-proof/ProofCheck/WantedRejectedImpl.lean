import ProofCheck.WantedValidImpl
import ProofCheck.WantedSkipRejected
import ProofCheck.KeyFailureSpec

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def wantedFailureKind (reason : WantedFailureReason) (kinds : FeltErrorKinds) : GoInterface :=
  match reason with
  | .key reason => keyFailureKind reason kinds
  | .skipped reason => skipFailureKind reason kinds.toSkipErrorKinds

def WantedFailureError (err : Loc) (pos : w64) (reason : WantedFailureReason) (kind : GoInterface) : IProp GF :=
  match reason with
  | .key reason => KeyFailureError err pos reason kind
  | .skipped reason => SkipFailureError err pos reason kind

/-- Every rejected independent scan is realized by the actual loop, after
    arbitrary valid ignored prefixes. Failure leaves the current pair and
    container open state intact, with exact cursor and diagnostic positions. -/
theorem wp_nextWanted_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (seen : List Nat) (xs wire : List w8) (qf qn dq : DFrac)
    (d n consumed offset left : Nat) (reason : WantedFailureReason) (kinds : FeltErrorKinds)
    (hfields : s.fields' = fields) (hleft : s.left' = W64 n) (countBound : n < 2^63)
    (small : names.length ≤ 64) (distinct : (names.map (List.map fieldFoldByte)).Nodup)
    (hs : WantedSeen names.length s.seen' seen)
    (hd : sint.Z v.depth' = d) (depth : d ≤ 32)
    (failure : WantedRejected d names seen n wire consumed offset left reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextWanted")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ {v with pos' := v.pos' + W64 consumed} ∗ st ↦ {s with left' := W64 left} ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       WantedFailureError err (v.pos' + W64 offset) reason (wantedFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
         sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
         sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hglobals, Hc, Hst, Hfields, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Htotal := input_consumption xs wire [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have Hbounds := wantedRejected_bounds d n consumed offset left names seen wire reason failure
  have Hprogress : sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
      sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.pos' + W64 offset) = sint.Z v.pos' + offset ∧
      sint.Z (v.pos' + W64 offset) ≤ sint.Z v.buf'.len := by refine ⟨?_, ?_, ?_, ?_⟩ <;> word
  wp_auto
  ihave HI : (∃ (remaining tailConsumed tailOffset : Nat) (todo : List w8) (pos : w64),
      "Hc" ∷ c ↦ {v with pos' := pos} ∗ "Hst" ∷ st ↦ {s with left' := W64 remaining} ∗
      "%Hfailure" ∷ ⌜WantedRejected d names seen remaining todo tailConsumed tailOffset left reason⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ n⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝ : IProp GF) $$ [Hc Hst]
  · iexists n, consumed, offset, wire, v.pos'
    have initial : {s with left' := W64 n} = s := by rw [← hleft]
    rw [initial]
    iframe Hc Hst
    ipureintro; exact ⟨failure, Nat.le_refl _, hp, rfl, rfl, hbytes⟩
  wp_for HI
  cases Hfailure with
  | @key remaining todo keyReason rejected =>
    have hmore : 0 < sint.Z (W64 (remaining + 1)) := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz, hmore, _root_.decide_true, ite_true]
    wp_auto
    ihave #Hkind := keyErrorGlobals_lookup (GF := GF) kinds keyReason $$ Hglobals
    wp_apply wp_Key_rejected validator c {v with pos' := pos} xs todo dq keyReason (keyFailureKind keyReason kinds)
      rejected Hpos Hremaining $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_for_post
    have hfinal : pos = v.pos' + W64 consumed := by word
    have herror : pos = v.pos' + W64 offset := by word
    iapply HΦ
    isimp only [hfinal] at Hc
    isimp only [herror] at Herr
    isimp only [WantedFailureError, wantedFailureKind]
    iframe Hc Hst Hfields Hnames Hs Herr
    ipureintro; exact Hprogress
  | @skipped remaining innerOffset keyBytes wire choice skipReason keyValid ignored rejected =>
    have hmore : 0 < sint.Z (W64 (remaining + 1)) := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    simp only [Int.natCast_add, Int.natCast_one, hz, hmore, _root_.decide_true, ite_true]
    wp_auto
    ihave #Hkind : (skipFailureAddr (GF := GF) skipReason ↦□ skipFailureKind skipReason kinds.toSkipErrorKinds : IProp GF) $$ []
    · iapply skipErrorGlobals_lookup kinds.toSkipErrorKinds skipReason
      isimp only [FeltErrorGlobals] at Hglobals
      icases Hglobals with ⟨#Hskip, _⟩
      iexact Hskip
    obtain ⟨decoded, keyValid, meaning⟩ := keyValid
    cases keyValid with
    | @integer head payload argument enc major bound =>
      wp_apply wp_Key_integer c {v with pos' := pos} xs payload wire head argument dq enc major bound Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      wp_apply wp_Skip_failure c {v with pos' := pos + W64 (1 + payload.length)} xs wire dq d innerOffset skipReason
        (skipFailureKind skipReason kinds.toSkipErrorKinds) rejected hd depth
        ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_for_post
      have hfinal : pos + W64 (1 + payload.length) = v.pos' + W64 consumed := by
        simp only [List.length_append, List.length_cons] at Hconsumed
        dsimp only at Hkey
        word
      have herror : pos + W64 (1 + payload.length) + W64 innerOffset = v.pos' + W64 offset := by
        simp only [List.length_append, List.length_cons] at Hoffset
        dsimp only at Hkey
        word
      iapply HΦ
      isimp only [hfinal] at Hc
      isimp only [herror] at Herr
      isimp only [WantedFailureError, wantedFailureKind]
      iframe Hc Hst Hfields Hnames Hs Herr
      ipureintro; exact Hprogress
    | @text head payload key argument enc major count utf8 =>
      have argEq : argument = W64 key.length := by word
      subst argument
      wp_apply wp_Key_text validator c {v with pos' := pos} xs payload key wire head dq enc major utf8 Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
      wp_auto
      by_cases allSeen : s.seen' = f.all'
      · simp only [allSeen, _root_.decide_true]
        wp_auto
        ihave Hfields := Hrestore $$ Hall
        wp_apply wp_Skip_failure c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs wire dq d innerOffset skipReason
          (skipFailureKind skipReason kinds.toSkipErrorKinds) rejected hd depth
          ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
          $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe Hkind Hc Hs
        wp_for_post
        have hfinal : pos + W64 (1 + payload.length + key.length) = v.pos' + W64 consumed := by
          simp only [List.length_append, List.length_cons] at Hconsumed
          dsimp only at Hkey
          word
        have herror : pos + W64 (1 + payload.length + key.length) + W64 innerOffset = v.pos' + W64 offset := by
          simp only [List.length_append, List.length_cons] at Hoffset
          dsimp only at Hkey
          word
        iapply HΦ
        isimp only [hfinal] at Hc
        isimp only [herror] at Herr
        isimp only [WantedFailureError, wantedFailureKind]
        iframe Hc Hst Hfields Hnames Hs Herr
        ipureintro; exact Hprogress
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
          simpa only [List.append_assoc] using (list_slice3 (head :: payload) key wire (1 + payload.length) (1 + payload.length + key.length)
            (by simp; omega) (by omega) (by omega)).2.1
        simp only [hfields]
        wp_apply wp_fields_match_nested_range fields f names v.buf' xs key pos
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq Hpos hbounds hkey
          $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
        · iframe Hfields Hnames Hs
        cases ok with
        | false =>
          wp_auto
          wp_apply wp_Skip_failure c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs wire dq d innerOffset skipReason
            (skipFailureKind skipReason kinds.toSkipErrorKinds) rejected hd depth
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
            $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
          · iframe Hkind Hc Hs
          wp_for_post
          have hfinal : pos + W64 (1 + payload.length + key.length) = v.pos' + W64 consumed := by
            simp only [List.length_append, List.length_cons] at Hconsumed
            dsimp only at Hkey
            word
          have herror : pos + W64 (1 + payload.length + key.length) + W64 innerOffset = v.pos' + W64 offset := by
            simp only [List.length_append, List.length_cons] at Hoffset
            dsimp only at Hkey
            word
          iapply HΦ
          isimp only [hfinal] at Hc
          isimp only [herror] at Herr
          isimp only [WantedFailureError, wantedFailureKind]
          iframe Hc Hst Hfields Hnames Hs Herr
          ipureintro; exact Hprogress
        | true =>
          have duplicate := wantedIgnored_mask names key choice seen s.seen' field small distinct meaning ignored hs Hmatch
          wp_auto
          wp_apply wp_shiftCount_int
          · ipureintro; exact Hmatch.1
          simp only [duplicate, _root_.decide_false]
          wp_auto
          wp_apply wp_Skip_failure c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs wire dq d innerOffset skipReason
            (skipFailureKind skipReason kinds.toSkipErrorKinds) rejected hd depth
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
            $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
          · iframe Hkind Hc Hs
          wp_for_post
          have hfinal : pos + W64 (1 + payload.length + key.length) = v.pos' + W64 consumed := by
            simp only [List.length_append, List.length_cons] at Hconsumed
            dsimp only at Hkey
            word
          have herror : pos + W64 (1 + payload.length + key.length) + W64 innerOffset = v.pos' + W64 offset := by
            simp only [List.length_append, List.length_cons] at Hoffset
            dsimp only at Hkey
            word
          iapply HΦ
          isimp only [hfinal] at Hc
          isimp only [herror] at Herr
          isimp only [WantedFailureError, wantedFailureKind]
          iframe Hc Hst Hfields Hnames Hs Herr
          ipureintro; exact Hprogress

  | @next remaining tailConsumed tailOffset left keyBytes value tail choice reason keyValid ignored valueValid restFailure =>
    have hmore : 0 < sint.Z (W64 (remaining + 1)) := by word
    have hz : sint.Z (W64 0) = 0 := rfl
    have hdecrement : W64 (remaining + 1) - W64 1 = W64 remaining := by word
    simp only [Int.natCast_add, Int.natCast_one, hz, hmore, _root_.decide_true, ite_true]
    wp_auto
    obtain ⟨decoded, keyValid, meaning⟩ := keyValid
    cases keyValid with
    | @integer head payload argument enc major bound =>
      wp_apply wp_Key_integer c {v with pos' := pos} xs payload (value ++ tail) head argument dq enc major bound Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length)} xs value tail dq d hd depth valueValid
        ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
        $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
      · iframe Hc Hs
      wp_for_post
      isimp only [hdecrement] at Hst
      iframe
      isplitr
      · ipureintro; with_unfolding_all rfl
      iexists remaining, tailConsumed, tailOffset, tail, (pos + W64 (1 + payload.length) + W64 value.length)
      iframe Hc Hst
      ipureintro
      dsimp only at Hkey Hvalue
      refine ⟨restFailure, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, ?_, Hvalue.2.2⟩
      all_goals simp only [List.length_append, List.length_cons] at Hconsumed Hoffset
      all_goals omega
    | @text head payload key argument enc major count utf8 =>
      have argEq : argument = W64 key.length := by word
      subst argument
      wp_apply wp_Key_text validator c {v with pos' := pos} xs payload key (value ++ tail) head dq enc major utf8 Hpos
        (by simpa only [List.append_assoc] using Hremaining) $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
      · iframe Hc Hs
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_all fields f qf).access_strict $$ Hfields with ⟨Hall, Hrestore⟩
      wp_auto
      by_cases allSeen : s.seen' = f.all'
      · simp only [allSeen, _root_.decide_true]
        wp_auto
        ihave Hfields := Hrestore $$ Hall
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value tail dq d hd depth valueValid
          ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hdecrement] at Hst
        iframe
        isplitr
        · ipureintro; with_unfolding_all rfl
        iexists remaining, tailConsumed, tailOffset, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
        iframe Hc Hst
        ipureintro
        dsimp only at Hkey Hvalue
        refine ⟨restFailure, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, ?_, Hvalue.2.2⟩
        all_goals simp only [List.length_append, List.length_cons] at Hconsumed Hoffset
        all_goals omega
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
          simpa only [List.append_assoc] using (list_slice3 (head :: payload) key (value ++ tail) (1 + payload.length) (1 + payload.length + key.length)
            (by simp; omega) (by omega) (by omega)).2.1
        simp only [hfields]
        wp_apply wp_fields_match_nested_range fields f names v.buf' xs key pos
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq Hpos hbounds hkey
          $$ [Hfields Hnames Hs] as %field %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
        · iframe Hfields Hnames Hs
        cases ok with
        | false =>
          wp_auto
          wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value tail dq d hd depth valueValid
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hdecrement] at Hst
          iframe
          isplitr
          · ipureintro; with_unfolding_all rfl
          iexists remaining, tailConsumed, tailOffset, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
          iframe Hc Hst
          ipureintro
          dsimp only at Hkey Hvalue
          refine ⟨restFailure, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, ?_, Hvalue.2.2⟩
          all_goals simp only [List.length_append, List.length_cons] at Hconsumed Hoffset
          all_goals omega
        | true =>
          have duplicate := wantedIgnored_mask names key choice seen s.seen' field small distinct meaning ignored hs Hmatch
          wp_auto
          wp_apply wp_shiftCount_int
          · ipureintro; exact Hmatch.1
          simp only [duplicate, _root_.decide_false]
          wp_auto
          wp_apply wp_Skip_valid c {v with pos' := pos + W64 (1 + payload.length + key.length)} xs value tail dq d hd depth valueValid
            ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ (by simpa only [List.append_assoc] using Hkey.2.2)
            $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
          · iframe Hc Hs
          wp_for_post
          isimp only [hdecrement] at Hst
          iframe
          isplitr
          · ipureintro; with_unfolding_all rfl
          iexists remaining, tailConsumed, tailOffset, tail, (pos + W64 (1 + payload.length + key.length) + W64 value.length)
          iframe Hc Hst
          ipureintro
          dsimp only at Hkey Hvalue
          refine ⟨restFailure, by omega, ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, ?_, Hvalue.2.2⟩
          all_goals simp only [List.length_append, List.length_cons] at Hconsumed Hoffset
          all_goals omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextWanted_rejected
