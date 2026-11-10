import ProofCheck.HeaderMapComplete
import ProofCheck.HeaderStringRejected
import ProofCheck.HeaderRecordError
import ProofCheck.HeaderMapValueFailure

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

/-- Every complete Header pair rejection is realized by the generated loop.
    Earlier fields remain owned, failed replacements remain unpublished, and
    consumption, diagnostic offset and retained child depth are exact. The
    production UTF-8 validator contract remains an explicit premise. -/
theorem wp_readHeader_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields gasFields : Loc) (v : cursor.Cursor) (f gasF : cursor.Fields)
    (xs payload body : List w8) (head : w8)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) (d count consumed offset extraDepth : Nat)
    (final : HeaderValue) (reason : HeaderRecordFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : 2 * count ≤ body.length)
    (failure : HeaderPairsRejected (d + 1) count [] headerZeroValue body final consumed offset extraDepth reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1 + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderRecordFailureError err (v.pos' + W64 (1 + payload.length + offset)) reason (headerRecordFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32⌝ }} := by
  have hz (x : w64) : x + W64 0 = x := by word
  wp_start as ⟨#Hglobal, #HgasGlobal, Hglobals, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
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
  have bounds := headerPairsRejected_bounds (d + 1) count consumed offset extraDepth [] headerZeroValue final body reason (by omega) failure
  have hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32 := by
    exact ⟨by word, by word, by word, by word, bounds.2.2⟩
  ihave Hchildren := headerChildren_zero (GF := GF)
  isimp only [HeaderChildren] at Hchildren
  icases Hchildren with ⟨Hbody, -, -⟩
  ihave Hready := headerGasReady_zero (GF := GF) (W64 0)
  ihave HI : ((∃ (remaining tailConsumed tailOffset : Nat) (seenFields : List HeaderField) (todo : List w8)
      (current : Header) (value : HeaderValue) (base : Loc) (pos next mask : w64),
      "Herrslot" ∷ err_ptr ↦ interface.nil ∗
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hbody" ∷ HeaderBody current value ∗
      "Hprices" ∷ prices_ptr ↦ base ∗ "Hready" ∷ HeaderGasReady base current value mask ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hfailure" ∷ ⌜HeaderPairsRejected (d + 1) remaining seenFields value todo final tailConsumed tailOffset extraDepth reason⌝ ∗
      "%Hseen" ∷ ⌜HeaderSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + 1 + payload.length + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + 1 + payload.length + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝) ∨
      (∃ (errorLoc : Loc),
        err_ptr ↦ interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∗
        HeaderOwned dst final ∗
        c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1 + W64 extraDepth} ∗
        HeaderRecordFailureError errorLoc (v.pos' + W64 (1 + payload.length + offset)) reason (headerRecordFailureKind reason kinds)) : IProp GF)
      $$ [s err prices Hdst Hbody Hready Hc]
  · ileft
    iexists count, consumed, offset, [], body, (zero_val Header), headerZeroValue, null, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe Hbody Hready
    iframe
    ipureintro
    exact ⟨failure, headerSeen_zero, Nat.le_refl _, ⟨by decide, by simp [headerFieldKeys]⟩,
      ⟨by word, by word⟩, by omega, by omega, hremaining⟩
  wp_for
  icases HI with (Hrun | Hfailed)
  · iNamed Hrun
    wp_auto
    cleanup_bool_decide
    try wp_auto
    cleanup_bool_decide
    try wp_auto
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
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f headerFieldKeys headerFieldNames xs todo qf qo dq
        keyReason (keyFailureKind keyReason kinds) rfl hleft Hsearch headerFieldEncodings_nonempty
        headerFieldHeads_ordered headerCachedShortTable headerInlineFieldTable headerFieldNames_ascii rejected Hpos Hremaining
        $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
      · iframe HerrorKind Hc Hst Hfields Horder Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      ihave Hstate := headerGasReady_forget base current final mask $$ Hready
      ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
      · iframe Hbody Hstate
      isimp only [HeaderRecordFailureError, headerRecordFailureKind, Int.natCast_zero, hz] at HΦ
      iapply HΦ
      iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
      isplitl [Hdst Hchildren]
      · unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      ipureintro; exact hprogress
    | @duplicate n seenFields initial key tail field valid duplicate =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hseen : mask &&& (W64 1 <<< headerKeyIndex field) ≠ W64 0 := by
        intro hz
        exact (Hseen field).mp hz duplicate
      have hposfinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + consumed) := by word
      have hoffinal : pos + W64 key.length = v.pos' + W64 (1 + payload.length + offset) := by word
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextHeaderKey_duplicate_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key field kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch valid hseen Hpos Hremaining $$ [Hc Hst Hfields Horder Hnames Hs]
        as %err ⟨%next', %args, Hc, Hst, Hfields, Horder, Hnames, Hs, Herr, Hargs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      wp_for_post
      isimp only [hposfinal] at Hc
      isimp only [hoffinal] at Herr
      ihave Hstate := headerGasReady_forget base current final mask $$ Hready
      ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
      · iframe Hbody Hstate
      isimp only [Int.natCast_zero, hz] at HΦ
      iapply HΦ
      iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
      isplitl [Hdst Hchildren]
      · unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      isplitl [Herr Hargs]
      · isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderLookupFailureError]
        iexists args
        iframe Herr Hargs
      ipureintro
      exact hprogress
    | @value n valueConsumed valueOffset valueDepth seenFields choice initial key tail valueReason valid fresh rejected =>
      have hleft : 0 < sint.Z (W64 (n + 1)) := by word
      have hfresh : HeaderKeyFresh mask choice := by
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
      wp_apply wp_nextHeaderKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs tail key choice kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch valid hfresh Hpos Hremaining $$ [Hc Hst Hfields Horder Hnames Hs]
        with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      cases rejected with
      | @felt field wire childOffset reason childFailure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds reason $$ Hglobals
        have hread := wp_headerFelt_failure (GF := GF) c dst current final field
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} xs tail dq (d + 1) valueOffset reason (feltFailureKind reason kinds)
          childFailure (by word) (by omega) hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerFeltKey]
        all_goals
          wp_auto
          simp only [headerFeltName] at hread
          wp_apply hread $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr, %Hoffset⟩
          · iframe HerrorKind Hdst Hbody Hc Hs
          wp_for_post
          try simp only [Int.natCast_zero, hz] at hposfinal
          isimp only [hposfinal] at Hc
          try simp only [Int.natCast_zero, hz] at hoffinal
          isimp only [hoffinal] at Herr
          ihave Hstate := headerGasReady_forget base current final mask $$ Hready
          ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @scalar field wire reason childFailure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds (.expected (W8 0) reason) $$ Hglobals
        isimp only [feltFailureAddr, feltFailureKind] at HerrorKind
        have hread := wp_headerScalar_failure (GF := GF) c dst current final field
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} xs tail dq reason (expectedFailureKind reason kinds) childFailure hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerScalarKey]
        all_goals
          wp_auto
          simp only [headerScalarName, headerScalarMode, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr⟩
          · iframe HerrorKind Hdst Hbody Hc Hs
          wp_for_post
          try simp only [Int.natCast_zero, hz] at hposfinal
          isimp only [hposfinal] at Hc
          try simp only [Int.natCast_zero, hz] at hoffinal
          isimp only [hoffinal] at Herr
          ihave Hstate := headerGasReady_forget base current final mask $$ Hready
          ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, feltFailureKind, FeltFailureError, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @signatures wire childConsumed childOffset childDepth reason childFailure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds reason $$ Hglobals
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerSignatures_failure c dst current final {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} xs tail dq (d + 1) valueConsumed valueOffset extraDepth
          reason (feltFailureKind reason kinds) childFailure (by word) (by omega) hvaluepos Hkey.2.2
          $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr, %Hoffset⟩
        · iframe HerrorKind Hdst Hbody Hc Hs
        wp_for_post
        try simp only [Int.natCast_zero, hz] at hposfinal
        isimp only [hposfinal] at Hc
        try simp only [Int.natCast_zero, hz] at hoffinal
        isimp only [hoffinal] at Herr
        ihave Hstate := headerGasReady_forget base current final mask $$ Hready
        ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      | @string wire reason childFailure =>
        ihave #HerrorKind := stringErrorGlobals_lookup kinds reason $$ Hglobals
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerString_rejected validator c dst base current final mask {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} xs tail dq reason
          (stringFailureKind reason kinds) childFailure hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
          as %err ⟨Hdst, Hbody, Hready, Hc, Hs, Herr⟩
        · iframe HerrorKind Hdst Hbody Hready Hc Hs
        wp_for_post
        try simp only [Int.natCast_zero, hz] at hposfinal
        isimp only [hposfinal] at Hc
        try simp only [Int.natCast_zero, hz] at hoffinal
        isimp only [hoffinal] at Herr
        ihave Hstate := headerGasReady_forget base current final mask $$ Hready
        ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      | @gas second retained wire childConsumed childOffset entered reason childFailure =>
        let marked := headerKeySeen mask (some (headerGasKey second))
        have hread := wp_headerGas_fresh_rejected (GF := GF) validator c dst prices_ptr base gasFields current final second retained mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} gasF xs tail kinds qgf qgo qgn dq (d + 1) valueConsumed valueOffset entered reason
          hfresh childFailure (by word) (by omega) hvaluepos Hkey.2.2
        cases second <;> simp only [headerKeyCode, headerKeyIndex, headerGasKey, Bool.false_eq_true, ite_false, ite_true]
        all_goals
          wp_auto
          simp only [headerGasName, headerGasSlot, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames]
            as %err %newBase ⟨Hdst, Hbody, Hprices, Hready, Hc, Hs, HgasFields, HgasOrder, HgasNames, Herr, %Hoffset⟩
          · iframe HgasGlobal Hglobals Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames
          have hdepthStep : (if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1) =
              v.depth' + W64 1 + W64 (if entered then 1 else 0 : Nat) := by cases entered <;> simp
          isimp only [hdepthStep] at Hc
          wp_for_post
          try simp only [Int.natCast_zero, hz] at hposfinal
          isimp only [hposfinal] at Hc
          try simp only [Int.natCast_zero, hz] at hoffinal
          isimp only [hoffinal] at Herr
          ihave Hstate := headerGasReady_forget newBase current final marked $$ Hready
          ihave Hchildren := headerGasState_publish newBase current final $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @ignored wire childOffset reason childFailure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds (.skipped reason) $$ Hglobals
        isimp only [feltFailureAddr, feltFailureKind] at HerrorKind
        simp only [headerKeyCode]
        wp_auto
        wp_apply wp_Skip_failure_bounded c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1} xs tail dq (d + 1) valueOffset reason
          (skipFailureKind reason kinds.toSkipErrorKinds) childFailure (by word) (by omega) hvaluepos Hkey.2.2
          $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoffset⟩
        · iframe HerrorKind Hc Hs
        wp_for_post
        try simp only [Int.natCast_zero, hz] at hposfinal
        isimp only [hposfinal] at Hc
        try simp only [Int.natCast_zero, hz] at hoffinal
        isimp only [hoffinal] at Herr
        ihave Hstate := headerGasReady_forget base current final mask $$ Hready
        ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderRecordFailureError, headerRecordFailureKind, HeaderValueFailureError, headerValueFailureKind, feltFailureKind, FeltFailureError, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
    | @next n tailConsumed tailOffset childDepth seenFields choice initial middle target key valueBytes tail reason keyValid fresh valueValid rest =>
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
      have hfresh : HeaderKeyFresh mask choice := by
        cases choice with
        | none => trivial
        | some field => exact (Hseen field).mpr fresh
      isimp only [Int.natCast_add, Int.natCast_one] at Hst
      wp_apply wp_nextHeaderKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
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
      | @felt field old wire feltValue feltValid =>
        let choice0 := some (headerFeltKey field)
        let out := headerSetFeltPointer field current
        let middleValue := headerSetFeltValue field value feltValue
        have hread := wp_headerFelt_success (GF := GF) c dst base current value field mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail feltValue dq (d + 1) (by word) (by omega) feltValid hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerFeltKey]
        all_goals
          wp_auto
          simp only [headerFeltName] at hread
          wp_apply hread $$ [Hdst Hbody Hready Hc Hs] as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
          · iframe Hdst Hbody Hready Hc Hs
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields choice0), tail, (out p), middleValue, base,
            (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
          iframe
          ipureintro
          refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | @scalar field old wire scalarValue scalarValid =>
        let choice0 := some (headerScalarKey field)
        let out := headerSetScalarWord field current scalarValue
        let middleValue := headerSetScalarValue field value scalarValue
        have hread := wp_headerScalar_success (GF := GF) c dst base current value field mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail scalarValue dq scalarValid hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerScalarKey]
        all_goals
          wp_auto
          simp only [headerScalarName, headerScalarMode, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hready Hc Hs] with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
          · iframe Hdst Hbody Hready Hc Hs
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields choice0), tail, out, middleValue, base,
            (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
          iframe
          ipureintro
          refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | @signatures old wire sigValue sigValid =>
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerSignatures_success c dst base current value mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail sigValue dq (d + 1) (by word) (by omega) sigValid hvaluepos Hkey.2.2
          $$ [Hdst Hbody Hready Hc Hs] as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        ileft
        iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields (some .signatures)), tail, {current with Signatures' := p}, {value with signatures := sigValue}, base,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask (some .signatures))
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields (some .signatures) Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | @string old wire textValue textValid =>
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerString_valid validator c dst base current value mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail textValue dq textValid hvaluepos Hkey.2.2
          $$ [Hdst Hbody Hready Hc Hs] with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        ileft
        iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields (some .protocolVersion)), tail, {current with ProtocolVersion' := textValue}, {value with protocolVersion := textValue}, base,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask (some .protocolVersion))
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields (some .protocolVersion) Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
        all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | @gas second old wire gasValue gasValid =>
        let choice0 := some (headerGasKey second)
        let middleValue := headerSetGasValue second value gasValue
        have hread := wp_headerGas_valid (GF := GF) validator c dst prices_ptr base gasFields current value second mask
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          gasF xs valueBytes tail gasValue kinds.unsupportedKind qgf qgo qgn dq (d + 1)
          hfresh gasValid (by word) hvaluepos Hkey.2.2
        cases second <;> simp only [headerKeyCode, headerKeyIndex, headerGasKey, Bool.false_eq_true, ite_false, ite_true]
        all_goals
          wp_auto
          simp only [headerGasName, headerGasSlot, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames]
            as %out %newBase ⟨Hdst, Hbody, Hprices, Hready, Hc, Hs, HgasFields, HgasOrder, HgasNames, %Hvalue⟩
          · iframe HgasGlobal Hkind Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames
          wp_for_post
          isimp only [hbodypos] at Hc
          iframe
          ileft
          iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields choice0), tail, out, middleValue, newBase,
            (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
          iframe
          ipureintro
          refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
            ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, ?_, Hadvance.2.2⟩
          all_goals simp only [List.length_append, Int.natCast_add] at Hconsumed Hoffset Hadvance ⊢ <;> omega
      | ignored skipValid =>
        simp only [headerKeyCode]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes tail dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        ihave Hready := headerGasReady_mark base current value mask none $$ Hready
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        ileft
        iexists n, tailConsumed, tailOffset, (headerChoiceSeen seenFields none), tail, current, value, base,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask none)
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields none Hseen, by omega, Hsearch',
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
    iapply HΦ
    iframe Howned Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
    ipureintro
    exact hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_map_rejected
