import ProofCheck.FieldFallback

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

/-- A failed expected-key word comparison preserves the cursor and all table state
    before invoking the proved general search/name path. Duplicate bits cannot hide a mismatch. -/
theorem wp_nextField_cached_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (input tail : List w8) (encoded : GoString) (index : w64) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hcache : CachedShortTable order)
    (hnext : 0 ≤ sint.Z s.next' ∧ sint.Z s.next' < sint.Z f.order'.len)
    (hlookup : order[(sint.Z s.next').toNat]? = some (cachedField encoded index))
    (hinput : input.length = 24) (hkey : encoded.length ≤ 24)
    (hmatch : input.take encoded.length ≠ encoded)
    (hwindow : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = input ++ tail)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      ((head :: payload) ++ key ++ rem).take entry.Encoded'.length ≠ entry.Encoded') (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  let R : IProp GF := iprop(globalAddr cursor.ErrUnsupported ↦□ kind ∗ f.names' ↦*{qn} names)
  let Q (code : w64) (err : GoInterface) : IProp GF := iprop(c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)))
  have hslow :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
    wp_start_folded as ⟨HR, Hc, Hst, Hfields, Horder, Hs⟩
    isimp only [R] at HR
    icases HR with ⟨#Hkind, Hnames⟩
    wp_apply wp_nextField_slow_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
      hfields hleft hstart hcache hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    unfold Q
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  have hcmp : cacheCompare input encoded ≠ W64 0 := mt (cached_compare_prefix input encoded (by omega) hkey).mp hmatch
  have hblocked : cacheCompare input encoded ||| (s.seen' &&& (W64 1 <<< index)) ≠ W64 0 := by
    intro hz; exact hcmp (BitVec.or_eq_zero_iff.mp hz).1
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  wp_apply wp_nextField_cached_else c st fields v s f order xs input tail encoded index qf qo dq R Q
    hfields hleft hnext hlookup hinput hblocked hp hwindow hslow
    $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err HQ
  · unfold R
    iframe Hkind Hc Hst Hfields Horder Hnames Hs
  iapply HΦ
  isimp only [Q] at HQ
  iexact HQ

/-- Public field reading after an absent encoded-table prefix, independent of the
    remaining input length. The actual cache, search, parser and duplicate check compose. -/
theorem wp_nextField_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hcache : CachedShortTable order)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ [])
    (hsorted : FieldHeadsOrdered order)
    (hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
      ((head :: payload) ++ key ++ rem).take entry.Encoded'.length ≠ entry.Encoded') (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  by_cases hwindow : ((head :: payload) ++ key ++ rem).length < 24
  · wp_apply wp_nextField_short_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
      hfields hleft hstart hwindow hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hadvance
  · by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
    · ihave %Horderlen := ownSlice_len _ _ _ $$ Horder
      have hi : (sint.Z s.next').toNat < order.length := by word
      let entry := order[(sint.Z s.next').toNat]'hi
      have hlookup : order[(sint.Z s.next').toNat]? = some entry := List.getElem?_eq_getElem hi
      obtain ⟨hrep, hkey⟩ := hcache entry (List.getElem_mem hi)
      have hlookup' : order[(sint.Z s.next').toNat]? = some (cachedField entry.Encoded' entry.Index') := by
        exact hlookup.trans (congrArg some hrep)
      have hinput : (((head :: payload) ++ key ++ rem).take 24).length = 24 := by simp only [List.length_take]; omega
      have hmismatch : (((head :: payload) ++ key ++ rem).take 24).take entry.Encoded'.length ≠ entry.Encoded' := by
        rw [List.take_take, Nat.min_eq_left hkey]
        exact hmiss _ entry (Nat.le_refl _) hlookup
      have hwindowbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
          ((head :: payload) ++ key ++ rem).take 24 ++ ((head :: payload) ++ key ++ rem).drop 24 := by
        simpa only [List.take_append_drop] using hbytes
      wp_apply wp_nextField_cached_miss_ascii c st fields v s f names xs payload key rem head order
        (((head :: payload) ++ key ++ rem).take 24) (((head :: payload) ++ key ++ rem).drop 24)
        entry.Encoded' entry.Index' qf qo qn dq kind hfields hleft hstart hcache ⟨hstart.1, hnext⟩
        hlookup' hinput hkey hmismatch hwindowbytes hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance
    · wp_method_call
      wp_call
      wp_auto
      have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
      simp only [hleft', _root_.decide_true]
      wp_auto
      simp only [hfields]
      icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
      wp_auto
      simp only [hnext, _root_.decide_false]
      wp_auto
      ihave Hfields := Hrestore $$ Hfo
      wp_apply wp_nextField_slow_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
        hfields hleft hstart hcache hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
        $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
      · iframe Hkind Hc Hst Hfields Horder Hnames Hs
      isimp only [hfields] at Hout
      iapply HΦ
      iframe Hc Hfields Horder Hnames Hs Hout
      ipureintro; exact Hadvance

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_cached_miss_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_miss_ascii
