import ProofCheck.SegmentFieldValid
import ProofCheck.SegmentSeen
import ProofCheck.SegmentKeyRead
import ProofCheck.ClassOpen

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

theorem wp_readSegment_map_valid_of_array (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : SegmentValue) (v : cursor.Cursor)
    (xs payload body rem : List w8) (head : w8) (dq : DFrac) (d : Nat) (children : SegmentArrayValidContract (GF := GF) (d + 1)) (count : Nat)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (valid : SegmentPairsValid (d + 1) count [] old body rem final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       SegmentOwned dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengths)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; SegmentOwned dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hschema, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have Hentries := segmentPairs_count_bound (d + 1) count [] old final body rem valid
  have hfit : (count : Int) < 2 ^ 63 := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext
    word
  have Hfit : 2 * uint.Z (W64 count) ≤ (body ++ rem).length := by
    have castCount : uint.Z (W64 count) = (count : Int) := by word
    rw [castCount]
    simp only [List.length_append]
    omega
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, Hglobals⟩
  isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
  icases Hglobals with ⟨⟨_, _, #Hkind, _⟩, _⟩
  ihave #Htable := class_initialized_table (GF := GF) .segmentLengths $$ Hpkg
  isimp only [ClassTable, classGlobal, StaticFieldsShared] at Htable
  icases Htable with ⟨%fields, #Hglobal, %f, #Hfields, #Horder, #Hnames, %Hmask⟩
  wp_func_call
  wp_call
  wp_auto
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload (body ++ rem) head (W64 count) dq d enc major hd depth Hfit hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  ihave HI : (∃ (remaining : Nat) (seenFields : List SegmentField) (todo : List w8)
      (value : SegmentValue) (pos next mask : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) next mask true ∗
      "Howned" ∷ SegmentOwned dst value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hvalid" ∷ ⌜SegmentPairsValid (d + 1) remaining seenFields value todo rem final⌝ ∗
      "%Hseen" ∷ ⌜SegmentSeen mask seenFields⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hsearch" ∷ ⌜0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ segmentFieldKeys.length⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + 1 + payload.length + body.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF)
      $$ [s Howned Hc]
  · iexists count, [], body, old, (v.pos' + W64 (1 + payload.length)), (W64 0), (W64 0)
    iframe
    ipureintro
    exact ⟨valid, segmentSeen_zero, Nat.le_refl _, ⟨by decide, by simp [segmentFieldKeys, classKeys]⟩,
      ⟨by rw [Hopen.1]; omega, Hopen.2.1⟩, by rw [Hopen.1], Hopen.2.2.1⟩
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
    iframe Howned Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext
  | @step _ n seenFields choice initial middle target key valueBytes tail suffix keyValid fresh valueValid rest =>
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
    have hfresh : SegmentKeyFresh mask choice := by
      cases choice with
      | none => trivial
      | some wei => exact (Hseen wei).mpr fresh
    isimp only [Int.natCast_add, Int.natCast_one] at Hst
    wp_apply wp_nextSegmentKey_valid validator c s_ptr fields {v with pos' := pos, depth' := v.depth' + W64 1}
      (cursor.Struct.mk fields (W64 (n + 1)) next mask true) f xs (valueBytes ++ (tail ++ rem))
      key choice kinds.unsupportedKind DFrac.discard DFrac.discard DFrac.discard dq rfl hleft Hsearch keyValid hfresh Hpos hkey
      $$ [Hc Hst Hs]
      with ⟨%next', Hc, Hst, _, _, _, Hs, %Hsearch', %Hkey⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    dsimp only at Hkey
    have hvaluepos : 0 ≤ sint.Z (pos + W64 key.length) ∧ sint.Z (pos + W64 key.length) ≤ sint.Z v.buf'.len :=
      ⟨by rw [Hkey.1]; omega, Hkey.2.1⟩
    isimp only [hleftstep] at Hst
    cases valueValid with
    | @selected _ field _ _ _ _ fieldValid =>
      have hread := wp_segment_field_valid_of_array (GF := GF) (d + 1) children c dst value middle field
        {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) dq fieldValid (by word) (by omega) hvaluepos Hkey.2.2
      cases field
      all_goals
        simp only [segmentKeyCode, segmentKeyIndex]
        wp_auto
        simp only [segmentFieldReader, segmentFieldName] at hread
        wp_apply hread $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hbody⟩
        · iframe Howned Hc Hs
        wp_for_post
        isimp only [hbodypos] at Hc
        iframe
        iexists n, (segmentChoiceSeen seenFields _), tail, middle,
          (pos + W64 (key ++ valueBytes).length), next', (segmentKeySeen mask _)
        iframe
        ipureintro
        refine ⟨rest, segmentSeen_mark mask seenFields _ Hseen, by omega, Hsearch',
          ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
        simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
        omega
    | ignored skipValid =>
      simp only [segmentKeyCode]
      wp_auto
      wp_apply wp_Skip_valid c {v with pos' := pos + W64 key.length, depth' := v.depth' + W64 1}
        xs valueBytes (tail ++ rem) dq (d + 1) (by word) (by omega) skipValid hvaluepos Hkey.2.2
        $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
      · iframe Hc Hs
      wp_for_post
      isimp only [hbodypos] at Hc
      iframe
      iexists n, (segmentChoiceSeen seenFields none), tail, value,
        (pos + W64 (key ++ valueBytes).length), next', (segmentKeySeen mask none)
      iframe
      ipureintro
      refine ⟨rest, segmentSeen_mark mask seenFields none Hseen, by omega, Hsearch',
        ⟨by rw [Hadvance.1]; omega, Hadvance.2.1⟩, ?_, Hadvance.2.2⟩
      simp only [List.length_append, Int.natCast_add] at Hposition Hadvance ⊢
      omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegment_map_valid_of_array
