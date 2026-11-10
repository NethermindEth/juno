import ProofCheck.HeaderMapSpec
import ProofCheck.HeaderStringValid
import ProofCheck.HeaderGasComplete
import ProofCheck.SupportedSpec

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

/-- The generated Header loop realizes every complete successful pair
    specification from a zero destination. Arbitrary field order, all supported
    key/value encodings and all fifteen fields are covered, conditional only on
    the unresolved production UTF-8 validator contract. -/
theorem wp_readHeader_map_valid (validator : Utf8ValidatorContract (GF := GF)) (c dst fields gasFields : Loc)
    (v : cursor.Cursor) (f gasF : cursor.Fields) (xs payload body rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn qgf qgo qgn dq : DFrac) (d count : Nat) (final : HeaderValue)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (valid : HeaderPairsValid (d + 1) count [] headerZeroValue body final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborHeaderFields ↦□ fields ∗ globalAddr internal.cborschema.cborGasPriceFields ↦□ gasFields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ (zero_val Header) ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames }}
      (App (App (Val (@! cborReadHeader)) (Val #c)) (Val #dst))
    {{ RET #interface.nil;
       HeaderOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} headerFieldKeys ∗ f.names' ↦*{qn} headerFieldNames ∗
       gasFields ↦{qgf} gasF ∗ gasF.order' ↦*{qgo} gasFieldKeys ∗ gasF.names' ↦*{qgn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) =
           sint.Z v.pos' + 1 + payload.length + body.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #HgasGlobal, #Hkind, Hdst, Hc, Hs, Hfields, Horder, Hnames, HgasFields, HgasOrder, HgasNames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hentries := headerPairs_count_bound (d + 1) count [] headerZeroValue final body valid
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hi := input_suffix_head xs (payload ++ body ++ rem)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (body ++ rem) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_encoded c fields v xs payload (body ++ rem) head (W64 count) dq
    enc major hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) (body ++ rem)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      body ++ rem := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave Hchildren := headerChildren_zero (GF := GF)
  isimp only [HeaderChildren] at Hchildren
  icases Hchildren with ⟨Hbody, -, -⟩
  ihave Hready := headerGasReady_zero (GF := GF) (W64 0)
  ihave HI : (∃ (remaining : Nat) (seenFields : List HeaderField) (todo : List w8)
      (current : Header) (value : HeaderValue) (base : Loc) (pos next mask : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hbody" ∷ HeaderBody current value ∗
      "Hprices" ∷ prices_ptr ↦ base ∗ "Hready" ∷ HeaderGasReady base current value mask ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hvalid" ∷ ⌜HeaderPairsValid (d + 1) remaining seenFields value todo final⌝ ∗
      "%Hseen" ∷ ⌜HeaderSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ headerFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + 1 + payload.length + body.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF)
      $$ [s prices Hdst Hbody Hready Hc]
  · iexists count, [], body, (zero_val Header), headerZeroValue, null, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe Hbody Hready
    iframe
    ipureintro
    exact ⟨valid, headerSeen_zero, Nat.le_refl _, ⟨by decide, by simp [headerFieldKeys]⟩,
      ⟨by word, by word⟩, by omega, hremaining⟩
  wp_for HI
  cases Hvalid with
  | nil =>
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + body.length) := by
      simp only [List.length_nil] at Hposition; word
    wp_apply wp_nextField_close c s_ptr fields {v with pos' := pos} next mask $$ [Hst Hc] with ⟨Hst, Hc⟩
    · iframe Hst Hc
    wp_for_post
    isimp only [hposfinal] at Hc
    ihave Hstate := headerGasReady_forget base current final mask $$ Hready
    ihave Hchildren := headerGasState_publish base current final $$ [Hbody Hstate]
    · iframe Hbody Hstate
    iapply HΦ
    iframe Hc Hs Hfields Horder Hnames HgasFields HgasOrder HgasNames
    isplitl [Hdst Hchildren]
    · unfold HeaderOwned
      iexists current
      iframe Hdst Hchildren
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1, Int.add_assoc] using And.intro Hnext.1 Hnext.2.2
  | @step n seenFields choice initial middle target key valueBytes tail keyValid fresh valueValid rest =>
    have hleft : 0 < sint.Z (W64 (n + 1)) := by word
    have hleftstep : W64 (n + 1) - W64 1 = W64 n := by word
    have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs =
        (key ++ valueBytes) ++ (tail ++ rem) := by
      simpa only [List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs (key ++ valueBytes) (tail ++ rem) pos v.buf'.len Hpos Hlen.1 hentry
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = key ++ (valueBytes ++ (tail ++ rem)) := by
      simpa only [List.append_assoc] using hentry
    have hbodypos : pos + W64 key.length + W64 valueBytes.length = pos + W64 (key ++ valueBytes).length := by
      simp only [List.length_append]; word
    have hfresh : HeaderKeyFresh mask choice := by
      cases choice with
      | none => trivial
      | some field => exact (Hseen field).mpr fresh
    isimp only [Int.natCast_add, Int.natCast_one] at Hst
    wp_apply wp_nextHeaderKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs (valueBytes ++ (tail ++ rem))
      key choice kind qf qo qn dq rfl hleft Hsearch keyValid hfresh Hpos hkey
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
        xs valueBytes (tail ++ rem) feltValue dq (d + 1) (by word) (by omega) feltValid hvaluepos Hkey.2.2
      cases field <;> simp only [headerKeyCode, headerKeyIndex, headerFeltKey]
      all_goals
        wp_auto
        simp only [headerFeltName] at hread
        wp_apply hread $$ [Hdst Hbody Hready Hc Hs] as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        iexists n, (headerChoiceSeen seenFields choice0), tail, (out p), middleValue, base,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | @scalar field old wire scalarValue scalarValid =>
      let choice0 := some (headerScalarKey field)
      let out := headerSetScalarWord field current scalarValue
      let middleValue := headerSetScalarValue field value scalarValue
      have hread := wp_headerScalar_success (GF := GF) c dst base current value field mask
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) scalarValue dq scalarValid hvaluepos Hkey.2.2
      cases field <;> simp only [headerKeyCode, headerKeyIndex, headerScalarKey]
      all_goals
        wp_auto
        simp only [headerScalarName, headerScalarMode, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hbody Hready Hc Hs] with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
        · iframe Hdst Hbody Hready Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        iexists n, (headerChoiceSeen seenFields choice0), tail, out, middleValue, base,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | @signatures old wire sigValue sigValid =>
      simp only [headerKeyCode, headerKeyIndex]
      wp_auto
      wp_apply wp_headerSignatures_success c dst base current value mask
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) sigValue dq (d + 1) (by word) (by omega) sigValid hvaluepos Hkey.2.2
        $$ [Hdst Hbody Hready Hc Hs] as %p ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
      · iframe Hdst Hbody Hready Hc Hs
      wp_for_post
      isimp only [hbodypos] at Hc
      iframe
      iexists n, (headerChoiceSeen seenFields (some .signatures)), tail, {current with Signatures' := p}, {value with signatures := sigValue}, base,
        (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask (some .signatures))
      iframe
      ipureintro
      refine ⟨rest, headerSeen_mark mask seenFields (some .signatures) Hseen, by omega, Hsearch',
        ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
      omega
    | @string old wire textValue textValid =>
      simp only [headerKeyCode, headerKeyIndex]
      wp_auto
      wp_apply wp_headerString_valid validator c dst base current value mask
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) textValue dq textValid hvaluepos Hkey.2.2
        $$ [Hdst Hbody Hready Hc Hs] with ⟨Hdst, Hbody, Hready, Hc, Hs, %Hvalue⟩
      · iframe Hdst Hbody Hready Hc Hs
      wp_for_post
      isimp only [hbodypos] at Hc
      iframe
      iexists n, (headerChoiceSeen seenFields (some .protocolVersion)), tail, {current with ProtocolVersion' := textValue}, {value with protocolVersion := textValue}, base,
        (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask (some .protocolVersion))
      iframe
      ipureintro
      refine ⟨rest, headerSeen_mark mask seenFields (some .protocolVersion) Hseen, by omega, Hsearch',
        ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
      omega
    | @gas second old wire gasValue gasValid =>
      let choice0 := some (headerGasKey second)
      let middleValue := headerSetGasValue second value gasValue
      have hread := wp_headerGas_valid (GF := GF) validator c dst prices_ptr base gasFields current value second mask
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        gasF xs valueBytes (tail ++ rem) gasValue kind qgf qgo qgn dq (d + 1)
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
        iexists n, (headerChoiceSeen seenFields choice0), tail, out, middleValue, newBase,
          (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask choice0)
        iframe
        ipureintro
        refine ⟨rest, headerSeen_mark mask seenFields choice0 Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | ignored skipValid =>
      simp only [headerKeyCode]
      wp_auto
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
        $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
      · iframe Hc Hs
      ihave Hready := headerGasReady_mark base current value mask none $$ Hready
      wp_for_post
      isimp only [hbodypos] at Hc
      iframe
      iexists n, (headerChoiceSeen seenFields none), tail, current, value, base,
        (pos + W64 (key ++ valueBytes).length), next', (headerKeySeen mask none)
      iframe
      ipureintro
      refine ⟨rest, headerSeen_mark mask seenFields none Hseen, by omega, Hsearch',
        ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeader_map_valid
