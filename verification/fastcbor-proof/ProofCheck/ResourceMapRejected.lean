import ProofCheck.ResourceMapError
import ProofCheck.ResourceBoundsRecordComplete
import ProofCheck.ResourceMapEntriesSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 4096

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

/-- The actual map loop preserves the old destination on every entry failure,
    retaining private completed entries and any failed partial record. -/
theorem wp_readResourceMap_map_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (old : Loc) (v : cursor.Cursor) (xs payload body : List w8)
    (head : w8) (dq : DFrac) (d count consumed offset : Nat)
    (doneValues : List (w32 × ResourceBoundsValue)) (retained : ResourceBoundsValue)
    (entered : Bool) (reason : ResourceMapEntryFailureReason)
    (failure : ResourceMapEntriesRejected (d + 1) count body doneValues retained consumed offset entered reason)
    (enc : HeadArgument head payload (W64 count)) (hm : head &&& W8 224 = W8 160)
    (hd : sint.Z v.depth' = d) (hdepth : d < 32) (fit : 2 * count ≤ body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ body) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cbortransactions ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadResourceBoundsMap)) (Val #c)) (Val #dst))
    {{ (result : GoInterface), RET #result;
       dst ↦ old ∗ ResourceMapPartial doneValues retained reason ∗
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + consumed), depth' := if entered then v.depth' + W64 1 + W64 1 else v.depth' + W64 1} ∗
       v.buf' ↦*{dq} xs ∗ ResourceMapEntryError result (v.pos' + W64 (1 + payload.length + offset)) reason ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
         sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
         sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := header_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  ihave %HunsupportedNonNil := projection_unsupported_nonNil kinds $$ [Hglobals]
  · iframe Hglobals
    solve_pkg_init
  ihave %HtypeNonNil := cursor_type_nonNil kinds $$ [Hglobals]
  · iframe Hglobals
    solve_pkg_init
  ihave #Hunsupported : (globalAddr cursor.ErrUnsupported ↦□ kinds.unsupportedKind : IProp GF) $$ [Hglobals]
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨⟨_, _, Hkind, _⟩, _⟩
    iexact Hkind
  ihave #Htype : (globalAddr cursor.ErrType ↦□ kinds.typeKind : IProp GF) $$ [Hglobals]
  · isimp only [FeltErrorGlobals, SkipErrorGlobals] at Hglobals
    icases Hglobals with ⟨_, Hkind⟩
    iexact Hkind
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ body) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have htotal : sint.Z v.pos' + 1 + payload.length + body.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have allBounds := resourceMapEntriesRejected_bounds _ _ _ _ _ _ _ _ _ failure
  have Hprogress : sint.Z (v.pos' + W64 (1 + payload.length + consumed)) = sint.Z v.pos' + 1 + payload.length + consumed ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) = sint.Z v.pos' + 1 + payload.length + offset ∧
      sint.Z (v.pos' + W64 (1 + payload.length + consumed)) ≤ sint.Z (v.pos' + W64 (1 + payload.length + offset)) ∧
      sint.Z (v.pos' + W64 (1 + payload.length + offset)) ≤ sint.Z v.buf'.len := by
    constructor; · word
    constructor; · word
    constructor <;> word
  have hfit : (count : Int) < 2 ^ 63 := by word
  have hcount : 0 ≤ sint.Z (W64 count) := by word
  have hi := input_suffix_head xs (payload ++ body)
    (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 246 &&& W8 224 ≠ W8 160) hc
  have hu : head ≠ W8 247 := by
    intro h
    have hc := hm
    rw [h] at hc
    exact (by decide : W8 247 &&& W8 224 ≠ W8 160) hc
  have hp1 : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs =
      payload ++ body := by
    rw [hp1]
    exact input_suffix_after xs [head] _ _ _ (by simpa only [List.cons_append, List.nil_append, List.append_assoc] using hbytes)
  wp_auto
  wp_apply wp_null_other c v xs head dq hp.1 hi hn hu $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_MapLen_encoded c v xs payload body head (W64 count) dq
    enc hm hp.1 (by omega) (by omega) (by word) hi hafter $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply (wp_map_make2 (K := w32) (V := ResourceBounds) (W64 count)) with %out Hmap
  irename count => Hcount
  ihave Hchildren : (ResourceMapChildren (∅ : GMap w32 ResourceBounds) ∅ : IProp GF) $$ []
  · iapply resourceMapChildren_empty
  have Hhead := input_consumption xs (head :: payload) body
    v.pos' v.buf'.len hp Hlen.1 (by simpa only [List.append_assoc] using hbytes)
  have hstart : sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hhead
    word
  have hremaining : subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = body := by
    simpa only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm] using Hhead.2.2
  ihave HI : (∃ (done todo : List (w32 × ResourceBoundsValue)) (stored : GMap w32 ResourceBounds)
      (front tail : List w8) (leftConsumed leftOffset : Nat) (pos : w64),
      "Hcount" ∷ count_ptr ↦ W64 (count - done.length : Nat) ∗
      "Hmap" ∷ out ↦$ stored ∗
      "Hchildren" ∷ ResourceMapChildren stored (resourceMapOfEntries done) ∗
      "Hc" ∷ c ↦ {v with pos' := pos, depth' := v.depth' + W64 1} ∗
      "%Hsplit" ∷ ⌜doneValues = done ++ todo⌝ ∗
      "%Hwire" ∷ ⌜body = front ++ tail⌝ ∗
      "%Hfailure" ∷ ⌜ResourceMapEntriesRejected (d + 1) (count - done.length) tail todo retained leftConsumed leftOffset entered reason⌝ ∗
      "%Hconsumed" ∷ ⌜consumed = front.length + leftConsumed⌝ ∗
      "%Hoffset" ∷ ⌜offset = front.length + leftOffset⌝ ∗
      "%Hpos" ∷ ⌜0 ≤ sint.Z pos ∧ sint.Z pos ≤ sint.Z v.buf'.len⌝ ∗
      "%Hposition" ∷ ⌜sint.Z pos = sint.Z v.pos' + 1 + payload.length + front.length⌝ ∗
      "%Hremaining" ∷ ⌜subslice (sint.nat pos) (sint.nat v.buf'.len) xs = tail⌝ : IProp GF)
      $$ [Hcount Hmap Hchildren Hc]
  · iexists [], doneValues, ∅, [], body, consumed, offset, (v.pos' + W64 (1 + payload.length))
    isimp only [List.length_nil, List.nil_append, Nat.sub_zero, resourceMapOfEntries, resourceMapApply, List.foldl_nil]
    iframe
    ipureintro
    exact ⟨trivial, trivial, failure, by simp, by simp, ⟨by word, by word⟩, by simpa using hstart, hremaining⟩
  wp_for HI
  have bounds := resourceMapEntriesRejected_bounds _ _ _ _ _ _ _ _ _ Hfailure
  have hdone : done.length < count := by omega
  have hcond : sint.Z (W64 0) < sint.Z (W64 (count - done.length : Nat)) := by word
  simp only [hcond, _root_.decide_true, ite_true]
  wp_auto
  generalize hleft : count - done.length = left at Hfailure
  cases Hfailure with
  | @here n wire retained leftConsumed leftOffset entered reason fail =>
    cases fail with
    | key keyFailure =>
      cases keyFailure with
      | @nullish keyHead keyTail nullish =>
        have hi := input_suffix_head xs keyTail (sint.nat pos) (sint.nat v.buf'.len) keyHead Hremaining
        wp_apply wp_null_at c {v with pos' := pos, depth' := v.depth' + W64 1}
          xs keyHead dq Hpos.1 hi nullish $$ [Hc Hs] with ⟨Hc, Hs⟩
        · iframe Hc Hs
        ihave Herr : (ResourceMapEntryError kinds.unsupportedKind (pos + W64 1) (.key .nullish) : IProp GF) $$ []
        · unfold ResourceMapEntryError
          iexists kinds
          iframe Hglobals
          ipureintro; exact ⟨rfl, HunsupportedNonNil⟩
        wp_for_post
        have hposfinal : pos + W64 1 = v.pos' + W64 (1 + payload.length + consumed) := by word
        have hoffsetfinal : pos + W64 1 = v.pos' + W64 (1 + payload.length + offset) := by word
        have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
        isimp only [hposfinal] at Hc
        isimp only [hoffsetfinal] at Herr
        isimp only [hdoneall, Bool.false_eq_true, ite_false] at HΦ
        iapply HΦ
        iframe Hdst Hc Hs Herr
        isplitl [Hmap Hchildren]
        · unfold ResourceMapPartial
          isplitl [Hmap Hchildren]
          · iexists out
            unfold ResourceMapOwned
            iexists stored
            iframe Hmap Hchildren
          · ipureintro; rfl
        · ipureintro; exact Hprogress
      | numeric numericFailure =>
        cases numericFailure with
        | expected parseFailure =>
          wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
            xs tail dq parseFailure.1 parseFailure.2.1 Hpos Hremaining $$ [Hc Hs] with ⟨Hc, Hs⟩
          · iframe Hc Hs
          ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 0) _) $$ Hglobals
          isimp only [feltFailureAddr, feltFailureKind] at Hkind
          wp_apply wp_uint64_failure c {v with pos' := pos, depth' := v.depth' + W64 1}
            xs tail dq _ _ parseFailure.2.2 Hpos Hremaining $$ [Hc Hs] as %err ⟨Hc, Hs, Hdiagnostic⟩
          · iframe Hkind Hc Hs
          ihave Herr : (ResourceMapEntryError (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)
              pos (.key (.numeric (.expected _))) : IProp GF) $$ [Hdiagnostic]
          · unfold ResourceMapEntryError
            iexists kinds
            iframe Hglobals
            unfold DAModeError
            iexists err
            iframe Hdiagnostic
            ipureintro; rfl
          wp_for_post
          have hposfinal : pos = v.pos' + W64 (1 + payload.length + consumed) := by word
          have hoffsetfinal : pos = v.pos' + W64 (1 + payload.length + offset) := by word
          have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
          isimp only [hposfinal] at Hc
          isimp only [hoffsetfinal] at Herr
          isimp only [hdoneall, Bool.false_eq_true, ite_false] at HΦ
          iapply HΦ
          iframe Hdst Hc Hs Herr
          isplitl [Hmap Hchildren]
          · unfold ResourceMapPartial
            isplitl [Hmap Hchildren]
            · iexists out
              unfold ResourceMapOwned
              iexists stored
              iframe Hmap Hchildren
            · ipureintro; rfl
          · ipureintro; exact Hprogress
        | @overflow keyHead keyPayload keyRest argument keyEnc keyMajor large =>
          have hn : keyHead ≠ W8 246 ∧ keyHead ≠ W8 247 := by
            constructor
            · intro eq
              have h := keyMajor
              rw [eq] at h
              exact (by decide : W8 246 &&& W8 224 ≠ W8 0) h
            · intro eq
              have h := keyMajor
              rw [eq] at h
              exact (by decide : W8 247 &&& W8 224 ≠ W8 0) h
          have hi := input_suffix_head xs (keyPayload ++ keyRest) (sint.nat pos) (sint.nat v.buf'.len) keyHead Hremaining
          wp_apply wp_null_other c {v with pos' := pos, depth' := v.depth' + W64 1}
            xs keyHead dq Hpos.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
          · iframe Hc Hs
          wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs keyRest
            ⟨keyHead, keyPayload, argument, keyMajor, keyEnc⟩ dq Hpos Hremaining
            $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
          · iframe Hc Hs
          have hbad : uint.Z (W64 4294967295) < uint.Z argument := large
          simp only [hbad, _root_.decide_true]
          wp_auto
          have hlength : (keyHead :: keyPayload).length = 1 + keyPayload.length := by simp; omega
          isimp only [UnsignedWire.bytes, hlength, Int.natCast_add, Int.natCast_one] at Hc
          ihave Herr : (ResourceMapEntryError kinds.typeKind (pos + W64 (1 + keyPayload.length))
              (.key (.numeric .overflow)) : IProp GF) $$ []
          · unfold ResourceMapEntryError
            iexists kinds
            iframe Hglobals
            unfold DAModeError
            ipureintro; exact ⟨rfl, HtypeNonNil⟩
          wp_for_post
          have hposfinal : pos + W64 (1 + keyPayload.length) = v.pos' + W64 (1 + payload.length + consumed) := by word
          have hoffsetfinal : pos + W64 (1 + keyPayload.length) = v.pos' + W64 (1 + payload.length + offset) := by word
          have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
          isimp only [hposfinal] at Hc
          isimp only [hoffsetfinal] at Herr
          isimp only [hdoneall, Bool.false_eq_true, ite_false] at HΦ
          iapply HΦ
          iframe Hdst Hc Hs Herr
          isplitl [Hmap Hchildren]
          · unfold ResourceMapPartial
            isplitl [Hmap Hchildren]
            · iexists out
              unfold ResourceMapOwned
              iexists stored
              iframe Hmap Hchildren
            · ipureintro; rfl
          · ipureintro; exact Hprogress
    | @value keyItem wire key retained valueConsumed valueOffset entered valueReason keyValid reject =>
      have hn := resourceMapKeyValid_not_nullish keyItem wire key keyValid
      wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
        xs (keyItem ++ wire) dq hn.1 hn.2 Hpos Hremaining $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      cases keyValid with
      | @unsigned keyHead keyPayload argument keyEnc keyMajor fits =>
        wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs wire
          ⟨keyHead, keyPayload, argument, keyMajor, keyEnc⟩ dq Hpos Hremaining
          $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
        · iframe Hc Hs
        have hgood : ¬ uint.Z (W64 4294967295) < uint.Z argument := by change ¬ (4294967295 : Int) < uint.Z argument; omega
        simp only [hgood, _root_.decide_false]
        wp_auto
        isimp only [UnsignedWire.bytes] at Hc
        ihave Howned := resourceBoundsOwned_zero value_ptr $$ value
        wp_apply wp_readResourceBounds_record_rejected validator c value_ptr resourceBoundsZero retained
          {v with pos' := pos + W64 (keyHead :: keyPayload).length, depth' := v.depth' + W64 1}
          xs wire dq (d + 1) valueConsumed valueOffset entered valueReason reject (by word)
          ⟨by simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
          $$ [Howned Hc Hs] as %err ⟨Howned, Hc, Hs, Hdiagnostic, %Hstep⟩
        · iframe Howned Hc Hs
        simp only at Hstep
        ihave Herr : (ResourceMapEntryError (interface.mkOk (go.PointerType cursor.DecodeError.ty) #err)
            (pos + W64 (keyHead :: keyPayload).length + W64 valueOffset) (.value valueReason) : IProp GF) $$ [Hdiagnostic]
        · unfold ResourceMapEntryError
          iexists err
          iframe Hdiagnostic
          ipureintro; rfl
        wp_for_post
        have hposfinal : pos + W64 (keyHead :: keyPayload).length + W64 valueConsumed = v.pos' + W64 (1 + payload.length + consumed) := by word
        have hoffsetfinal : pos + W64 (keyHead :: keyPayload).length + W64 valueOffset = v.pos' + W64 (1 + payload.length + offset) := by word
        have hdoneall : doneValues = done := by simpa only [List.append_nil] using Hsplit
        isimp only [hposfinal] at Hc
        isimp only [hoffsetfinal] at Herr
        isimp only [hdoneall] at HΦ
        iapply HΦ
        iframe Hdst Hc Hs Herr
        isplitl [Hmap Hchildren Howned]
        · unfold ResourceMapPartial
          isplitl [Hmap Hchildren]
          · iexists out
            unfold ResourceMapOwned
            iexists stored
            iframe Hmap Hchildren
          · iexists value_ptr; iexact Howned
        · ipureintro; exact Hprogress
  | @next n item tail entry todo retained leftConsumed leftOffset entered reason first rest =>
    have hitem : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = item ++ tail := Hremaining
    have Hadvance := input_consumption xs item tail pos v.buf'.len Hpos Hlen.1 hitem
    have hrestcount : count - (done ++ [entry]).length = n := by
      simp only [List.length_append, List.length_singleton]; omega
    cases first with
    | @pair keyItem valueItem suffix key value keyValid record =>
      have hn := resourceMapKeyValid_not_nullish keyItem (valueItem ++ tail) key keyValid
      have hkey : subslice (sint.nat pos) (sint.nat v.buf'.len) xs = keyItem ++ (valueItem ++ tail) := by
        simpa only [List.append_assoc] using hitem
      wp_apply wp_null_not_nullish c {v with pos' := pos, depth' := v.depth' + W64 1}
        xs (keyItem ++ (valueItem ++ tail)) dq hn.1 hn.2 Hpos hkey $$ [Hc Hs] with ⟨Hc, Hs⟩
      · iframe Hc Hs
      cases keyValid with
      | @unsigned keyHead keyPayload argument keyEnc keyMajor fits =>
        wp_apply wp_uint64_wire c {v with pos' := pos, depth' := v.depth' + W64 1} xs
          (valueItem ++ tail) ⟨keyHead, keyPayload, argument, keyMajor, keyEnc⟩ dq Hpos hkey
          $$ [Hc Hs] with ⟨Hc, Hs, %Hkey⟩
        · iframe Hc Hs
        have hgood : ¬ uint.Z (W64 4294967295) < uint.Z argument := by change ¬ (4294967295 : Int) < uint.Z argument; omega
        simp only [hgood, _root_.decide_false]
        wp_auto
        isimp only [UnsignedWire.bytes] at Hc
        ihave Howned := resourceBoundsOwned_zero value_ptr $$ value
        wp_apply wp_readResourceBounds_record_valid validator c value_ptr resourceBoundsZero value
          {v with pos' := pos + W64 (keyHead :: keyPayload).length, depth' := v.depth' + W64 1}
          xs valueItem tail dq (d + 1) record (by word)
          ⟨by simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega, Hkey.2.1⟩ Hkey.2.2
          $$ [Howned Hc Hs] with ⟨Howned, Hc, Hs, %Hstep⟩
        · iframe Howned Hc Hs
        icases resourceMapValue_take value_ptr value $$ Howned with ⟨%newStored, Hvalue, Hchild⟩
        wp_auto
        wp_apply wp_mapInsert $$ Hmap with Hmap
        ihave Hchildren := resourceMapChildren_insert stored (resourceMapOfEntries done)
          (W32 (uint.Z argument)) newStored value $$ [Hchildren Hchild]
        · iframe Hchildren Hchild
        wp_for_post
        have hdec : W64 ((n + 1 : Nat) : Int) - W64 1 = W64 (count - (done ++ [(W32 (uint.Z argument), value)]).length : Nat) := by word
        isimp only [hdec] at Hcount
        isimp only [← resourceMapOfEntries_append_one] at Hchildren
        iframe
        iexists (done ++ [(W32 (uint.Z argument), value)]), todo,
          (<[W32 (uint.Z argument) := newStored]> stored),
          (front ++ ((keyHead :: keyPayload) ++ valueItem)), tail,
          leftConsumed, leftOffset, (pos + W64 (keyHead :: keyPayload).length + W64 valueItem.length)
        iframe
        ipureintro
        refine ⟨?_, ?_, ?_, ?_, ?_, ⟨?_, Hstep.2.1⟩, ?_, Hstep.2.2⟩
        · simpa only [List.append_assoc, List.singleton_append] using Hsplit
        · simpa only [List.append_assoc] using Hwire
        · simpa only [hrestcount] using rest
        · simp only [List.length_append] at Hconsumed ⊢; omega
        · simp only [List.length_append] at Hoffset ⊢; omega
        · simp only at Hstep; rw [Hstep.1]; simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega
        · simp only [List.length_append, Int.natCast_add]
          simp only at Hstep; rw [Hstep.1]; simp only [UnsignedWire.bytes] at Hkey; rw [Hkey.1]; omega

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readResourceMap_map_rejected
