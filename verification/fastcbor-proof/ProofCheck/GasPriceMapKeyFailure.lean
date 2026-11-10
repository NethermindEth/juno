import ProofCheck.KeyFailureSound
import ProofCheck.GasPriceKeys
import ProofCheck.GasPriceMapSpec

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

/-- Actual key rejection after any successful supported-key prefix. The cursor
    remains at the rejected key, earlier fields and children remain owned, and
    the exact diagnostic and unread suffix are preserved. -/
theorem wp_readGasPrice_map_key_failure (c dst fields : Loc) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload wire : List w8) (head : w8)
    (kind errorKind : GoInterface) (qf qo qn dq : DFrac) (d count : Nat)
    (entries : List (GasPriceEntry (d + 1))) (reason : KeyFailureReason)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32)
    (more : entries.length < count)
    (fit : 2 * count ≤ (entries.flatMap GasPriceEntry.bytes ++ wire).length)
    (fresh : GasEntriesFresh (W64 0) entries)
    (failure : KeyFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ entries.flatMap GasPriceEntry.bytes ++ wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       keyFailureAddr (GF := GF) reason ↦□ errorKind ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (out : GasPrice) (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ out ∗ GasPriceChildren out (entries.foldl GasPriceEntry.apply initial) ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length), depth' := v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       KeyFailureError err (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) reason errorKind ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) = sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length))) (sint.nat v.buf'.len) xs = wire⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, #HerrorKind, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ entries.flatMap GasPriceEntry.bytes) wire v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length ≤ sint.Z v.buf'.len := by
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
  have hi := input_suffix_head xs (payload ++ entries.flatMap GasPriceEntry.bytes ++ wire)
    (sint.nat v.pos') (sint.nat v.buf'.len) head (by simpa only [List.append_assoc, List.cons_append] using hbytes)
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ (entries.flatMap GasPriceEntry.bytes ++ wire) := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_encoded c fields v xs payload (entries.flatMap GasPriceEntry.bytes ++ wire) head (W64 count) dq
    enc major hp.1 (by omega) (by omega) hcountfit hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  have Hhead := input_consumption xs (head :: payload) (entries.flatMap GasPriceEntry.bytes ++ wire)
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs =
      entries.flatMap GasPriceEntry.bytes ++ wire := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (GasPriceEntry (d + 1))) (current : GasPrice) (value : GasPriceValue) (pos next seen : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 (count - done.length)) next seen true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hchildren" ∷ GasPriceChildren current value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜entries = done ++ todo⌝ ∗
      "%Hfresh" ∷ ⌜GasEntriesFresh seen todo⌝ ∗
      "%Htarget" ∷ ⌜todo.foldl GasPriceEntry.apply value = entries.foldl GasPriceEntry.apply initial⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ gasFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + (done.flatMap GasPriceEntry.bytes).length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo.flatMap GasPriceEntry.bytes ++ wire⌝ : IProp GF)
      $$ [s Hdst Hchildren Hc]
  · iexists [], entries, g, initial, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe
    ipureintro
    exact ⟨rfl, fresh, rfl, ⟨by decide, by simp [gasFieldKeys]⟩, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have hlength : entries.length = done.length + todo.length := by rw [Hsplit, List.length_append]
  cases todo with
  | nil =>
    have hdoneall : done = entries := by simpa using Hsplit.symm
    have hvalue : value = entries.foldl GasPriceEntry.apply initial := by simpa using Htarget
    have hposfinal : pos = v.pos' + W64 (1 + payload.length + (entries.flatMap GasPriceEntry.bytes).length) := by
      rw [hdoneall] at Hposition; word
    have hleft : 0 < sint.Z (W64 (count - done.length)) := by rw [hdoneall]; word
    have hwire : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = wire := by simpa using Hremaining
    wp_apply wp_nextField_failure c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f gasFieldKeys gasFieldNames xs wire qf qo dq reason errorKind
      rfl hleft Hsearch gasFieldEncodings_nonempty gasFieldHeads_ordered gasCachedShortTable gasInlineFieldTable gasFieldNames_ascii failure Hpos hwire
      $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
    · iframe HerrorKind Hc Hst Hfields Horder Hs
    wp_for_post
    isimp only [hposfinal] at Hc Herr
    isimp only [hvalue] at Hchildren
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames Herr
    ipureintro
    rw [hdoneall] at Hposition
    exact ⟨by rw [← hposfinal]; exact Hposition, by rw [← hposfinal]; exact Hpos.2,
      by rw [← hposfinal]; exact hwire⟩
  | cons entry todo =>
    have hleft : 0 < sint.Z (W64 (count - done.length)) := by simp only [List.length_cons] at hlength; word
    have hleftstep : W64 (count - done.length) - W64 1 = W64 (count - (done ++ [entry]).length) := by
      simp only [List.length_cons] at hlength
      simp only [List.length_append, List.length_singleton]; word
    have hentry : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.bytes ++ (todo.flatMap GasPriceEntry.bytes ++ wire) := by
      simpa only [List.flatMap_cons, List.append_assoc] using Hremaining
    have Hadvance := input_consumption xs entry.bytes (todo.flatMap GasPriceEntry.bytes ++ wire) pos v.buf'.len Hpos Hlen.1 hentry
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = entry.key.bytes ++ (entry.valueBytes ++ (todo.flatMap GasPriceEntry.bytes ++ wire)) := by
      simpa only [GasPriceEntry.bytes, List.append_assoc] using hentry
    have hnewlen : ((done ++ [entry]).flatMap GasPriceEntry.bytes).length = (done.flatMap GasPriceEntry.bytes).length + entry.bytes.length := by simp
    have hbodypos : pos + W64 entry.key.bytes.length + W64 entry.valueBytes.length = pos + W64 entry.bytes.length := by
      simp only [GasPriceEntry.bytes, List.length_append]; word
    rcases Hfresh with ⟨hfresh, hrestfresh⟩
    wp_apply wp_nextGasKey c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (count - done.length)) next seen true) f xs
      (entry.valueBytes ++ (todo.flatMap GasPriceEntry.bytes ++ wire)) entry.key kind qf qo qn dq
      rfl hleft Hsearch hfresh Hpos hkey $$ [Hc Hst Hfields Horder Hnames Hs]
      with ⟨%next', Hc, Hst, Hfields, Horder, Hnames, Hs, %Hsearch', %Hkey⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    dsimp only at Hkey
    have hvaluepos : 0 ≤ sint.Z (pos + W64 entry.key.bytes.length) ∧ sint.Z (pos + W64 entry.key.bytes.length) ≤ sint.Z v.buf'.len :=
      ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
    isimp only [hleftstep] at Hst
    cases entry with
    | amount wei entryKey choice item =>
      cases wei with
      | false =>
        simp only [GasPriceEntry.key, choice, gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
        wp_auto
        have hread := wp_gasPrice_field_valid (GF := GF) c dst current false
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap GasPriceEntry.bytes ++ wire) item.value dq (d + 1)
          (by word) (by omega) item.valid hvaluepos Hkey.2.2
        simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
        · iframe Hdst Hc Hs
        ihave Hchildren := gasPriceChildren_set current value false p item.value $$ [Hchildren Hvalue]
        · iframe Hchildren Hvalue
        wp_for_post
        simp only [GasPriceEntry.key, GasPriceEntry.valueBytes] at hbodypos
        isimp only [hbodypos] at Hc
        isimp only [← choice] at Hst
        iframe
        iexists (done ++ [(.amount false entryKey choice item)]), todo, (gasPriceSet false current p), (gasPriceValueSet false value item.value),
          (pos + W64 (GasPriceEntry.bytes (.amount false entryKey choice item)).length), next', (gasKeySeen seen entryKey.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, GasPriceEntry.apply] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
      | true =>
        simp only [GasPriceEntry.key, choice, gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
        wp_auto
        have hread := wp_gasPrice_field_valid (GF := GF) c dst current true
          {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
          xs item.bytes (todo.flatMap GasPriceEntry.bytes ++ wire) item.value dq (d + 1)
          (by word) (by omega) item.valid hvaluepos Hkey.2.2
        simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
        · iframe Hdst Hc Hs
        ihave Hchildren := gasPriceChildren_set current value true p item.value $$ [Hchildren Hvalue]
        · iframe Hchildren Hvalue
        wp_for_post
        simp only [GasPriceEntry.key, GasPriceEntry.valueBytes] at hbodypos
        isimp only [hbodypos] at Hc
        isimp only [← choice] at Hst
        iframe
        iexists (done ++ [(.amount true entryKey choice item)]), todo, (gasPriceSet true current p), (gasPriceValueSet true value item.value),
          (pos + W64 (GasPriceEntry.bytes (.amount true entryKey choice item)).length), next', (gasKeySeen seen entryKey.choice)
        iframe
        ipureintro
        refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.foldl_cons, GasPriceEntry.apply] using Htarget
        · rw [Hadvance.1]; omega
        · rw [Hadvance.1, hnewlen]; omega
    | ignored entryKey choice entryWire valid =>
      simp only [GasPriceEntry.key, choice, gasKeyCode]
      wp_auto
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 entryKey.bytes.length, depth' := v.depth' + W64 1}
        xs entryWire (todo.flatMap GasPriceEntry.bytes ++ wire) dq (d + 1) (by word) (by omega) valid hvaluepos Hkey.2.2
        $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
      · iframe Hc Hs
      wp_for_post
      simp only [GasPriceEntry.key, GasPriceEntry.valueBytes] at hbodypos
      isimp only [hbodypos] at Hc
      isimp only [← choice] at Hst
      iframe
      iexists (done ++ [(.ignored entryKey choice entryWire valid)]), todo, current, value,
        (pos + W64 (GasPriceEntry.bytes (.ignored entryKey choice entryWire valid)).length), next', (gasKeySeen seen entryKey.choice)
      iframe
      ipureintro
      refine ⟨?_, hrestfresh, ?_, Hsearch', ⟨?_, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      · simpa only [List.append_assoc, List.singleton_append] using Hsplit
      · simpa only [List.foldl_cons, GasPriceEntry.apply] using Htarget
      · rw [Hadvance.1]; omega
      · rw [Hadvance.1, hnewlen]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_map_key_failure
