import ProofCheck.CasmClassMapValid

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

local instance (kinds : SkipErrorKinds) : Persistent (SkipErrorGlobals (GF := GF) kinds) := by
  unfold SkipErrorGlobals
  infer_instance

/-- The diagnostic retains the actual initialized error categories and owned
    arguments. Field-slice diagnostics are not reduced to scalar felt errors. -/
def CasmClassFailureError (err : Loc) (pos : w64) (reason : CasmClassFailureReason) (kinds : FeltErrorKinds) : IProp GF :=
  match reason with
  | .opening reason => FeltFailureError err pos reason (feltFailureKind reason kinds)
  | .key reason => KeyFailureError err pos reason (keyFailureKind reason kinds)
  | .duplicate field => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.unsupportedKind pos (go!"field %s named twice") args ∗
      args ↦* [interface.mkOk go.string #(casmClassFieldName field)])
  | .value (.field reason) => CasmClassFieldFailureOwned err pos reason
  | .value (.ignored reason) => SkipFailureError err pos reason (skipFailureKind reason kinds.toSkipErrorKinds)

def CasmClassFailureOwned (err : Loc) (pos : w64) (reason : CasmClassFailureReason) : IProp GF := iprop(
  ∃ kinds : FeltErrorKinds, FeltErrorGlobals kinds ∗ CasmClassFailureError err pos reason kinds)

/-- Actual generated CasmClass map rejection after any successful prefix. Prior
    fields retain exact partial state, and consumed/diagnostic
    positions stay distinct. The outer map and failed child levels remain on failure. -/
theorem wp_readCasmClass_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : CasmClassValue) (v : cursor.Cursor)
    (xs payload body : List w8) (head : w8) (dq : DFrac) (d count consumed offset extraDepth : Nat)
    (reason : CasmClassFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32) (fit : 2 * count ≤ body.length)
    (failure : CasmClassPairsRejected (d + 1) count [] old body final consumed offset extraDepth reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       CasmClassOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadCasmClass)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       CasmClassOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 (1 + extraDepth)} ∗ v.buf' ↦*{dq} xs ∗
       CasmClassFailureOwned err (v.pos' + W64 (1 + payload.length + offset)) reason ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hschema, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hfit : (count : Int) < 2 ^ 63 := by word
  have Hfit : 2 * uint.Z (W64 count) ≤ body.length := by word
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, Hglobals⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨#Hsyntax, #Htruncated, #Hkind, #Hdepth⟩, #Htype⟩
  ihave #Hglobals : FeltErrorGlobals kinds $$ []
  · unfold FeltErrorGlobals SkipErrorGlobals
    iframe Hsyntax Htruncated Hkind Hdepth Htype
  ihave #Hskip : SkipErrorGlobals kinds.toSkipErrorKinds $$ []
  · unfold SkipErrorGlobals
    iframe Hsyntax Htruncated Hkind Hdepth
  ihave #Htable := class_initialized_table (GF := GF) .casmClass $$ Hpkg
  isimp only [ClassTable, classGlobal, StaticFieldsShared] at Htable
  icases Htable with ⟨%fields, #Hglobal, %f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload body head (W64 count) dq d enc major hd depth Hfit hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  have bounds := casmClassPairsRejected_bounds (d + 1) count consumed offset extraDepth [] old final body reason failure
  have hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len := by word
  ihave HI : ((∃ (remaining tailConsumed tailOffset : Nat) (seenFields : List CasmClassField) (todo : List w8)
      (value : CasmClassValue) (pos next mask : w64),
      "Herrslot" ∷ err_ptr ↦ interface.nil ∗
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Howned" ∷ CasmClassOwned dst value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hfailure" ∷ ⌜CasmClassPairsRejected (d + 1) remaining seenFields value todo final tailConsumed tailOffset extraDepth reason⌝ ∗
      "%Hseen" ∷ ⌜CasmClassSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ casmClassFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + 1 + payload.length + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + 1 + payload.length + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝) ∨
      (∃ (errorLoc : Loc),
        err_ptr ↦ interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∗
        CasmClassOwned dst final ∗
        c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 (1 + extraDepth)} ∗
        CasmClassFailureOwned errorLoc (v.pos' + W64 (1 + payload.length + offset)) reason) : IProp GF)
      $$ [s err Howned Hc]
  · ileft
    iexists count, consumed, offset, [], body, old, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe
    ipureintro
    exact ⟨failure, casmClassSeen_zero, Nat.le_refl _, ⟨by decide, by simp [casmClassFieldKeys, classKeys]⟩,
      ⟨by rw [Hopen.1]; omega, Hopen.2.1⟩, by rw [Hopen.1], by rw [Hopen.1], Hopen.2.2.1⟩
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
      wp_apply wp_nextCasmClassKey_rejected validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs todo DFrac.discard DFrac.discard dq
        keyReason (keyFailureKind keyReason kinds) rfl hleft Hsearch rejected Hpos Hremaining
        $$ [Hc Hst Hs] as %err ⟨Hc, Hst, _, _, Hs, Herr⟩
      · iframe HerrorKind Hc Hst Hfields Horder Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      iapply HΦ
      isimp only [Int.natCast_zero, Int.add_zero]
      iframe Howned Hc Hs
      isplitl [Herr]
      · unfold CasmClassFailureOwned
        iexists kinds
        isimp only [CasmClassFailureError]
        iframe Hglobals Herr
      ipureintro
      exact hprogress
    | @duplicate n seenFields initial key tail wei valid duplicate =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hseen : mask &&& (W64 1 <<< casmClassKeyIndex wei) ≠ W64 0 := by
        intro hz
        exact (Hseen wei).mp hz duplicate
      have hposfinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + consumed) := by word
      have hoffinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + offset) := by word
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextCasmClassKey_duplicate_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key wei kinds.unsupportedKind DFrac.discard DFrac.discard DFrac.discard dq
        rfl hleft Hsearch valid hseen Hpos Hremaining $$ [Hc Hst Hs]
        as %err ⟨%next', %args, Hc, Hst, _, _, _, Hs, Herr, Hargs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      iapply HΦ
      isimp only [Int.natCast_zero, Int.add_zero]
      iframe Howned Hc Hs
      isplitl [Herr Hargs]
      · unfold CasmClassFailureOwned
        iexists kinds
        iframe Hglobals
        isimp only [CasmClassFailureError]
        iexists args
        iframe Herr Hargs
      ipureintro
      exact hprogress
    | @value n seenFields choice initial retained key tail valueConsumed valueOffset valueDepth valueReason valid fresh rejected =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hfresh : CasmClassKeyFresh mask choice := by
        cases choice with
        | none => trivial
        | some field => exact (Hseen field).mpr fresh
      have hposfinal : pos + W64 key.length + W64 valueConsumed = v.pos' + W64 (1 + payload.length + consumed) := by
        simp only [Int.natCast_add] at Hconsumed
        word
      have hoffinal : pos + W64 key.length + W64 valueOffset = v.pos' + W64 (1 + payload.length + offset) := by
        simp only [Int.natCast_add] at Hoffset
        word
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextCasmClassKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key choice kinds.unsupportedKind
        DFrac.discard DFrac.discard DFrac.discard dq rfl hleft Hsearch valid hfresh Hpos Hremaining
        $$ [Hc Hst Hs] with ⟨%next', Hc, Hst, _, _, _, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      cases rejected with
      | @selected field _ _ _ _ _ _ fieldReason fieldRejected =>
        have hread := wp_casmClass_field_rejected (GF := GF) validator c dst value final field
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs tail dq (d + 1) valueConsumed valueOffset extraDepth fieldReason fieldRejected (by word) (by omega) hvaluepos Hkey.2.2
        cases field
        all_goals
          simp only [casmClassKeyCode, casmClassKeyIndex]
          wp_auto
          simp only [casmClassFieldCall] at hread
          wp_apply hread $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hdiagnostic⟩
          · iframe Howned Hc Hs
          wp_for_post
          have hdep : v.depth' + W64 1 + W64 extraDepth = v.depth' + W64 (1 + extraDepth) := by word
          isimp only [hposfinal, hdep] at Hc
          isimp only [hoffinal] at Herr
          iframe
          iright
          iexists err
          iframe Herrslot Howned Hc
          unfold CasmClassFailureOwned
          iexists kinds
          isimp only [CasmClassFailureError]
          iframe Hglobals Herr
      | @ignored _ _ _ skipReason skipRejected =>
        ihave #HerrorKind := skipErrorGlobals_lookup kinds.toSkipErrorKinds skipReason $$ Hskip
        simp only [casmClassKeyCode]
        wp_auto
        wp_apply wp_Skip_failure c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs tail dq (d + 1) valueOffset skipReason (skipFailureKind skipReason kinds.toSkipErrorKinds)
          skipRejected (by word) (by omega) hvaluepos Hkey.2.2 $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
        · iframe HerrorKind Hc Hs
        wp_for_post
        have hz : pos + W64 key.length + W64 0 = pos + W64 key.length := by word
        simp only [Int.natCast_zero, hz] at hposfinal
        isimp only [hposfinal] at Hc
        isimp only [hoffinal] at Herr
        iframe
        iright
        iexists err
        isimp only [Int.natCast_zero, Int.add_zero]
        iframe Herrslot Howned Hc
        unfold CasmClassFailureOwned
        iexists kinds
        isimp only [CasmClassFailureError]
        iframe Hglobals Herr
    | @next n seenFields choice initial middle target key valueBytes tail tailConsumed tailOffset extraDepth reason keyValid fresh valueValid rest =>
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
      have hfresh : CasmClassKeyFresh mask choice := by
        cases choice with
        | none => trivial
        | some wei => exact (Hseen wei).mpr fresh
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextCasmClassKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs (valueBytes ++ tail)
        key choice kinds.unsupportedKind DFrac.discard DFrac.discard DFrac.discard dq rfl hleft Hsearch keyValid hfresh Hpos hkey
        $$ [Hc Hst Hs]
        with ⟨%next', Hc, Hst, _, _, _, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      isimp only [hleftstep] at Hst
      cases valueValid with
      | @selected field _ _ _ _ fieldValid =>
        have hread := wp_casmClass_field_valid (GF := GF) validator c dst value middle field
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail dq (d + 1) fieldValid (by word) (by omega) hvaluepos Hkey.2.2
        cases field
        all_goals
          simp only [casmClassKeyCode, casmClassKeyIndex]
          wp_auto
          simp only [casmClassFieldCall] at hread
          wp_apply hread $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hbody⟩
          · iframe Howned Hc Hs
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (casmClassChoiceSeen seenFields _), tail, middle,
            (pos + W64 (key ++ valueBytes).length), next', (casmClassKeySeen mask _)
          iframe
          ipureintro
          refine ⟨rest, casmClassSeen_mark mask seenFields _ Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | ignored skipValid =>
        simp only [casmClassKeyCode]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
        · iframe Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        ileft
        iexists n, tailConsumed, tailOffset, (casmClassChoiceSeen seenFields none), tail, value,
          (pos + W64 (key ++ valueBytes).length), next', (casmClassKeySeen mask none)
        iframe
        ipureintro
        refine ⟨rest, casmClassSeen_mark mask seenFields none Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega

  · icases Hfailed with ⟨%err, Herrslot, Howned, Hc, Herr⟩
    wp_auto
    cleanup_bool_decide
    try wp_auto
    cleanup_bool_decide
    try wp_auto
    cleanup_bool_decide
    try wp_auto
    simp
    try wp_auto
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro
    simpa using hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readCasmClass_map_rejected
