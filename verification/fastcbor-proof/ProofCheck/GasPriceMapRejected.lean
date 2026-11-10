import ProofCheck.SchemaKeysComplete
import ProofCheck.SupportedSpec
import ProofCheck.GasPriceFailureSound

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

/-- The actual GasPrice loop realizes every pair rejection, preserving the
    specified partial record, rollback position and diagnostic offset. Arbitrary
    key text is conditional on the explicit production validator contract. -/
theorem wp_readGasPrice_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields : Loc) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload body : List w8) (head : w8)
    (kinds : FeltErrorKinds) (qf qo qn dq : DFrac) (d count consumed offset : Nat)
    (final : GasPriceValue) (reason : GasPriceFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : 2 * count ≤ body.length)
    (failure : GasPairsRejected (d + 1) count [] initial body final consumed offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ FeltErrorGlobals kinds ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (out : GasPrice) (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ out ∗ GasPriceChildren out final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       GasPriceFailureError err (v.pos' + W64 (1 + payload.length + offset)) reason (gasPriceFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨#Hglobal, Hglobals, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨#Hsyntax, #Htruncated, #Hkind, #Hdepth⟩, #Htype⟩
  haveI : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
    unfold FeltErrorGlobals SkipErrorGlobals
    infer_instance
  ihave #Hglobals : FeltErrorGlobals kinds $$ []
  · unfold FeltErrorGlobals SkipErrorGlobals
    iframe Hsyntax Htruncated Hkind Hdepth Htype
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hi := input_suffix_head xs (payload ++ body)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ body := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_encoded c fields v xs payload body head (W64 count) dq
    enc major hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) body
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  have bounds := gasPairsRejected_bounds (d + 1) count consumed offset [] initial final body reason failure
  have hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len := by word
  ihave HI : ((∃ (remaining tailConsumed tailOffset : Nat) (seenFields : List Bool) (todo : List w8)
      (current : GasPrice) (value : GasPriceValue) (pos next mask : w64),
      "Herrslot" ∷ err_ptr ↦ interface.nil ∗
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hchildren" ∷ GasPriceChildren current value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hfailure" ∷ ⌜GasPairsRejected (d + 1) remaining seenFields value todo final tailConsumed tailOffset reason⌝ ∗
      "%Hseen" ∷ ⌜GasSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ gasFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + 1 + payload.length + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + 1 + payload.length + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝) ∨
      (∃ (out : GasPrice) (errorLoc : Loc),
        err_ptr ↦ interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∗
        dst ↦ out ∗ GasPriceChildren out final ∗
        c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1} ∗
        GasPriceFailureError errorLoc (v.pos' + W64 (1 + payload.length + offset)) reason (gasPriceFailureKind reason kinds)) : IProp GF)
      $$ [s err Hdst Hchildren Hc]
  · ileft
    iexists count, consumed, offset, [], body, g, initial, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe
    ipureintro
    exact ⟨failure, gasSeen_zero, Nat.le_refl _, ⟨by decide, by simp [gasFieldKeys]⟩,
      ⟨by word, by word⟩, by omega, by omega, hremaining⟩
  wp_for
  icases HI with (Hrun | Hfailed)
  · iNamed Hrun
    wp_auto
    cleanup_bool_decide
    try wp_auto
    cases Hfailure with
    | @key n seenFields initial wire keyReason rejected =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hposfinal : pos = v.pos' + W64 (1 + payload.length + consumed) := by word
      have hoffinal : pos = v.pos' + W64 (1 + payload.length + offset) := by word
      ihave #HerrorKind := keyErrorGlobals_lookup kinds keyReason $$ Hglobals
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextField_rejected_utf8 validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f gasFieldKeys gasFieldNames xs todo qf qo dq
        keyReason (keyFailureKind keyReason kinds) rfl hleft Hsearch gasFieldEncodings_nonempty
        gasFieldHeads_ordered gasCachedShortTable gasInlineFieldTable gasFieldNames_ascii rejected Hpos Hremaining
        $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
      · iframe HerrorKind Hc Hst Hfields Horder Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      isimp only [GasPriceFailureError, gasPriceFailureKind] at HΦ
      iapply HΦ
      iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
      ipureintro
      exact hprogress
    | @duplicate n seenFields initial key tail wei valid duplicate =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hseen : mask &&& (W64 1 <<< gasKeyIndex wei) ≠ W64 0 := by
        intro hz
        exact (Hseen wei).mp hz duplicate
      have hposfinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + consumed) := by word
      have hoffinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + offset) := by word
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextGasKey_duplicate_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key wei kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch valid hseen Hpos Hremaining $$ [Hc Hst Hfields Horder Hnames Hs]
        as %err ⟨%next', %args, Hc, Hst, Hfields, Horder, Hnames, Hs, Herr, Hargs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      iapply HΦ
      iframe Hdst Hchildren Hc Hs Hfields Horder Hnames
      isplitl [Herr Hargs]
      · isimp only [GasPriceFailureError, gasPriceFailureKind]
        iexists args
        iframe Herr Hargs
      ipureintro
      exact hprogress
    | @value n valueOffset seenFields choice initial key tail valueReason valid fresh rejected =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hfresh : GasKeyFresh mask choice := by
        cases choice with
        | none => trivial
        | some wei => exact (Hseen wei).mpr fresh
      have hposfinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + consumed) := by word
      have hoffinal : pos + W64 key.length + W64 valueOffset = v.pos' + W64 (1 + payload.length + offset) := by
        simp only [Int.natCast_add] at Hoffset
        word
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextGasKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key choice kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch valid hfresh Hpos Hremaining $$ [Hc Hst Hfields Horder Hnames Hs]
        with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      ihave #HerrorKind := feltErrorGlobals_lookup kinds valueReason $$ Hglobals
      cases rejected with
      | @amount wei wire off reason rejected =>
        cases wei with
        | false =>
          simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
          wp_auto
          have hread := wp_gasPrice_field_failure (GF := GF) c dst current false
            {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs tail dq (d + 1) valueOffset valueReason (feltFailureKind valueReason kinds)
            rejected (by word) (by omega) hvaluepos Hkey.2.2
          simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hdiagnostic⟩
          · iframe HerrorKind Hdst Hc Hs
          wp_for_post
          isimp only [hposfinal] at Hc
          isimp only [hoffinal] at Herr
          iframe
          iright
          iexists current, err
          isimp only [GasPriceFailureError, gasPriceFailureKind]
          iframe Herrslot Hdst Hchildren Hc Herr
        | true =>
          simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
          wp_auto
          have hread := wp_gasPrice_field_failure (GF := GF) c dst current true
            {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs tail dq (d + 1) valueOffset valueReason (feltFailureKind valueReason kinds)
            rejected (by word) (by omega) hvaluepos Hkey.2.2
          simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr, %Hdiagnostic⟩
          · iframe HerrorKind Hdst Hc Hs
          wp_for_post
          isimp only [hposfinal] at Hc
          isimp only [hoffinal] at Herr
          iframe
          iright
          iexists current, err
          isimp only [GasPriceFailureError, gasPriceFailureKind]
          iframe Herrslot Hdst Hchildren Hc Herr
      | @ignored wire off skipReason rejected =>
        simp only [gasKeyCode]
        wp_auto
        isimp only [feltFailureAddr, feltFailureKind] at HerrorKind
        wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason kinds.toSkipErrorKinds)
          rejected (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe HerrorKind Hc Hs
        wp_for_post
        isimp only [hposfinal] at Hc
        isimp only [hoffinal] at Herr
        iframe
        iright
        iexists current, err
        isimp only [GasPriceFailureError, gasPriceFailureKind, FeltFailureError, feltFailureKind]
        iframe Herrslot Hdst Hchildren Hc Herr
    | @next n tailConsumed tailOffset seenFields choice initial middle target key valueBytes tail reason keyValid fresh valueValid rest =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hleftstep : W64 (n + 1) - W64 1 = W64 n := by word
      have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs =
          (key ++ valueBytes) ++ tail := by
        simpa only [List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs (key ++ valueBytes) tail pos v.buf'.len Hpos Hlen.1 hentry
      have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = key ++ (valueBytes ++ tail) := by
        simpa only [List.append_assoc] using hentry
      have hbodypos : pos + W64 key.length + W64 valueBytes.length = pos + W64 (key ++ valueBytes).length := by
        simp only [List.length_append]; word
      have hfresh : GasKeyFresh mask choice := by
        cases choice with
        | none => trivial
        | some wei => exact (Hseen wei).mpr fresh
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextGasKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs (valueBytes ++ tail)
        key choice kinds.unsupportedKind qf qo qn dq rfl hleft Hsearch keyValid hfresh Hpos hkey
        $$ [Hc Hst Hfields Horder Hnames Hs]
        with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      isimp only [hleftstep] at Hst
      cases valueValid with
      | @amount wei old valueWire feltValue feltValid =>
        cases wei with
        | false =>
          simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
          wp_auto
          have hread := wp_gasPrice_field_valid (GF := GF) c dst current false
            {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs valueBytes tail feltValue dq (d + 1)
            (by word) (by omega) feltValid hvaluepos Hkey.2.2
          simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
          · iframe Hdst Hc Hs
          ihave Hchildren := gasPriceChildren_set current value false p feltValue $$ [Hchildren Hvalue]
          · iframe Hchildren Hvalue
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (gasChoiceSeen seenFields (some false)), tail, (gasPriceSet false current p),
            (gasPriceValueSet false value feltValue), (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask (some false))
          iframe
          ipureintro
          refine ⟨rest, gasSeen_mark mask seenFields (some false) Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
        | true =>
          simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
          wp_auto
          have hread := wp_gasPrice_field_valid (GF := GF) c dst current true
            {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
            xs valueBytes tail feltValue dq (d + 1)
            (by word) (by omega) feltValid hvaluepos Hkey.2.2
          simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
          · iframe Hdst Hc Hs
          ihave Hchildren := gasPriceChildren_set current value true p feltValue $$ [Hchildren Hvalue]
          · iframe Hchildren Hvalue
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (gasChoiceSeen seenFields (some true)), tail, (gasPriceSet true current p),
            (gasPriceValueSet true value feltValue), (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask (some true))
          iframe
          ipureintro
          refine ⟨rest, gasSeen_mark mask seenFields (some true) Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | ignored skipValid =>
        simp only [gasKeyCode]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        ileft
        iexists n, tailConsumed, tailOffset, (gasChoiceSeen seenFields none), tail, current, value,
          (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask none)
        iframe
        ipureintro
        refine ⟨rest, gasSeen_mark mask seenFields none Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
  · icases Hfailed with ⟨%out, %err, Herrslot, Hdst, Hchildren, Hc, Herr⟩
    wp_auto
    cleanup_bool_decide
    try wp_auto
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    exact hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_map_rejected
