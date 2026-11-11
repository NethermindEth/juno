import ProofCheck.HeaderProjectionBoundary
import ProofCheck.HeaderProjectionFieldComplete
import ProofCheck.HeaderProjectionScanDecompose
import ProofCheck.ProjectionScanValid

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

/-- The actual hash-projection map loop for every valid supported pair sequence,
    arbitrary existing owned value, unknown/duplicate fields and unread suffix.
    Key decoding retains the explicit production UTF-8 validator dependency. -/
theorem wp_readHeaderHashProjection_map_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : HeaderProjectionValue) (v : cursor.Cursor)
    (xs payload body rem : List w8) (head : w8) (dq : DFrac) (d count : Nat)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (valid : HeaderProjectionPairsValid (d + 1) .hash count [] old body final)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       HeaderProjectionOwned .hash dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeaderHashProjection)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; HeaderProjectionOwned .hash dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + body.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + body.length)) = sint.Z v.pos' + 1 + payload.length + body.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + body.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hschema, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Htotal := input_consumption xs ((head :: payload) ++ body) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have Hentries := headerProjectionPairs_count_bound (d + 1) count .hash [] old final body valid
  have HcountBound : count < 2^63 := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Htotal
    word
  have Hfit : 2 * uint.Z (W64 count) ≤ (body ++ rem).length := by
    have castCount : uint.Z (W64 count) = (count : Int) := by word
    rw [castCount]
    simp only [List.length_append]
    omega
  have plain : ¬ projectionHasTag ((head :: payload) ++ body ++ rem) := by
    rintro ⟨other, tail, same, tagged⟩
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    rw [major] at tagged
    contradiction
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ Hschema
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave #Htruncated : (globalAddr cursor.ErrTruncated ↦□ kinds.truncatedKind : IProp GF) $$ []
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, #Htruncated, _, _⟩, _⟩
    iexact Htruncated
  ihave #Htable := projection_initialized_table (GF := GF) .headerHash $$ Hpkg
  isimp only [ProjectionTable, projectionGlobal] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_projectionTag_untagged c v xs ((head :: payload) ++ body ++ rem) dq kinds.truncatedKind plain hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Htruncated Hc Hs
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload (body ++ rem) head (W64 count) dq d enc major hd depth Hfit hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  ihave HI : (∃ (remaining : Nat) (seen : List Nat) (todo : List w8) (value : HeaderProjectionValue) (pos mask : w64),
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) (W64 0) mask true ∗
      "Howned" ∷ HeaderProjectionOwned .hash dst value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hvalid" ∷ ⌜HeaderProjectionPairsValid (d + 1) .hash remaining seen value todo final⌝ ∗
      "%Hseen" ∷ ⌜WantedSeen (projectionNames .headerHash).length mask seen⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos + todo.length = sint.Z v.pos' + 1 + payload.length + body.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo ++ rem⌝ : IProp GF) $$ [s Howned Hc]
  · iexists count, [], body, old, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨valid, wantedSeen_zero _, Nat.le_refl _, ⟨by rw [Hopen.1]; omega, Hopen.2.1⟩, by rw [Hopen.1], Hopen.2.2.1⟩
  wp_for HI
  rcases headerProjectionPairs_scan (d + 1) remaining .hash seen value final todo Hvalid with
    ⟨Hscan, sameValue⟩ | ⟨scanned, valueBytes, tail, index, left, field, middle, split, Hscan, lookup, fieldValid, restValid⟩
  · wp_apply wp_nextWanted_projection_valid validator .headerHash c s_ptr
      {v with pos' := pos, depth' := v.depth' + W64 1} (cursor.Struct.mk fields (W64 remaining) (W64 0) mask true)
      seen xs todo rem dq (d + 1) remaining .done rfl (by omega) Hseen (by word) (by omega) Hscan Hpos Hremaining
      $$ [Hc Hst Hs] with ⟨Hc, Hst, Hs, %Hnext⟩
    · isimp only [projectionGlobal]
      iframe Hfields Hc Hst Hs
    have hfinal : pos + W64 todo.length = v.pos' + W64 (1 + payload.length + body.length) := by word
    have hdepth : v.depth' + W64 1 - W64 1 = v.depth' := by word
    simp only [wantedCode]
    wp_for_post
    isimp only [wantedCursor, wantedStruct, ite_true, hdepth, hfinal] at Hc
    iapply HΦ
    isimp only [sameValue]
    iframe Howned Hc Hs
    ipureintro
    simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Htotal
  · have selected := wantedValid_selected (d + 1) remaining index left (headerProjectionNames .hash) seen scanned Hscan
    have indexZero : index = 0 := by
      simp only [headerProjectionNames, headerProjectionSchema, projectionNames, List.length_cons, List.length_nil] at selected
      omega
    subst index
    simp only [headerProjectionFields, List.getElem?_cons_zero, Option.some.injEq] at lookup
    subst field
    have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = scanned ++ (valueBytes ++ tail ++ rem) := by
      rw [Hremaining, split]
      simp only [List.append_assoc]
    wp_apply wp_nextWanted_projection_valid validator .headerHash c s_ptr
      {v with pos' := pos, depth' := v.depth' + W64 1} (cursor.Struct.mk fields (W64 remaining) (W64 0) mask true)
      seen xs scanned (valueBytes ++ tail ++ rem) dq (d + 1) remaining (.field 0 left) rfl (by omega) Hseen
      (by word) (by omega) Hscan Hpos hkey $$ [Hc Hst Hs] with ⟨Hc, Hst, Hs, %Hkey⟩
    · isimp only [projectionGlobal]
      iframe Hfields Hc Hst Hs
    isimp only [wantedCursor, wantedStruct] at Hc Hst
    simp only [wantedCode]
    wp_auto
    have hread := wp_headerProjectionField_valid (GF := GF) (d + 1) .hash .hash c dst value middle
      {v with pos' := pos + W64 scanned.length, depth' := v.depth' + W64 1}
      xs valueBytes (tail ++ rem) dq (by simp [headerProjectionFields]) fieldValid (by word) (by omega)
      ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩
      (by simpa only [List.append_assoc] using Hkey.2.2)
    simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName] at hread
    wp_apply hread $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hvalue⟩
    · iframe Howned Hc Hs
    wp_for_post
    iframe
    iexists left, (0 :: seen), tail, middle, (pos + W64 scanned.length + W64 valueBytes.length), (mask ||| (W64 1 <<< W64 0))
    iframe
    ipureintro
    dsimp only at Hkey Hvalue
    refine ⟨restValid, wantedSeen_mark _ 0 mask seen (projectionNames_small .headerHash) selected.1 Hseen, by omega,
      ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, ?_, Hvalue.2.2⟩
    rw [split] at Hposition
    simp only [List.length_append, Int.natCast_add] at Hposition
    omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderHashProjection_map_valid
