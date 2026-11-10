import ProofCheck.HeaderKeyDuplicate
import ProofCheck.HeaderMapSpec

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

/-- Fresh Header entries followed by an already-seen schema identity. -/
def HeaderEntriesBeforeDuplicate {d : Nat} (seen : w64) : List (HeaderEntry d) → HeaderField → Prop
  | [], duplicateField => seen &&& (W64 1 <<< headerKeyIndex duplicateField) ≠ W64 0
  | entry :: rest, duplicateField => HeaderKeyFresh seen entry.key.choice ∧
      HeaderEntriesBeforeDuplicate (headerKeySeen seen entry.key.choice) rest duplicateField

/-- Actual duplicate rejection after any successful supported-key Header prefix. The
    duplicate key is consumed but its value is untouched, and earlier fields
    and their nested allocations remain owned. -/
theorem wp_readHeader_map_duplicate (c dst fields gasFields : Loc)
    (v : cursor.Cursor) (f gasF : cursor.Fields) (xs payload wire : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) (d count : Nat)
    (entries : List (HeaderEntry (d + 1))) (duplicate : HeaderKeyWire) (duplicateField : HeaderField)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : entries.length < count)
    (fit : 2 * count ≤ (entries.flatMap HeaderEntry.bytes ++ duplicate.bytes ++ wire).length)
    (beforeDuplicate : HeaderEntriesBeforeDuplicate (W64 0) entries duplicateField)
    (choice : duplicate.choice = some duplicateField)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap HeaderEntry.bytes ++ duplicate.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       globalAddr cborHeaderFields ↦□ fields ∗ globalAddr cborGasPriceFields ↦□ gasFields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err); ∃ args : GoSlice,
       HeaderOwned dst (entries.foldl HeaderEntry.apply headerZeroValue) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 duplicate.bytes.length, depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       err ↦ cursor.DecodeError.mk kind (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 duplicate.bytes.length) (go!"field %s named twice") args ∗
       args ↦* [interface.mkOk go.string #(headerFieldWireName duplicateField)] ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 duplicate.bytes.length) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap HeaderEntry.bytes).length + duplicate.bytes.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 duplicate.bytes.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) + W64 duplicate.bytes.length)) (sint.nat v.buf'.len) xs = wire⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ entries.flatMap HeaderEntry.bytes) (duplicate.bytes ++ wire) v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
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
  have hi := input_suffix_head xs (payload ++ entries.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire))
    (sint.nat v.pos') (sint.nat v.buf'.len) head (by simpa only [List.append_assoc, List.cons_append] using hbytes)
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (entries.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_openStruct_encoded c fields v xs payload (entries.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) head (W64 count) dq
    enc major hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) (entries.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire))
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      entries.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire) := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hchildren := headerChildren_zero (GF := GF)
  isimp only [HeaderChildren] at Hchildren
  icases Hchildren with ⟨Hbody, -, -⟩
  ihave Hready := headerGasReady_zero (GF := GF) (W64 0)
  ihave HI : (∃ (done todo : List (HeaderEntry (d + 1))) (current : Header) (value : HeaderValue) (base : Loc) (pos next seen : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 (count - done.length)) next seen true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hbody" ∷ HeaderBody current value ∗
      "Hprices" ∷ prices_ptr ↦ base ∗ "Hready" ∷ HeaderGasReady base current value seen ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜entries = done ++ todo⌝ ∗
      "%Hfresh" ∷ ⌜HeaderEntriesBeforeDuplicate seen todo duplicateField⌝ ∗
      "%Htarget" ∷ ⌜todo.foldl HeaderEntry.apply value = entries.foldl HeaderEntry.apply headerZeroValue⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap HeaderEntry.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)⌝ : IProp GF)
      $$ [s prices Hdst Hbody Hready Hc]
  · iexists [], entries, (zero_val Header), headerZeroValue, null, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe Hbody Hready
    iframe
    ipureintro
    exact ⟨rfl, beforeDuplicate, rfl, ⟨by decide, by simp [headerFieldKeys]⟩, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : entries.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  cases todo with
  | nil =>
    have hdoneall : done = entries := by simpa using Hsplit.symm
    have hvalue : value = entries.foldl HeaderEntry.apply headerZeroValue := by simpa using Htarget
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (entries.flatMap HeaderEntry.bytes).length) := by
      rw [hdoneall] at Hposition; word
    have hleft : 0 < sint.Z (W64 (count - done.length)) := by rw [hdoneall]; word
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = duplicate.bytes ++ wire := by simpa using Hremaining
    wp_apply wp_nextHeaderKey_duplicate c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f xs wire duplicate duplicateField kind qf qo qn dq
      rfl hleft Hsearch choice Hfresh Hpos hkey $$ [Hc Hst Hfields Horder Hnames Hs]
      as %err ⟨%next', %args, Hc, Hst, Hfields, Horder, Hnames, Hs, Herr, Hargs, %Hsearch', %Hkey⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    wp_for_post
    isimp only [hposfinal] at Hc Herr
    isimp only [hvalue] at Hbody Hready
    ihave Hstate := headerGasReady_forget base current (entries.foldl HeaderEntry.apply headerZeroValue) seen $$ Hready
    ihave Hchildren := headerGasState_publish base current (entries.foldl HeaderEntry.apply headerZeroValue) $$ [Hbody Hstate]
    · iframe Hbody Hstate
    iapply HΦ
    iexists args
    iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames Herr Hargs
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists current
      iframe Hdst Hchildren
    ipureintro
    dsimp only at Hkey
    rw [hdoneall] at Hposition
    refine ⟨?_, ?_, ?_⟩
    · rw [← hposfinal, Hkey.1]; omega
    · simpa only [hposfinal] using Hkey.2.1
    · simpa only [hposfinal] using Hkey.2.2
  | cons entry todo =>
    have hleft : 0 < sint.Z (W64 (count - done.length)) := by simp only [List.length_cons] at hlength; word
    have hleftstep : W64 (count - done.length) - W64 1 = W64 (count - (done ++ [entry]).length) := by
      simp only [List.length_cons] at hlength
      simp only [List.length_append, List.length_singleton]; word
    have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.bytes ++ (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) := by
      simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs entry.bytes (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) pos v.buf'.len Hpos Hlen.1 hentry
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.key.bytes ++ (entry.valueBytes ++ (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire))) := by
      simpa only [HeaderEntry.bytes, List.append_assoc] using hentry
    have hnewlen : ((done ++ [entry]).flatMap HeaderEntry.bytes).length = (done.flatMap HeaderEntry.bytes).length + entry.bytes.length := by simp
    have hbodypos : pos + W64 entry.key.bytes.length + W64 entry.valueBytes.length = pos + W64 entry.bytes.length := by
      simp only [HeaderEntry.bytes, List.length_append]; word
    rcases Hfresh with ⟨hfresh, hrestfresh⟩
    wp_apply wp_nextHeaderKey c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f xs
      (entry.valueBytes ++ (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire))) entry.key kind qf qo qn dq
      rfl hleft Hsearch hfresh Hpos hkey $$ [Hc Hst Hfields Horder Hnames Hs]
      with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    dsimp only at Hkey
    have hvaluepos : 0 ≤ sint.Z (pos + W64 entry.key.bytes.length) ∧ sint.Z (pos + W64 entry.key.bytes.length) ≤ sint.Z v.buf'.len :=
      ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
    isimp only [hleftstep] at Hst
    cases entry with
    | felt field key choice item =>
      let entry0 : HeaderEntry (d + 1) := .felt field key choice item
      let out := headerSetFeltPointer field current
      let final := headerSetFeltValue field value item.value
      have hread := wp_headerFelt_success (GF := GF) c dst base current value field seen
        {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
        xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) item.value dq (d + 1)
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
        iexists (done ++ [entry0]), todo, (out p), final, base,
          (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
    | scalar field key choice item =>
      let entry0 : HeaderEntry (d + 1) := .scalar field key choice item
      let final := headerSetScalarValue field value (item.value.getD (headerScalarValue field value))
      let out := headerSetScalarWord field current (item.value.getD (headerScalarValue field value))
      have hread := wp_headerScalar_success (GF := GF) c dst base current value field seen
        {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
        xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) (item.value.getD (headerScalarValue field value)) dq
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
        iexists (done ++ [entry0]), todo, out, final, base,
          (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
    | signatures key choice item =>
      let entry0 : HeaderEntry (d + 1) := .signatures key choice item
      simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex]
      wp_auto
      wp_apply wp_headerSignatures_success c dst base current value seen
        {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
        xs item.bytes (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) item.value dq (d + 1)
        (by word) (by omega) item.valid hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
        as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
      · iframe Hdst Hbody Hready Hc Hs
      wp_for_post
      simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
      isimp only [hbodypos] at Hc
      try isimp only [← choice] at Hst
      isimp only [← choice] at Hready
      iframe
      iexists (done ++ [entry0]), todo, {current with Signatures' := p}, {value with signatures := item.value}, base,
        (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
      iframe
      ipureintro
      refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
      · rw [Hadvance.1]; omega
      · rw [Hadvance.1, hnewlen]; omega
    | versionNull key choice head hn =>
      let entry0 : HeaderEntry (d + 1) := .versionNull key choice head hn
      simp only [HeaderEntry.key, choice, headerKeyCode, headerKeyIndex]
      wp_auto
      wp_apply wp_headerString_null c dst base current value seen
        {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
        xs (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) head dq hn hvaluepos Hkey.2.2 $$ [Hdst Hbody Hready Hc Hs]
        with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
      · iframe Hdst Hbody Hready Hc Hs
      ihave Hready := headerGasReady_mark base current value seen (some .protocolVersion) $$ Hready
      wp_for_post
      simp only [HeaderEntry.key, HeaderEntry.valueBytes, List.length_singleton, Int.natCast_one] at hbodypos
      isimp only [hbodypos] at Hc
      try isimp only [← choice] at Hst
      isimp only [← choice] at Hready
      iframe
      iexists (done ++ [entry0]), todo, current, value, base,
        (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
      iframe
      ipureintro
      refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
      · rw [Hadvance.1]; omega
      · rw [Hadvance.1, hnewlen]; omega
    | gas second key choice item =>
      have freshField : HeaderKeyFresh seen (some (headerGasKey second)) := by simpa only [HeaderEntry.key, choice] using hfresh
      cases item with
      | nullish head hn =>
        let entry0 : HeaderEntry (d + 1) := .gas second key choice (.nullish head hn)
        let out := headerSetGasPointer second current null
        let final := headerSetGasValue second value none
        have hread := wp_headerGas_null_ready (GF := GF) c dst prices_ptr base current value seen second
          {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
          xs (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) head dq hn hvaluepos Hkey.2.2
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
          iexists (done ++ [entry0]), todo, out, final, base,
            (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
          iframe
          ipureintro
          refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
          · rw [Hadvance.1]; omega
          · rw [Hadvance.1, hnewlen]; omega
      | map head payload children enc major bound fresh =>
        let entry0 : HeaderEntry (d + 1) := .gas second key choice (.map head payload children enc major bound fresh)
        let out := fun base => headerSetGasPointer second current (arrayIndexRef GasPrice (sint.Z (headerGasSlot second)) base)
        let final := headerSetGasValue second value (some (children.foldl GasPriceEntry.apply ⟨none, none⟩))
        have hread := wp_headerGas_fresh_map (GF := GF) c dst prices_ptr base gasFields current value second seen
          {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
          gasF xs payload (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) head kind qgf qgo qgn dq (d + 1) children
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
          have hgaspos : pos + W64 key.bytes.length + W64 (1 + payload.length + (children.flatMap GasPriceEntry.bytes).length) =
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
          iexists (done ++ [entry0]), todo, (out newBase), final, newBase,
            (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
          iframe
          ipureintro
          refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
          · simpa only [List.append_assoc, List.singleton_append] using Hsplit
          · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
          · rw [Hadvance.1]; omega
          · rw [Hadvance.1, hnewlen]; omega
    | ignored key choice ignoredWire valid =>
      let entry0 : HeaderEntry (d + 1) := .ignored key choice ignoredWire valid
      simp only [HeaderEntry.key, choice, headerKeyCode]
      wp_auto
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.bytes.length, depth' := v.depth' + W64 1}
        xs ignoredWire (todo.flatMap HeaderEntry.bytes ++ (duplicate.bytes ++ wire)) dq (d + 1) (by word) (by omega) valid hvaluepos Hkey.2.2
        $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
      · iframe Hc Hs
      ihave Hready := headerGasReady_mark base current value seen none $$ Hready
      wp_for_post
      simp only [HeaderEntry.key, HeaderEntry.valueBytes] at hbodypos
      isimp only [hbodypos] at Hc
      try isimp only [← choice] at Hst
      isimp only [← choice] at Hready
      iframe
      iexists (done ++ [entry0]), todo, current, value, base,
        (pos + W64 (HeaderEntry.bytes entry0).length), next', (headerKeySeen seen key.choice)
      iframe
      ipureintro
      refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · simpa only [List.foldl_cons, HeaderEntry.apply, HeaderGasWire.value] using Htarget
      · rw [Hadvance.1]; omega
      · rw [Hadvance.1, hnewlen]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_map_duplicate
