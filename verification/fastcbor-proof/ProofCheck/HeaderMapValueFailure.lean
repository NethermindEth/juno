import ProofCheck.HeaderValueFailureSpec

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

/-- Actual field-value failure after any successful HeaderEntry prefix. The
    failed field publishes no replacement, earlier outputs remain owned, and
    child cursor/depth progress and the exact diagnostic are preserved. -/
theorem wp_readHeader_map_value_failure (c dst fields gasFields : Loc)
    (v : cursor.Cursor) (f gasF : cursor.Fields) (xs payload wire : List w8) (head : w8)
    (kinds : FeltErrorKinds) (qf qo qn qgf qgo qgn dq : DFrac) (d count consumed offset extraDepth : Nat)
    (entries : List (HeaderEntry (d + 1))) (key : HeaderKeyWire) (reason : HeaderValueFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : entries.length < count)
    (fit : 2 * count ≤ (entries.flatMap HeaderEntry.bytes ++ key.bytes ++ wire).length)
    (fresh : HeaderEntriesBefore (W64 0) entries key.choice)
    (failure : HeaderValueFailure (d + 1) key.choice wire consumed offset extraDepth reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap HeaderEntry.bytes ++ key.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ FeltErrorGlobals kinds ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderOwned dst (entries.foldl HeaderEntry.apply headerZeroValue) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed, depth' := v.depth' + W64 1 + W64 extraDepth} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       HeaderValueFailureError err (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) reason (headerValueFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed) ≤ sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32⌝ }} := by
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
  have Hnext := input_consumption xs ((head :: payload) ++ entries.flatMap HeaderEntry.bytes) (key.bytes ++ wire) v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hbody := congrArg List.length hbytes
  simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega),
    List.length_append, List.length_cons] at hbody
  have hfit : (count : Int) < 2 ^ 63 := by
    simp only [sint.nat, sint.Z, List.length_append] at Hlen hp hbody fit ⊢
    word
  have hcountfit : (2 : Int) * uint.Z (W64 count) ≤
      sint.Z v.buf'.len - (sint.Z v.pos' + 1 + payload.length) := by
    simp only [sint.nat, sint.Z, List.length_append] at Hlen hp hbody fit
    word
  have Hbounds := headerValueFailure_bounds (d + 1) consumed offset extraDepth key.choice wire reason (by omega) failure
  have Hwhole := input_consumption xs ((head :: payload) ++ entries.flatMap HeaderEntry.bytes ++ key.bytes ++ wire) []
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have hprogress : sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + key.bytes.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed) ≤ sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) ≤ sint.Z v.buf'.len ∧ d + 1 + extraDepth ≤ 32 := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hwhole
    exact ⟨by word, by word, by word, by word, Hbounds.2.2⟩
  have hi := input_suffix_head xs (payload ++ entries.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire))
    (sint.nat v.pos') (sint.nat v.buf'.len) head (by simpa only [List.append_assoc, List.cons_append] using hbytes)
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (entries.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_encoded c fields v xs payload (entries.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) head (W64 count) dq
    enc major hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) (entries.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire))
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      entries.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire) := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hchildren := headerChildren_zero (GF := GF)
  isimp only [HeaderChildren] at Hchildren
  icases Hchildren with ⟨Hbody, -, -⟩
  ihave Hready := headerGasReady_zero (GF := GF) (W64 0)
  ihave HI : ((∃ (done todo : List (HeaderEntry (d + 1))) (current : Header) (value : HeaderValue) (base : Loc) (pos next seen : w64),
      "Herrslot" ∷ err_ptr ↦ interface.nil ∗
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 (count - done.length)) next seen true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hbody" ∷ HeaderBody current value ∗
      "Hprices" ∷ prices_ptr ↦ base ∗ "Hready" ∷ HeaderGasReady base current value seen ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜entries = done ++ todo⌝ ∗
      "%Hfresh" ∷ ⌜HeaderEntriesBefore seen todo key.choice⌝ ∗
      "%Htarget" ∷ ⌜todo.foldl HeaderEntry.apply value = entries.foldl HeaderEntry.apply headerZeroValue⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap HeaderEntry.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)⌝) ∨
      (∃ errorLoc : Loc,
        err_ptr ↦ interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∗
        HeaderOwned dst (entries.foldl HeaderEntry.apply headerZeroValue) ∗
        c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 consumed, depth' := v.depth' + W64 1 + W64 extraDepth} ∗
        HeaderValueFailureError errorLoc (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 key.bytes.length + W64 offset) reason (headerValueFailureKind reason kinds)) : IProp GF)
      $$ [s prices err Hdst Hbody Hready Hc]
  · ileft
    iexists [], entries, (zero_val Header), headerZeroValue, null, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe Hbody Hready
    iframe
    ipureintro
    exact ⟨rfl, fresh, rfl, ⟨by decide, by simp [headerFieldKeys]⟩, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for
  icases HI with (Hrun | Hfailed)
  · iNamed Hrun
    wp_auto
    cleanup_bool_decide
    try wp_auto
    have hlength : entries.length = done.length + todo.length := by rw [Hsplit, List.length_append]
    cases todo with
    | nil =>
      have hdoneall : done = entries := by simpa using Hsplit.symm
      have hvalue : value = entries.foldl HeaderEntry.apply headerZeroValue := by simpa using Htarget
      have hposfinal : pos = v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) := by
        rw [hdoneall] at Hposition; word
      have hleft : 0 < sint.Z (W64 (count - done.length)) := by rw [hdoneall]; word
      have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = key.bytes ++ wire := by simpa using Hremaining
      wp_apply wp_nextHeaderKey c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f xs wire key kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch Hfresh Hpos hkey $$ [Hc Hst Hfields Horder Hnames Hs]
        with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 key.bytes.length) ∧ sint.Z (pos + W64 key.bytes.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      generalize hchoice : key.choice = choice at failure ⊢
      cases failure with
      | @felt field wire offset reason failure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds reason $$ Hglobals
        have hread := wp_headerFelt_failure (GF := GF) c dst current value field
          {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} xs wire dq (d + 1) offset reason (feltFailureKind reason kinds)
          failure (by word) (by omega) hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerFeltKey]
        all_goals
          wp_auto
          simp only [headerFeltName] at hread
          wp_apply hread $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr, %Hoffset⟩
          · iframe HerrorKind Hdst Hbody Hc Hs
          wp_for_post
          isimp only [hposfinal] at Hc Herr
          isimp only [hvalue] at Hbody Hready
          ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
          ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @scalar field wire reason failure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds (.expected (W8 0) reason) $$ Hglobals
        isimp only [feltFailureAddr, feltFailureKind] at HerrorKind
        have hread := wp_headerScalar_failure (GF := GF) c dst current value field
          {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} xs wire dq reason (expectedFailureKind reason kinds) failure hvaluepos Hkey.2.2
        cases field <;> simp only [headerKeyCode, headerKeyIndex, headerScalarKey]
        all_goals
          wp_auto
          simp only [headerScalarName, headerScalarMode, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr⟩
          · iframe HerrorKind Hdst Hbody Hc Hs
          wp_for_post
          isimp only [hposfinal] at Hc Herr
          isimp only [hvalue] at Hbody Hready
          ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
          ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderValueFailureError, headerValueFailureKind, feltFailureKind, FeltFailureError, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @signatures wire consumed offset extraDepth reason failure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds reason $$ Hglobals
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerSignatures_failure c dst current value {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} xs wire dq (d + 1) consumed offset extraDepth
          reason (feltFailureKind reason kinds) failure (by word) (by omega) hvaluepos Hkey.2.2
          $$ [Hdst Hbody Hc Hs] as %err ⟨Hdst, Hbody, Hc, Hs, Herr, %Hoffset⟩
        · iframe HerrorKind Hdst Hbody Hc Hs
        wp_for_post
        isimp only [hposfinal] at Hc Herr
        isimp only [hvalue] at Hbody Hready
        ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
        ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      | @string wire reason failure =>
        ihave #HerrorKind := stringErrorGlobals_lookup kinds reason $$ Hglobals
        simp only [headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerString_failure c dst base current value seen {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} xs wire dq reason
          (stringFailureKind reason kinds) failure hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
          as %err ⟨Hdst, Hbody, Hready, Hc, Hs, Herr⟩
        · iframe HerrorKind Hdst Hbody Hready Hc Hs
        wp_for_post
        isimp only [hposfinal] at Hc Herr
        isimp only [hvalue] at Hbody Hready
        ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
        ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
      | @gas second final wire consumed offset entered reason failure =>
        have hfresh : HeaderKeyFresh seen (some (headerGasKey second)) := by simpa only [HeaderEntriesBefore, hchoice] using Hfresh
        have hread := wp_headerGas_fresh_failure (GF := GF) c dst prices_ptr base gasFields current value second final seen
          {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} gasF xs wire kinds qgf qgo qgn dq (d + 1) consumed offset entered reason
          hfresh failure (by word) (by omega) hvaluepos Hkey.2.2
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
          try isimp only [← hchoice] at Hready
          wp_for_post
          isimp only [hposfinal] at Hc Herr
          isimp only [hvalue] at Hbody Hready
          ihave Hstate := headerGasReady_forget newBase current (entries.foldl HeaderEntry.apply headerZeroValue) (headerKeySeen seen key.choice) $$ Hready
          ihave Hchildren := headerGasState_publish newBase current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
          · iframe Hbody Hstate
          iframe
          iright
          iexists err
          isimp only [HeaderValueFailureError, headerValueFailureKind, Int.natCast_zero, hz]
          iframe Herrslot Hc Herr
          unfold HeaderOwned
          iexists current
          iframe Hdst Hchildren
      | @ignored wire offset reason failure =>
        ihave #HerrorKind := feltErrorGlobals_lookup kinds (.skipped reason) $$ Hglobals
        isimp only [feltFailureAddr, feltFailureKind] at HerrorKind
        simp only [headerKeyCode]
        wp_auto
        wp_apply wp_Skip_failure_bounded c {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1} xs wire dq (d + 1) offset reason
          (skipFailureKind reason kinds.toSkipErrorKinds) failure (by word) (by omega) hvaluepos Hkey.2.2
          $$ [Hc Hs] as %err ⟨Hc, Hs, Herr, %Hoffset⟩
        · iframe HerrorKind Hc Hs
        wp_for_post
        isimp only [hposfinal] at Hc Herr
        isimp only [hvalue] at Hbody Hready
        ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
        ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
        · iframe Hbody Hstate
        iframe
        iright
        iexists err
        isimp only [HeaderValueFailureError, headerValueFailureKind, feltFailureKind, FeltFailureError, Int.natCast_zero, hz]
        iframe Herrslot Hc Herr
        unfold HeaderOwned
        iexists current
        iframe Hdst Hchildren
    | cons entry todo =>
      have hleft : 0 < sint.Z (W64 (count - done.length)) := by simp only [List.length_cons] at hlength; word
      have hleftstep : W64 (count - done.length) - W64 1 = W64 (count - (done ++ [entry]).length) := by
        simp only [List.length_cons] at hlength
        simp only [List.length_append, List.length_singleton]; word
      have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.bytes ++ (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) := by
        simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
      have Hadvance := input_consumption xs entry.bytes (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) pos v.buf'.len Hpos Hlen.1 hentry
      have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.key.bytes ++ (entry.valueBytes ++ (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire))) := by
        simpa only [HeaderEntry.bytes, List.append_assoc] using hentry
      have hnewlen : ((done ++ [entry]).flatMap HeaderEntry.bytes).length = (done.flatMap HeaderEntry.bytes).length + entry.bytes.length := by simp
      have hbodypos : pos + W64 entry.key.bytes.length + W64 entry.valueBytes.length = pos + W64 entry.bytes.length := by
        simp only [HeaderEntry.bytes, List.length_append]; word
      rcases Hfresh with ⟨hfresh, hrestfresh⟩
      wp_apply wp_nextHeaderKey c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
        (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f xs
        (entry.valueBytes ++ (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire))) entry.key kinds.unsupportedKind qf qo qn dq
        rfl hleft Hsearch hfresh Hpos hkey $$ [Hc Hst Hfields Horder Hnames Hs]
        with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      dsimp only at Hkey
      have hvaluepos : 0 ≤ sint.Z (pos + W64 entry.key.bytes.length) ∧ sint.Z (pos + W64 entry.key.bytes.length) ≤ sint.Z v.buf'.len :=
        ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
      isimp only [hleftstep] at Hst
      cases entry with
      | felt field entryKey choice item =>
        let entry0 : HeaderEntry (d + 1) := .felt field entryKey choice item
        let out := headerSetFeltPointer field current
        let final := headerSetFeltValue field value item.value
        have hread := wp_headerFelt_success (GF := GF) c dst base current value field seen
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) item.value dq (d + 1)
          (by word) (by omega) item.valid hvaluepos Hkey.2.2
        cases field <;> simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex, headerFeltKey]
        all_goals
          wp_auto
          simp only [headerFeltName] at hread
          wp_apply hread $$ [Hdst Hbody Hready Hc Hs] as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
          · iframe Hdst Hbody Hready Hc Hs
          wp_for_post
          simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
          isimp only [hbodypos] at Hc
          have hchoice := choice
          dsimp only [headerFeltKey] at hchoice
          try isimp only [← hchoice] at Hst
          isimp only [headerFeltKey, ← hchoice] at Hready
          iframe
          ileft
          iexists (done ++ [entry0]), todo, (out p), final, base,
            (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
          iframe
          ipureintro
          refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
          · rw [Hadvance.1]; omega
          · rw [Hadvance.1, hnewlen]; omega
      | scalar field entryKey choice item =>
        let entry0 : HeaderEntry (d + 1) := .scalar field entryKey choice item
        let final := headerSetScalarValue field value (item.value.getD (headerScalarValue field value))
        let out := headerSetScalarWord field current (item.value.getD (headerScalarValue field value))
        have hread := wp_headerScalar_success (GF := GF) c dst base current value field seen
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) (item.value.getD (headerScalarValue field value)) dq
          (item.valid _) hvaluepos Hkey.2.2
        cases field <;> simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex, headerScalarKey]
        all_goals
          wp_auto
          simp only [headerScalarName, headerScalarMode, Bool.false_eq_true, ite_false, ite_true] at hread
          wp_apply hread $$ [Hdst Hbody Hready Hc Hs] with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
          · iframe Hdst Hbody Hready Hc Hs
          wp_for_post
          simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
          isimp only [hbodypos] at Hc
          have hchoice := choice
          dsimp only [headerScalarKey] at hchoice
          try isimp only [← hchoice] at Hst
          isimp only [headerScalarKey, ← hchoice] at Hready
          iframe
          ileft
          iexists (done ++ [entry0]), todo, out, final, base,
            (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
          iframe
          ipureintro
          refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
          · rw [Hadvance.1]; omega
          · rw [Hadvance.1, hnewlen]; omega
      | signatures entryKey choice item =>
        let entry0 : HeaderEntry (d + 1) := .signatures entryKey choice item
        simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerSignatures_success c dst base current value seen
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) item.value dq (d + 1)
          (by word) (by omega) item.valid hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
          as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        wp_for_post
        simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
        isimp only [hbodypos] at Hc
        try isimp only [← choice] at Hst
        isimp only [← choice] at Hready
        iframe
        ileft
        iexists (done ++ [entry0]), todo, {current with Signatures' := p}, {value with signatures := item.value}, base,
          (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
      | versionNull entryKey choice head hn =>
        let entry0 : HeaderEntry (d + 1) := .versionNull entryKey choice head hn
        simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex]
        wp_auto
        wp_apply wp_headerString_null c dst base current value seen
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) head dq hn hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
          with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        ihave Hready := headerGasReady_mark base current value seen (some .protocolVersion) $$ Hready
        wp_for_post
        simp only [HeaderEntry.key, HeaderEntry.valueBytes, List.length_singleton, Int.natCast_one] at hbodypos
        isimp only [hbodypos] at Hc
        try isimp only [← choice] at Hst
        isimp only [← choice] at Hready
        iframe
        ileft
        iexists (done ++ [entry0]), todo, current, value, base,
          (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
      | gas second entryKey choice item =>
        have freshField : HeaderKeyFresh seen (some (headerGasKey second)) := by simpa only [HeaderEntry.key, choice] using hfresh
        cases item with
        | nullish head hn =>
          let entry0 : HeaderEntry (d + 1) := .gas second entryKey choice (.nullish head hn)
          let out := headerSetGasPointer second current null
          let final := headerSetGasValue second value none
          have hread := wp_headerGas_null_ready (GF := GF) c dst prices_ptr base current value seen second
            {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
            xs (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) head dq hn hvaluepos Hkey.2.2
          cases second <;> simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex, headerGasKey,
            Bool.false_eq_true, ite_false, ite_true]
          all_goals
            wp_auto
            simp only [headerGasName, headerGasSlot, Bool.false_eq_true, ite_false, ite_true] at hread
            wp_apply hread $$ [Hdst Hbody Hprices Hready Hc Hs] with ⟨Hdst, Hbody, Hprices, Hready, Hc, Hs, %Hvalue⟩
            · iframe Hdst Hbody Hprices Hready Hc Hs
            wp_for_post
            simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderGasWire.bytes, List.length_singleton, Int.natCast_one] at hbodypos
            isimp only [hbodypos] at Hc
            have hchoice := choice
            simp only [headerGasKey, Bool.false_eq_true, ite_false, ite_true] at hchoice
            try isimp only [← hchoice] at Hst
            isimp only [headerGasKey, Bool.false_eq_true, ite_false, ite_true, ← hchoice] at Hready
            iframe
            ileft
            iexists (done ++ [entry0]), todo, out, final, base,
              (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
            iframe
            ipureintro
            refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
            · simpa only [List.append_assoc, List.singleton_append] using Hsplit
            · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
            · rw [Hadvance.1]; omega
            · rw [Hadvance.1, hnewlen]; omega
        | map head payload children enc major bound fresh =>
          let entry0 : HeaderEntry (d + 1) := .gas second entryKey choice (.map head payload children enc major bound fresh)
          let out := fun base => headerSetGasPointer second current (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base)
          let final := headerSetGasValue second value (some (children.foldl GasPriceEntry.apply ⟨none, none⟩))
          have hread := wp_headerGas_fresh_map (GF := GF) c dst prices_ptr base gasFields current value second seen
            {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
            gasF xs payload (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) head kinds.unsupportedKind qgf qgo qgn dq (d + 1) children
            freshField enc major (by word) bound fresh hvaluepos Hkey.2.2
          cases second <;> simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex, headerGasKey,
            Bool.false_eq_true, ite_false, ite_true]
          all_goals
            wp_auto
            simp only [headerGasName, headerGasSlot, Bool.false_eq_true, ite_false, ite_true] at hread
            wp_apply hread $$ [Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames]
              as %newBase ⟨Hdst, Hbody, Hprices, Hready, Hc, Hs, HgasFields, HgasOrder, HgasNames, %Hvalue⟩
            · iframe HgasGlobal Hkind Hdst Hbody Hprices Hready Hc Hs HgasFields HgasOrder HgasNames
            wp_for_post
            simp only [HeaderEntry.key, HeaderEntry.valueBytes, HeaderGasWire.bytes, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1, Int.add_assoc] at hbodypos
            have hgaspos : pos + W64 entryKey.bytes.length + W64 (1 + payload.length + (children.flatMap GasPriceEntry.bytes).length) =
                pos + W64 entry0.bytes.length := by
              simp only [entry0, HeaderEntry.bytes, HeaderEntry.key, HeaderEntry.valueBytes,
                HeaderGasWire.bytes, List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one]
              word
            isimp only [hgaspos] at Hc
            have hchoice := choice
            simp only [headerGasKey, Bool.false_eq_true, ite_false, ite_true] at hchoice
            try isimp only [← hchoice] at Hst
            isimp only [headerGasKey, Bool.false_eq_true, ite_false, ite_true, ← hchoice] at Hready
            iframe
            ileft
            iexists (done ++ [entry0]), todo, (out newBase), final, newBase,
              (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
            iframe
            ipureintro
            refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
            · simpa only [List.append_assoc, List.singleton_append] using Hsplit
            · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
            · rw [Hadvance.1]; omega
            · rw [Hadvance.1, hnewlen]; omega
      | ignored entryKey choice ignoredWire valid =>
        let entry0 : HeaderEntry (d + 1) := .ignored entryKey choice ignoredWire valid
        simp only [HeaderEntry.key, choice, headerKeyCode]
        wp_auto
        wp_apply wp_Skip_valid c {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs ignoredWire (todo.flatMap HeaderEntry.bytes ++ (key.bytes ++ wire)) dq (d + 1) (by word) (by omega) valid hvaluepos Hkey.2.2
          $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
        · iframe Hc Hs
        ihave Hready := headerGasReady_mark base current value seen none $$ Hready
        wp_for_post
        simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
        isimp only [hbodypos] at Hc
        try isimp only [← choice] at Hst
        isimp only [← choice] at Hready
        iframe
        ileft
        iexists (done ++ [entry0]), todo, current, value, base,
          (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen entryKey.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega

  · icases Hfailed with ⟨%err, Herrslot, Hdst, Hc, Herr⟩
    wp_auto
    cleanup_bool_decide
    try wp_auto
    iapply HΦ
    iframe Hdst Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr
    ipureintro
    exact hprogress

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_map_value_failure
