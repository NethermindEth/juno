import ProofCheck.KeyASCII

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

/-- Application-level result of reading a text field name. Known names update a
    single seen bit or report a duplicate; unknown names leave struct progress alone. -/
def FieldReadOutcome (st : Loc) (s : cursor.Struct) (names : List GoString) (key : List w8)
    (code : w64) (err kind : GoInterface) (offset : w64) : IProp GF := iprop(
  (⌜code = W64 (-2) ∧ err = interface.nil ∧
      ∀ name ∈ names, key.map fieldFoldByte ≠ name.map fieldFoldByte⌝ ∗ st ↦ s) ∨
  ∃ (field : w64) (name : GoString),
    ⌜0 ≤ sint.Z field ∧ sint.Z field < 64 ∧ names[(sint.Z field).toNat]? = some name ∧
      key.map fieldFoldByte = name.map fieldFoldByte⌝ ∗
    ((⌜code = field ∧ err = interface.nil ∧ s.seen' &&& (W64 1 <<< field) = W64 0⌝ ∗
        st ↦ {s with seen' := s.seen' ||| (W64 1 <<< field)}) ∨
      ∃ (errorLoc : Loc) (args : GoSlice),
        ⌜code = W64 (-1) ∧ err = interface.mkOk (go.PointerType cursor.DecodeError.ty) #errorLoc ∧
          s.seen' &&& (W64 1 <<< field) ≠ W64 0⌝ ∗ st ↦ s ∗
        errorLoc ↦ cursor.DecodeError.mk kind offset (go!"field %s named twice") args ∗
        args ↦* [interface.mkOk go.string #name]))

/-- Matching a borrowed key restores both the unread suffix and the whole input. -/
theorem wp_fields_match_nested_range (fields : Loc) (f : cursor.Fields) (names : List GoString)
    (data : GoSlice) (xs key : List w8) (off low high : w64) (qf qn dq : DFrac)
    (hoff : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbound : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z (data.len - off))
    (hkey : subslice (sint.nat low) (sint.nat high) (subslice (sint.nat off) (sint.nat data.len) xs) = key) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ data ↦*{dq} xs }}
      (App (Val (fields @!! go.PointerType cursor.Fields.ty @!! go!"match"))
        (Val #(slice.slice (slice.slice data w8 off data.len) w8 low high)))
    {{ (index : w64) (ok : Bool), RET (PairV #index #ok);
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ data ↦*{dq} xs ∗
       ⌜FieldMatchResult names key index ok⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hfields, Hnames, Hs⟩ HΦ
  icases (ownSlice_slice off data.len data dq xs ⟨hoff.1, hoff.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  icases (ownSlice_slice low high (slice.slice data w8 off data.len) dq
      (subslice (sint.nat off) (sint.nat data.len) xs) hbound).1 $$ Hbuf with ⟨Hprefix, Hkey, Hsuffix⟩
  isimp only [hkey] at Hkey
  wp_apply wp_fields_match fields f names (slice.slice (slice.slice data w8 off data.len) w8 low high)
    key qf qn dq $$ [Hfields Hnames Hkey] as %index %ok ⟨Hfields, Hnames, Hkey, %Hmatch⟩
  · iframe Hfields Hnames Hkey
  ihave Hbuf : (slice.slice data w8 off data.len ↦*{dq}
      subslice (sint.nat off) (sint.nat data.len) xs : IProp GF) $$ [Hprefix Hkey Hsuffix]
  · iapply (ownSlice_slice low high (slice.slice data w8 off data.len) dq
      (subslice (sint.nat off) (sint.nat data.len) xs) hbound).2
    rw [hkey]
    iframe Hprefix Hkey Hsuffix
  ihave Hs : (data ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice off data.len data dq xs ⟨hoff.1, hoff.2, Int.le_refl _⟩).2
    iframe Hpre Hbuf Hpost
  iapply HΦ
  iframe Hfields Hnames Hs
  ipureintro; exact Hmatch

/-- Compose any proved text-key reader with actual name matching and duplicate
    handling. The explicit key WP must be discharged before claiming coverage
    for wider or non-ASCII keys; this theorem does not assume UTF-8 correctness. -/
theorem wp_otherField_text (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (qf qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hnames : names.length ≤ 64)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem)
    (hKey :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ RET (PairV (PairV #(slice.slice (slice.slice v.buf' w8 v.pos' v.buf'.len) w8
          (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length))) #true) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ }} ) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st s names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Hnames, Hs⟩
  wp_auto
  wp_apply hKey
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [hfields]
  have htotal : sint.Z v.pos' + 1 + payload.length + key.length ≤ sint.Z v.buf'.len := by omega
  have hbounds : 0 ≤ sint.Z (W64 (1 + payload.length)) ∧
      sint.Z (W64 (1 + payload.length)) ≤ sint.Z (W64 (1 + payload.length + key.length)) ∧
      sint.Z (W64 (1 + payload.length + key.length)) ≤ sint.Z (v.buf'.len - v.pos') := by word
  have hlo : sint.nat (W64 (1 + payload.length)) = 1 + payload.length := by word
  have hhi : sint.nat (W64 (1 + payload.length + key.length)) = 1 + payload.length + key.length := by word
  have hkey : subslice (sint.nat (W64 (1 + payload.length))) (sint.nat (W64 (1 + payload.length + key.length)))
      (subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs) = key := by
    rw [hbytes, hlo, hhi]
    exact (list_slice3 (head :: payload) key rem (1 + payload.length) (1 + payload.length + key.length)
      (by simp; omega) (by omega) (by omega)).2.1
  wp_apply wp_fields_match_nested_range fields f names v.buf' xs key v.pos'
    (W64 (1 + payload.length)) (W64 (1 + payload.length + key.length)) qf qn dq hp hbounds hkey
    $$ [Hfields Hnames Hs] as %index %ok ⟨Hfields, Hnames, Hs, %Hmatch⟩
  · iframe Hfields Hnames Hs
  cases ok with
  | false =>
    wp_auto
    iapply HΦ
    iframe Hc Hfields Hnames Hs
    isplitr [Hst]
    · ipureintro; exact Hnext
    unfold FieldReadOutcome
    ileft
    iframe Hst
    ipureintro
    exact ⟨rfl, rfl, Hmatch.2⟩
  | true =>
    rcases Hmatch with ⟨hi0, name, hlookup, hfold⟩
    have hibound : sint.Z index < 64 := by
      have hlt := (List.getElem?_eq_some_iff.mp hlookup).choose
      simp only [sint.Z] at hi0 hlt ⊢
      omega
    wp_auto
    by_cases hnew : s.seen' &&& (W64 1 <<< index) = W64 0
    · wp_apply wp_mark_new c st s index ⟨hi0, hibound⟩ hnew $$ [Hst] with Hst
      · iframe Hst
      iapply HΦ
      iframe Hc Hfields Hnames Hs
      isplitr [Hst]
      · ipureintro; exact Hnext
      unfold FieldReadOutcome
      iright
      iexists index, name
      isplitr [Hst]
      · ipureintro; exact ⟨hi0, hibound, hlookup, hfold⟩
      ileft
      iframe Hst
      ipureintro; exact ⟨rfl, rfl, hnew⟩
    · wp_apply wp_mark_duplicate c st fields {v with pos' := v.pos' + W64 (1 + payload.length + key.length)}
        s f names name index kind qf qn hfields ⟨hi0, hibound⟩ hlookup hnew
        $$ [Hc Hst Hfields Hnames] as %errorLoc ⟨Hc, Hst, Hfields, Hnames, Herr⟩
      · iframe Hc Hst Hfields Hnames Hkind
      icases Herr with ⟨%args, Herr, Hargs⟩
      iapply HΦ
      iframe Hc Hfields Hnames Hs
      isplitr [Hst Herr Hargs]
      · ipureintro; exact Hnext
      unfold FieldReadOutcome
      iright
      iexists index, name
      isplitr [Hst Herr Hargs]
      · ipureintro; exact ⟨hi0, hibound, hlookup, hfold⟩
      iright
      iexists errorLoc, args
      iframe Hst Herr Hargs
      ipureintro; exact ⟨rfl, rfl, hnew⟩

/-- The complete alternate-field path for short ASCII keys, including unknown
    names, case variants, arbitrary field order and owned duplicate diagnostics. -/
theorem wp_otherField_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (qf qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length))
    (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24)
    (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st s names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  exact wp_otherField_text c st fields v s f names xs payload key rem head qf qn dq kind
    hfields hnames hp hbytes (wp_Key_ascii c v xs payload key rem head dq enc hm hshort hascii hp hbytes)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_fields_match_nested_range
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_otherField_text
