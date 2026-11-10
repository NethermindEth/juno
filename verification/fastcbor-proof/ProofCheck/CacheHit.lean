import ProofCheck.CacheMiss

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A later table hit or a previously seen expected field falls through the
    actual word cache into the proved search/mark path without changing state first. -/
theorem wp_nextField_cached_fallthrough_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem input tail : List w8) (encoded : GoString) (index : w64) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hcache : CachedShortTable order)
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hexpected : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hinput : input.length = 24)
    (hblocked : cacheCompare input encoded ||| (s.seen' &&& (W64 1 <<< index)) ≠ W64 0)
    (hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail)
    (hindex : 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64)
    (hname : names[(sint.Z entry.Index').toNat]? = some name)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = entry.Encoded' ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 entry.Encoded'.length} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 entry.Encoded'.length) = sint.Z v.pos' + entry.Encoded'.length ∧
         sint.Z (v.pos' + W64 entry.Encoded'.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 entry.Encoded'.length)) (sint.nat v.buf'.len) xs = rem⌝ ∗
       KnownFieldOutcome st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index' name
         code err kind (v.pos' + W64 entry.Encoded'.length) }} := by
  let R : IProp GF := iprop(globalAddr cursor.ErrUnsupported ↦□ kind ∗ f.names' ↦*{qn} names)
  let Q (code : w64) (err : GoInterface) : IProp GF := iprop(c ↦ {v with pos' := v.pos' + W64 entry.Encoded'.length} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 entry.Encoded'.length) = sint.Z v.pos' + entry.Encoded'.length ∧
         sint.Z (v.pos' + W64 entry.Encoded'.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 entry.Encoded'.length)) (sint.nat v.buf'.len) xs = rem⌝ ∗
       KnownFieldOutcome st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index' name
         code err kind (v.pos' + W64 entry.Encoded'.length))
  have hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
    wp_start_folded as ⟨HR, Hc, Hst, Hfields, Horder, Hs⟩
    isimp only [R] at HR
    icases HR with ⟨#Hkind, Hnames⟩
    wp_apply wp_nextField_slow_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
      hfields hleft hstart hlookup hnonempty hsorted hapart hcache hindex hname hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    unfold Q
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  wp_apply wp_nextField_cached_else c st fields v s f order xs input tail encoded index qf qo dq R Q
    hfields hleft hnext hexpected hinput hblocked hp hwindow hslow
    $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err HQ
  · unfold R
    iframe Hkind Hc Hst Hfields Horder Hnames Hs
  iapply HΦ
  isimp only [Q] at HQ
  iexact HQ

/-- Public NextField handles every encoded-table hit at or after the search index,
    for arbitrary remaining input lengths, including new fields and duplicates. -/
theorem wp_nextField_hit (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (entry : cursor.StaticField) (name : GoString)
    (xs rem : List w8) (target : Nat) (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ target)
    (hlookup : order[target]? = some entry) (hnonempty : ∀ key ∈ order, key.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order)
    (hcache : CachedShortTable order)
    (hindex : 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64)
    (hname : names[(sint.Z entry.Index').toNat]? = some name)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = entry.Encoded' ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 entry.Encoded'.length} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 entry.Encoded'.length) = sint.Z v.pos' + entry.Encoded'.length ∧
         sint.Z (v.pos' + W64 entry.Encoded'.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 entry.Encoded'.length)) (sint.nat v.buf'.len) xs = rem⌝ ∗
       KnownFieldOutcome st {s with left' := s.left' - W64 1, next' := W64 (target + 1)} entry.Index' name
         code err kind (v.pos' + W64 entry.Encoded'.length) }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  by_cases hwindow : (entry.Encoded' ++ rem).length < 24
  · wp_apply wp_nextField_short_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
      hfields hleft hstart hlookup hnonempty hsorted hapart hwindow hindex hname hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  · ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have ht := (List.getElem?_eq_some_iff.mp hlookup).choose
    have hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len := by word
    have hi : (sint.Z s.next').toNat < order.length := by word
    let expected := order[(sint.Z s.next').toNat]'hi
    have hexpected : order[(sint.Z s.next').toNat]? = some expected := List.getElem?_eq_getElem hi
    obtain ⟨hrep, hkey⟩ := hcache expected (List.getElem_mem hi)
    obtain ⟨hentryrep, hentrylen⟩ := hcache entry (List.mem_of_getElem? hlookup)
    have hexpectedcache : order[(sint.Z s.next').toNat]? = some (cachedField expected.Encoded' expected.Index') :=
      hexpected.trans (congrArg some hrep)
    have hinput : ((entry.Encoded' ++ rem).take 24).length = 24 := by simp only [List.length_take]; omega
    have hwindowbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
        (entry.Encoded' ++ rem).take 24 ++ (entry.Encoded' ++ rem).drop 24 := by
      simpa only [List.take_append_drop] using hbytes
    by_cases hfast : (sint.Z s.next').toNat = target ∧ s.seen' &&& (W64 1 <<< entry.Index') = W64 0
    · have hfastlookup : order[(sint.Z s.next').toNat]? = some (cachedField entry.Encoded' entry.Index') := by
        rw [hfast.1]; exact hlookup.trans (congrArg some hentryrep)
      have hprefix : ((entry.Encoded' ++ rem).take 24).take entry.Encoded'.length = entry.Encoded' := by
        rw [List.take_take, Nat.min_eq_left hentrylen, List.take_left]
      wp_apply wp_nextField_cached c st fields v s f order xs ((entry.Encoded' ++ rem).take 24)
        ((entry.Encoded' ++ rem).drop 24) entry.Encoded' entry.Index' qf qo dq hfields hleft hnext
        hfastlookup hfast.2 hinput hentrylen hprefix hp hwindowbytes
        $$ [Hc Hst Hfields Horder Hs] with ⟨Hc, Hst, Hfields, Horder, Hs, %HfastAdvance⟩
      · iframe Hc Hst Hfields Horder Hs
      have hinc : s.next' + W64 1 = W64 (target + 1) := by word
      isimp only [hinc] at Hst
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs
      isplitr [Hst]
      · ipureintro; exact input_consumption xs entry.Encoded' rem v.pos' v.buf'.len hp Hlen.1 hbytes
      unfold KnownFieldOutcome
      ileft
      iframe Hst
      ipureintro; exact ⟨rfl, rfl, hfast.2⟩
    · have hblocked : cacheCompare ((entry.Encoded' ++ rem).take 24) expected.Encoded' |||
          (s.seen' &&& (W64 1 <<< expected.Index')) ≠ W64 0 := by
        intro hz
        obtain ⟨hcmp, hseen⟩ := BitVec.or_eq_zero_iff.mp hz
        have hprefix := (cached_compare_prefix ((entry.Encoded' ++ rem).take 24) expected.Encoded'
          (by omega) hkey).mp hcmp
        rw [List.take_take, Nat.min_eq_left hkey] at hprefix
        have hsame := fieldPrefix_unique order (entry.Encoded' ++ rem) hapart (sint.Z s.next').toNat target
          expected entry hexpected hlookup hprefix List.take_left
        have hent : expected = entry := by
          have hh := hexpected
          rw [hsame] at hh
          exact Option.some.inj (hh.symm.trans hlookup)
        apply hfast
        refine ⟨hsame, ?_⟩
        rw [hent] at hseen
        exact hseen.trans (by decide)
      wp_apply wp_nextField_cached_fallthrough_hit c st fields v s f order names entry name xs rem
        ((entry.Encoded' ++ rem).take 24) ((entry.Encoded' ++ rem).drop 24) expected.Encoded' expected.Index'
        target kind qf qo qn dq hfields hleft hstart hlookup hnonempty hsorted hapart hcache hnext
        hexpectedcache hinput hblocked hwindowbytes hindex hname hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached_fallthrough_hit

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_hit
