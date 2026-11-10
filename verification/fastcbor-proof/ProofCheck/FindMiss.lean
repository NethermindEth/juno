import ProofCheck.FindShort

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

/-- A short-buffer search miss reaches the name matcher, including unknown names,
    case variants and duplicates, after consuming exactly one outstanding pair. -/
theorem wp_nextField_slow_short_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hwindow : ((head :: payload) ++ key ++ rem).length < 24)
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
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"nextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗
       fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1} names key code err kind (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
  wp_auto
  have hn : s.left' ≠ W64 0 := by word
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp.1, hp.2, Hwf.2⟩)]
  wp_auto
  icases (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).1
    $$ Hs with ⟨Hpre, Hbuf, Hpost⟩
  isimp only [hbytes] at Hbuf
  wp_apply wp_find_short st fields {s with left' := s.left' - W64 1} f order
    (slice.slice v.buf' w8 v.pos' v.buf'.len) ((head :: payload) ++ key ++ rem) qf qo dq
    hfields hstart hwindow hnonempty hsorted $$ [Hst Hfields Horder Hbuf] as %result ⟨Hst, Hfields, Horder, Hbuf, %Hfind⟩
  · iframe Hst Hfields Horder Hbuf
  have hresult := fieldFindResult_miss order ((head :: payload) ++ key ++ rem) (sint.Z s.next').toNat result Hfind hmiss
  subst result
  wp_auto
  ihave Hs : (v.buf' ↦*{dq} xs : IProp GF) $$ [Hpre Hbuf Hpost]
  · iapply (ownSlice_slice v.pos' v.buf'.len v.buf' dq xs ⟨hp.1, hp.2, Int.le_refl _⟩).2
    rw [hbytes]; iframe Hpre Hbuf Hpost
  wp_apply wp_otherField_ascii c st fields v {s with left' := s.left' - W64 1} f names xs payload key rem head
    qf qn dq kind hfields hnames enc hm hshort hascii hp hbytes
    $$ [Hc Hst Hfields Hnames Hs] as %code %err ⟨Hc, Hfields, Hnames, Hs, %Hnext, Hout⟩
  · iframe Hkind Hc Hst Hfields Hnames Hs
  iapply HΦ
  iframe Hc Hfields Horder Hnames Hs Hout
  ipureintro; exact Hnext

/-- Public NextField composes the short search miss with actual key parsing and
    name matching. The whole input, metadata and exact suffix are preserved. -/
theorem wp_nextField_short_miss_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (order : List cursor.StaticField) (qf qo qn dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hwindow : ((head :: payload) ++ key ++ rem).length < 24)
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
  wp_start as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ key ++ rem) [] v.pos' v.buf'.len hp Hlen.1
    (by simpa only [List.append_nil] using hbytes)
  have hremaining : sint.Z v.buf'.len - sint.Z v.pos' < 24 := by
    have hl := congrArg List.length hbytes
    simp only [subslice, List.length_drop, List.length_take] at hl
    have hplen : sint.nat v.pos' ≤ xs.length := by word
    have hblen : sint.nat v.buf'.len = xs.length := Hlen.1.symm
    rw [hblen, Nat.min_self] at hl
    word
  wp_auto
  have hleft' : sint.Z (W64 0) < sint.Z s.left' := hleft
  simp only [hleft', _root_.decide_true]
  wp_auto
  simp only [hfields]
  icases (cursor.Fields.Fields_access_load_order fields f qf).access_strict $$ Hfields with ⟨Hfo, Hrestore⟩
  wp_auto
  by_cases hnext : sint.Z s.next' < sint.Z f.order'.len
  all_goals
    simp only [hnext, _root_.decide_true, _root_.decide_false]
    wp_auto
    have hguard : ¬ sint.Z (W64 24) ≤ sint.Z (v.buf'.len - v.pos') := by word
    try simp only [hguard, _root_.decide_false]
    try wp_auto
    ihave Hfields := Hrestore $$ Hfo
    wp_apply wp_nextField_slow_short_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
      hfields hleft hstart hwindow hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hnext, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    isimp only [hfields] at Hout
    iapply HΦ
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro; exact Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_slow_short_miss_ascii
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_short_miss_ascii
