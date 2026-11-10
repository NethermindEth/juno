import ProofCheck.CacheHit

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- A matching canonical inline text prefix identifies the text payload itself.
    This links raw cached-byte equality to the application-level field name. -/
theorem inline_text_prefix (head : w8) (payload key rem name : List w8)
    (enc : HeadArgument head payload (W64 key.length)) (hname : name.length < 24)
    (hkey : key.length < 2^63)
    (hprefix : (((head :: payload) ++ key) ++ rem).take (1 + name.length) = W8 (96 + name.length) :: name) :
    payload = [] ∧ key = name ∧ head = W8 (96 + name.length) := by
  have hp : (((head :: payload) ++ key) ++ rem).take (W8 (96 + name.length) :: name).length =
      W8 (96 + name.length) :: name := by simpa only [List.length_cons, Nat.add_comm] using hprefix
  have hh := prefix_head_eq (((head :: payload) ++ key) ++ rem) (W8 (96 + name.length) :: name) (by simp) hp
  have hhead : head = W8 (96 + name.length) := by simpa using hh
  have hbits : uint.Z (W8 (96 + name.length) &&& W8 31) = name.length := by
    have h : ∀ n : Fin 24, uint.Z (W8 (96 + n.val) &&& W8 31) = n.val := by decide
    exact h ⟨name.length, hname⟩
  have hsmall : uint.Z (head &&& W8 31) < 24 := by rw [hhead, hbits]; omega
  obtain ⟨hpay, harg⟩ := headArgument_inline_inv head payload (W64 key.length) enc hsmall
  have hlength : key.length = name.length := by rw [hhead, hbits] at harg; word
  have hnames : key = name := by
    rw [hpay] at hprefix
    simp only [List.singleton_append, List.cons_append, Nat.add_comm 1, List.take_succ_cons] at hprefix
    have htail := congrArg List.tail hprefix
    simp only [List.tail_cons] at htail
    rwa [← hlength, List.nil_append, List.take_left] at htail
  exact ⟨hpay, hnames, hhead⟩


/-- Each cached entry names the indexed schema field and uses its canonical inline
    text encoding. Table shape and search/cache representations are separate properties. -/
