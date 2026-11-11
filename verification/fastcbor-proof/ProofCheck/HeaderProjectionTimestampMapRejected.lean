import ProofCheck.ProjectionTagSuccess
import ProofCheck.HeaderProjectionBounds
import ProofCheck.HeaderProjectionBoundary
import ProofCheck.HeaderProjectionFieldComplete
import ProofCheck.HeaderProjectionPairError
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

/-- After any independently accepted optional tag prelude, the actual
    timestamp-projection map loop on every rejected pair sequence.
    Earlier successful fields remain owned; the retained cursor and diagnostic
    positions are exact, including failures after ignored or duplicate fields. -/
theorem wp_readHeaderTimestampProjection_map_rejected_after_tag (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : HeaderProjectionValue) (v : cursor.Cursor)
    (xs payload body : List w8) (head : w8) (dq : DFrac) (d count consumed offset : Nat)
    (reason : HeaderProjectionFailure) (kinds : FeltErrorKinds)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (fit : 2 * count ≤ body.length)
    (failure : HeaderProjectionPairsRejected (d + 1) .timestamp count [] old body final consumed offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body)
    (initial : cursor.Cursor) (wire : List w8) (tagConsumed : Nat)
    (tag : ProjectionTagSpec wire tagConsumed ((head :: payload) ++ body) .success)
    (initialPos : 0 ≤ sint.Z initial.pos' ∧ sint.Z initial.pos' ≤ sint.Z initial.buf'.len)
    (initialBytes : subslice (sint.nat initial.pos') (sint.nat initial.buf'.len) xs = wire)
    (advance : {initial with pos' := initial.pos' + W64 tagConsumed} = v) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ HeaderProjectionOwned .timestamp dst old ∗ c ↦ initial ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeaderTimestampProjection)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionOwned .timestamp dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       HeaderProjectionPairError err (v.pos' + W64 (1 + payload.length + offset)) reason (headerProjectionPairFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  iintro %Φ ⟨#Hpkg, #Hschema, #Hglobals, Howned, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Htotal := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_nil] using hbytes)
  have HcountBound : count < 2^63 := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Htotal
    word
  have Hfit : 2 * uint.Z (W64 count) ≤ body.length := by
    have castCount : uint.Z (W64 count) = (count : Int) := by word
    rw [castCount]
    omega
  ihave #Htable := projection_initialized_table (GF := GF) .headerTimestamp $$ Hpkg
  isimp only [ProjectionTable, projectionGlobal] at Htable
  icases Htable with ⟨%fields, #Hfields, _⟩
  wp_func_call
  wp_call
  wp_auto
  have sameBuf : initial.buf' = v.buf' := by rw [← advance]
  wp_apply wp_projectionTag_success c initial xs wire ((head :: payload) ++ body) dq tagConsumed kinds tag initialPos initialBytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Htag⟩
  · isimp only [sameBuf]
    iframe Hglobals Hc Hs
  isimp only [advance] at Hc
  isimp only [sameBuf] at Hs
  wp_func_call
  wp_call
  wp_auto
  wp_apply wp_openStruct_map c fields v xs payload body head (W64 count) dq d enc major hd depth Hfit hp
    (by simpa only [List.append_assoc] using hbytes) $$ [Hc Hs] with ⟨Hc, Hs, %Hopen⟩
  · iframe Hc Hs
  have Hbounds := headerProjectionPairsRejected_bounds (d + 1) count consumed offset .timestamp [] old final body reason failure
  have Hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z v.buf'.len ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Htotal
    exact ⟨by word, by word, by word, by word⟩
  ihave HI : ((∃ (remaining tailConsumed tailOffset : Nat) (seen : List Nat) (todo : List w8)
      (value : HeaderProjectionValue) (pos mask : w64),
      "Herrslot" ∷ err_ptr ↦ interface.nil ∗
      "Hst" ∷ s_ptr ↦ cursor.Struct.mk fields (W64 remaining) (W64 0) mask true ∗
      "Howned" ∷ HeaderProjectionOwned .timestamp dst value ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hfailure" ∷ ⌜HeaderProjectionPairsRejected (d + 1) .timestamp remaining seen value todo final tailConsumed tailOffset reason⌝ ∗
      "%Hseen" ∷ ⌜WantedSeen (projectionNames .headerTimestamp).length mask seen⌝ ∗
      "%Hcount" ∷ ⌜remaining ≤ count⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hconsumed" ∷ ⌜sint.Z pos + tailConsumed = sint.Z v.pos' + 1 + payload.length + consumed⌝ ∗
      "%Hoffset" ∷ ⌜sint.Z pos + tailOffset = sint.Z v.pos' + 1 + payload.length + offset⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = todo⌝) ∨
      (∃ (errorLoc : Loc),
        err_ptr ↦ interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∗
        HeaderProjectionOwned .timestamp dst final ∗
        c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1} ∗
        HeaderProjectionPairError errorLoc (v.pos' + W64 (1 + payload.length + offset)) reason
          (headerProjectionPairFailureKind reason kinds)) : IProp GF) $$ [err s Howned Hc]
  · ileft
    iexists count, consumed, offset, [], body, old, (v.pos' + W64 (1 + payload.length)), (W64 0)
    iframe
    ipureintro
    exact ⟨failure, wantedSeen_zero _, Nat.le_refl _, ⟨by rw [Hopen.1]; omega, Hopen.2.1⟩,
      by rw [Hopen.1], by rw [Hopen.1], Hopen.2.2.1⟩
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
    cases headerProjectionPairs_rejected_scan (d + 1) remaining tailConsumed tailOffset .timestamp seen value final todo reason Hfailure with
    | @scan todo tailConsumed tailOffset left scanReason rejected =>
      wp_apply wp_nextWanted_projection_rejected validator .headerTimestamp c s_ptr
        {v with pos' := pos, depth' := v.depth' + W64 1} (cursor.Struct.mk fields (W64 remaining) (W64 0) mask true)
        seen xs todo dq (d + 1) remaining tailConsumed tailOffset left scanReason kinds rfl (by omega) Hseen
        (by word) (by omega) rejected Hpos Hremaining $$ [Hc Hst Hs] as %err ⟨Hc, Hst, Hs, Herr, %Hnext⟩
      · isimp only [projectionGlobal]
        iframe Hglobals Hfields Hc Hst Hs
      have hfinal : pos + W64 tailConsumed = v.pos' + W64 (1 + payload.length + consumed) := by word
      have herror : pos + W64 tailOffset = v.pos' + W64 (1 + payload.length + offset) := by word
      wp_for_post
      isimp only [hfinal] at Hc
      isimp only [herror] at Herr
      iapply HΦ
      isimp only [headerProjectionPairError_scan]
      iframe Howned Hc Hs Herr
      ipureintro
      exact Hprogress
    | @field scanned tail index left field innerOffset childReason scan lookup rejected =>
      have selected := wantedValid_selected (d + 1) remaining index left (headerProjectionNames .timestamp) seen scanned scan
      have indexZero : index = 0 := by
        simp only [headerProjectionNames, headerProjectionSchema, projectionNames, List.length_cons, List.length_nil] at selected
        omega
      subst index
      simp only [headerProjectionFields, List.getElem?_cons_zero, Option.some.injEq] at lookup
      subst field
      wp_apply wp_nextWanted_projection_valid validator .headerTimestamp c s_ptr
        {v with pos' := pos, depth' := v.depth' + W64 1} (cursor.Struct.mk fields (W64 remaining) (W64 0) mask true)
        seen xs scanned tail dq (d + 1) remaining (.field 0 left) rfl (by omega) Hseen
        (by word) (by omega) scan Hpos Hremaining $$ [Hc Hst Hs] with ⟨Hc, Hst, Hs, %Hkey⟩
      · isimp only [projectionGlobal]
        iframe Hfields Hc Hst Hs
      isimp only [wantedCursor, wantedStruct] at Hc Hst
      simp only [wantedCode]
      wp_auto
      ihave #Hcategory := headerProjectionFieldErrorGlobals_lookup kinds childReason $$ Hglobals
      have hread := wp_headerProjectionField_rejected (GF := GF) (d + 1) innerOffset .timestamp .timestamp c dst final
        {v with pos' := pos + W64 scanned.length, depth' := v.depth' + W64 1} xs tail dq childReason
        (headerProjectionFieldFailureKind childReason kinds) (by simp [headerProjectionFields]) rejected
        (by word) (by omega) ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName] at hread
      wp_apply hread $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Herror⟩
      · iframe Hcategory Howned Hc Hs
      have hfinal : pos + W64 scanned.length = v.pos' + W64 (1 + payload.length + consumed) := by word
      have herror : pos + W64 scanned.length + W64 innerOffset = v.pos' + W64 (1 + payload.length + offset) := by
        simp only [Int.natCast_add] at Hoffset
        word
      wp_for_post
      isimp only [hfinal] at Hc
      isimp only [herror] at Herr
      iframe
      iright
      iexists err
      isimp only [HeaderProjectionPairError, headerProjectionPairFailureKind]
      iframe Herrslot Howned Hc Herr
    | @next middle final scanned valueBytes tail index left field tailConsumed tailOffset reason scan lookup valid rejected =>
      have selected := wantedValid_selected (d + 1) remaining index left (headerProjectionNames .timestamp) seen scanned scan
      have indexZero : index = 0 := by
        simp only [headerProjectionNames, headerProjectionSchema, projectionNames, List.length_cons, List.length_nil] at selected
        omega
      subst index
      simp only [headerProjectionFields, List.getElem?_cons_zero, Option.some.injEq] at lookup
      subst field
      wp_apply wp_nextWanted_projection_valid validator .headerTimestamp c s_ptr
        {v with pos' := pos, depth' := v.depth' + W64 1} (cursor.Struct.mk fields (W64 remaining) (W64 0) mask true)
        seen xs scanned (valueBytes ++ tail) dq (d + 1) remaining (.field 0 left) rfl (by omega) Hseen
        (by word) (by omega) scan Hpos (by simpa only [List.append_assoc] using Hremaining)
        $$ [Hc Hst Hs] with ⟨Hc, Hst, Hs, %Hkey⟩
      · isimp only [projectionGlobal]
        iframe Hfields Hc Hst Hs
      isimp only [wantedCursor, wantedStruct] at Hc Hst
      simp only [wantedCode]
      wp_auto
      have hread := wp_headerProjectionField_valid (GF := GF) (d + 1) .timestamp .timestamp c dst value middle
        {v with pos' := pos + W64 scanned.length, depth' := v.depth' + W64 1}
        xs valueBytes tail dq (by simp [headerProjectionFields]) valid (by word) (by omega)
        ⟨by dsimp only at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
      simp only [headerProjectionFieldReader, headerProjectionFieldRef, headerProjectionFieldName] at hread
      wp_apply hread $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hvalue⟩
      · iframe Howned Hc Hs
      wp_for_post
      iframe
      ileft
      iexists left, tailConsumed, tailOffset, (0 :: seen), tail, middle,
        (pos + W64 scanned.length + W64 valueBytes.length), (mask ||| (W64 1 <<< W64 0))
      iframe
      ipureintro
      dsimp only at Hkey Hvalue
      simp only [Int.natCast_add] at Hconsumed Hoffset
      exact ⟨rejected, wantedSeen_mark _ 0 mask seen (projectionNames_small .headerTimestamp) selected.1 Hseen, by omega,
        ⟨by rw [Hvalue.1, Hkey.1]; omega, Hvalue.2.1⟩, by omega, by omega, Hvalue.2.2⟩
  · icases Hfailed with ⟨%err, Herrslot, Howned, Hc, Herr⟩
    wp_auto
    cleanup_bool_decide
    try wp_auto
    cleanup_bool_decide
    try wp_auto
    cleanup_bool_decide
    try wp_auto
    simp
    iapply HΦ
    iframe Howned Hc Hs Herr
    ipureintro
    simpa using Hprogress

/-- The actual timestamp-projection map loop on every rejected pair sequence.
    Earlier successful fields remain owned; the retained cursor and diagnostic
    positions are exact, including failures after ignored or duplicate fields. -/
theorem wp_readHeaderTimestampProjection_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old final : HeaderProjectionValue) (v : cursor.Cursor)
    (xs payload body : List w8) (head : w8) (dq : DFrac) (d count consumed offset : Nat)
    (reason : HeaderProjectionFailure) (kinds : FeltErrorKinds)
    (enc : HeadArgument head payload (W64 count)) (major : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (depth : d < 32)
    (fit : 2 * count ≤ body.length)
    (failure : HeaderProjectionPairsRejected (d + 1) .timestamp count [] old body final consumed offset reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborprojections ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ HeaderProjectionOwned .timestamp dst old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadHeaderTimestampProjection)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       HeaderProjectionOwned .timestamp dst final ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := v.depth' + W64 1} ∗ v.buf' ↦*{dq} xs ∗
       HeaderProjectionPairError err (v.pos' + W64 (1 + payload.length + offset)) reason (headerProjectionPairFailureKind reason kinds) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z v.buf'.len ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  have plain : ¬ projectionHasTag ((head :: payload) ++ body) := by
    rintro ⟨other, tail, same, tagged⟩
    simp only [List.cons_append, List.cons.injEq] at same
    obtain ⟨rfl, _⟩ := same
    rw [major] at tagged
    contradiction
  wp_start_folded as ⟨Hglobals, Howned, Hc, Hs⟩
  wp_apply wp_readHeaderTimestampProjection_map_rejected_after_tag validator c dst old final v xs payload body head dq d count consumed offset reason kinds enc major hd depth fit failure hp hbytes
    v ((head :: payload) ++ body) 0 (.untagged plain) hp hbytes
    (by have hz : v.pos' + W64 0 = v.pos' := by word
        simp only [Int.natCast_zero, hz])
    $$ [Hglobals Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Herr, %Hnext⟩
  · iframe
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderTimestampProjection_map_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readHeaderTimestampProjection_map_rejected_after_tag
