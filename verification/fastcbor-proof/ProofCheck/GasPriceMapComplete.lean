import ProofCheck.SchemaKeysComplete
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

/-- The actual GasPrice loop realizes the complete successful pair specification,
    including all key encodings, arbitrary order and nested ignored values.
    Only the unresolved production validator contract is supplied as a premise. -/
theorem wp_readGasPrice_map_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst fields : Loc) (g : GasPrice) (initial : GasPriceValue)
    (v : cursor.Cursor) (f : cursor.Fields) (xs payload body rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac) (d count : Nat) (final : GasPriceValue)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (valid : GasPairsValid (d + 1) count [] initial body final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr internal.cborschema.cborGasPriceFields ↦□ fields ∗ globalAddr cursor.ErrUnsupported ↦□ kind ∗
       dst ↦ g ∗ GasPriceChildren g initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames }}
      (App (App (Val (@! cborReadGasPrice)) (Val #c)) (Val #dst))
    {{ (out : GasPrice), RET #interface.nil;
       dst ↦ out ∗ GasPriceChildren out final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗
       v.buf' ↦*{dq} xs ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} gasFieldKeys ∗ f.names' ↦*{qn} gasFieldNames ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) =
           sint.Z v.pos' + 1 + payload.length + body.length ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨#Hglobal, #Hkind, Hdst, Hchildren, Hc, Hs, Hfields, Horder, Hnames⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hentries := gasPairs_count_bound (d + 1) count [] initial final body valid
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
  ihave HI : (∃ (remaining : Nat) (seenFields : List Bool) (todo : List w8)
      (current : GasPrice) (value : GasPriceValue) (pos next mask : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Hdst" ∷ dst ↦ current ∗ "Hchildren" ∷ GasPriceChildren current value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hvalid" ∷ ⌜GasPairsValid (d + 1) remaining seenFields value todo final⌝ ∗
      "%Hseen" ∷ ⌜GasSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ gasFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + 1 + payload.length + body.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF)
      $$ [s Hdst Hchildren Hc]
  · iexists count, [], body, g, initial, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe
    ipureintro
    exact ⟨valid, gasSeen_zero, Nat.le_refl _, ⟨by decide, by simp [gasFieldKeys]⟩,
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
    iapply HΦ
    iframe Hdst Hchildren Hc Hs Hfields Horder Hnames
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
    have hfresh : GasKeyFresh mask choice := by
      cases choice with
      | none => trivial
      | some wei => exact (Hseen wei).mpr fresh
    isimp only [Int.natCast_add, Int.natCast_one] at Hst
    wp_apply wp_nextGasKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
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
    | @amount wei old valueWire feltValue feltValid =>
      cases wei with
      | false =>
        simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
        wp_auto
        have hread := wp_gasPrice_field_valid (GF := GF) c dst current false
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes (tail ++ rem) feltValue dq (d + 1)
          (by word) (by omega) feltValid hvaluepos Hkey.2.2
        simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
        · iframe Hdst Hc Hs
        ihave Hchildren := gasPriceChildren_set current value false p feltValue $$ [Hchildren Hvalue]
        · iframe Hchildren Hvalue
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        iexists n, (gasChoiceSeen seenFields (some false)), tail, (gasPriceSet false current p),
          (gasPriceValueSet false value feltValue), (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask (some false))
        iframe
        ipureintro
        refine ⟨rest, gasSeen_mark mask seenFields (some false) Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
      | true =>
        simp only [gasKeyCode, gasKeyIndex, Bool.false_eq_true, ite_false, ite_true]
        wp_auto
        have hread := wp_gasPrice_field_valid (GF := GF) c dst current true
          {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
          xs valueBytes (tail ++ rem) feltValue dq (d + 1)
          (by word) (by omega) feltValid hvaluepos Hkey.2.2
        simp only [gasPriceFieldName, Bool.false_eq_true, ite_false, ite_true] at hread
        wp_apply hread $$ [Hdst Hc Hs] as %p ⟨Hdst, Hvalue, Hc, Hs, %Hbody⟩
        · iframe Hdst Hc Hs
        ihave Hchildren := gasPriceChildren_set current value true p feltValue $$ [Hchildren Hvalue]
        · iframe Hchildren Hvalue
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        iexists n, (gasChoiceSeen seenFields (some true)), tail, (gasPriceSet true current p),
          (gasPriceValueSet true value feltValue), (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask (some true))
        iframe
        ipureintro
        refine ⟨rest, gasSeen_mark mask seenFields (some true) Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | ignored skipValid =>
      simp only [gasKeyCode]
      wp_auto
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
        $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
      · iframe Hc Hs
      wp_for_post
      isimp only [hbodypos] at Hc
      iframe
      iexists n, (gasChoiceSeen seenFields none), tail, current, value,
        (pos + W64 (key ++ valueBytes).length), next', (gasKeySeen mask none)
      iframe
      ipureintro
      refine ⟨rest, gasSeen_mark mask seenFields none Hseen, by omega, Hsearch',
        ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readGasPrice_map_valid