def InlineFieldTable (order : List cursor.StaticField) (names : List GoString) : Prop :=
  ∀ entry ∈ order, 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64 ∧
    match names[(sint.Z entry.Index').toNat]? with
    | none => False
    | some name => name.length < 24 ∧ entry.Encoded' = W8 (96 + name.length) :: name

theorem inlineFieldTable_entry (order : List cursor.StaticField) (names : List GoString)
    (entry : cursor.StaticField) (htable : InlineFieldTable order names) (hmem : entry ∈ order) :
    ∃ name, 0 ≤ sint.Z entry.Index' ∧ sint.Z entry.Index' < 64 ∧
      names[(sint.Z entry.Index').toNat]? = some name ∧ name.length < 24 ∧
      entry.Encoded' = W8 (96 + name.length) :: name := by
  obtain ⟨h0, h64, hn⟩ := htable entry hmem
  cases heq : names[(sint.Z entry.Index').toNat]? with
  | none => simp only [heq] at hn
  | some name =>
    simp only [heq] at hn
    exact ⟨name, h0, h64, rfl, hn⟩

theorem headerInlineFieldTable : InlineFieldTable headerFieldKeys headerFieldNames := by
  simp only [InlineFieldTable, headerFieldKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

theorem gasInlineFieldTable : InlineFieldTable gasFieldKeys gasFieldNames := by
  simp only [InlineFieldTable, gasFieldKeys, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals decide

/-- Exact search-index progress: an absent encoded prefix preserves the index;
    a matching remaining entry advances it to one past that entry. -/
def FieldSearchAdvance (order : List cursor.StaticField) (wire : List w8) (start next : w64) : Prop :=
  (next = start ∧ ∀ j entry, (sint.Z start).toNat ≤ j → order[j]? = some entry →
    wire.take entry.Encoded'.length ≠ entry.Encoded') ∨
  ∃ j entry, (sint.Z start).toNat ≤ j ∧ order[j]? = some entry ∧
    wire.take entry.Encoded'.length = entry.Encoded' ∧ next = W64 (j + 1)

/-- Search-index progress preserves the bound required by the next iteration. -/
theorem fieldSearchAdvance_bounds (order : List cursor.StaticField) (wire : List w8) (start next : w64)
    (hstart : 0 ≤ sint.Z start ∧ (sint.Z start).toNat ≤ order.length)
    (hlen : order.length < 2^63) (hstep : FieldSearchAdvance order wire start next) :
    0 ≤ sint.Z next ∧ (sint.Z next).toNat ≤ order.length := by
  rcases hstep with ⟨rfl, _⟩ | ⟨j, entry, hj, hlookup, hprefix, rfl⟩
  · exact hstart
  · have hjbound := (List.getElem?_eq_some_iff.mp hlookup).choose
    word

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- A single application-facing field-reading contract for all cache paths,
    arbitrary field order and inline/one-byte ASCII keys. -/
theorem wp_nextField_ascii (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload key rem : List w8) (head : w8)
    (kind : GoInterface) (qf qo qn dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hapart : FieldEncodingsApart order) (hcache : CachedShortTable order)
    (htable : InlineFieldTable order names) (hnames : names.length ≤ 64)
    (enc : HeadArgument head payload (W64 key.length)) (hm : head &&& W8 224 = W8 96)
    (hshort : uint.Z (head &&& W8 31) < 24 ∨ head &&& W8 31 = W8 24) (hascii : ASCIIBytes key)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ key ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrUnsupported ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (code : w64) (err : GoInterface), RET (PairV #code #err); ∃ next : w64,
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length + key.length)} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ f.names' ↦*{qn} names ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length + key.length)) = sint.Z v.pos' + 1 + payload.length + key.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length + key.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length + key.length))) (sint.nat v.buf'.len) xs = rem⌝ ∗
       ⌜FieldSearchAdvance order ((head :: payload) ++ key ++ rem) s.next' next⌝ ∗
       FieldReadOutcome st {s with left' := s.left' - W64 1, next' := next} names key code err kind
         (v.pos' + W64 (1 + payload.length + key.length)) }} := by
  iintro %Φ ⟨#Hpkg, #Hkind, Hc, Hst, Hfields, Horder, Hnames, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hconsume := input_consumption xs ((head :: payload) ++ key) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hfront : ((head :: payload) ++ key).length = 1 + payload.length + key.length := by simp; omega
  rw [hfront] at Hconsume
  simp only [Int.natCast_add, Int.natCast_one] at Hconsume
  have hkeybound : key.length < 2^63 := by word
  by_cases hmatch : ∃ target entry, (sint.Z s.next').toNat ≤ target ∧ order[target]? = some entry ∧
      ((head :: payload) ++ key ++ rem).take entry.Encoded'.length = entry.Encoded'
  · obtain ⟨target, entry, hafter, hlookup, hprefix⟩ := hmatch
    obtain ⟨name, hi0, hi64, hname, hnamelen, hencoded⟩ :=
      inlineFieldTable_entry order names entry htable (List.mem_of_getElem? hlookup)
    have htextprefix : ((head :: payload) ++ key ++ rem).take (1 + name.length) = W8 (96 + name.length) :: name := by
      rw [hencoded] at hprefix
      simpa only [List.length_cons, Nat.add_comm] using hprefix
    obtain ⟨hpay, hkey, hhead⟩ := inline_text_prefix head payload key rem name enc hnamelen hkeybound htextprefix
    have hwire : entry.Encoded' = (head :: payload) ++ key := by
      rw [hencoded, hpay, hkey, hhead]; rfl
    have hencodedlen : entry.Encoded'.length = 1 + payload.length + key.length := by rw [hwire, hfront]
    have hhitbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = entry.Encoded' ++ rem := by
      rw [hwire]; exact hbytes
    wp_apply wp_nextField_hit c st fields v s f order names entry name xs rem target kind qf qo qn dq
      hfields hleft ⟨hstart.1, hafter⟩ hlookup hnonempty hsorted hapart hcache ⟨hi0, hi64⟩ hname hp hhitbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    isimp only [hencodedlen, Int.natCast_add, Int.natCast_one] at Hc Hout
    iapply HΦ
    iexists (W64 (target + 1))
    iframe Hc Hfields Horder Hnames Hs
    isplitr [Hout]
    · ipureintro
      exact ⟨by omega, Hconsume.2⟩
    isplitr [Hout]
    · ipureintro
      exact Or.inr ⟨target, entry, hafter, hlookup, hprefix, rfl⟩
    unfold FieldReadOutcome
    iright
    iexists entry.Index', name
    isplitr [Hout]
    · ipureintro
      exact ⟨hi0, hi64, hname, congrArg (List.map fieldFoldByte) hkey⟩
    isimp only [KnownFieldOutcome] at Hout
    iexact Hout
  · have hmiss : ∀ j entry, (sint.Z s.next').toNat ≤ j → order[j]? = some entry →
        ((head :: payload) ++ key ++ rem).take entry.Encoded'.length ≠ entry.Encoded' := by
      intro j entry hj he hp
      exact hmatch ⟨j, entry, hj, he, hp⟩
    wp_apply wp_nextField_miss_ascii c st fields v s f names xs payload key rem head order qf qo qn dq kind
      hfields hleft hstart hcache hnonempty hsorted hmiss hnames enc hm hshort hascii hp hbytes
      $$ [Hc Hst Hfields Horder Hnames Hs] as %code %err ⟨Hc, Hfields, Horder, Hnames, Hs, %Hadvance, Hout⟩
    · iframe Hkind Hc Hst Hfields Horder Hnames Hs
    iapply HΦ
    iexists s.next'
    iframe Hc Hfields Horder Hnames Hs Hout
    ipureintro
    exact ⟨Hadvance, Or.inl ⟨rfl, hmiss⟩⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.inline_text_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_entry
#print axioms Perennial.github_com.NethermindEth.juno.core.headerInlineFieldTable
#print axioms Perennial.github_com.NethermindEth.juno.core.gasInlineFieldTable

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.fieldSearchAdvance_bounds
