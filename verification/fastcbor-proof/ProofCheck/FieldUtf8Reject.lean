import ProofCheck.FieldReject
import ProofCheck.KeyUtf8Reject

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

/-- Malformed UTF-8 text cannot equal a cached ASCII field name, including
    non-preferred text-length encodings. -/
theorem inlineFieldTable_bad_lead_miss (order : List cursor.StaticField) (names : List GoString)
    (head b : w8) (payload tail rem : List w8)
    (htable : InlineFieldTable order names) (hascii : ∀ name ∈ names, ASCIIBytes name)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (bound : (b :: tail).length < 2^63) (bad : Utf8BadLead b) :
    ∀ (j : Nat) (entry : cursor.StaticField), order[j]? = some entry →
      ((head :: payload) ++ (b :: tail) ++ rem).take entry.Encoded'.length ≠ entry.Encoded' := by
  intro j entry hlookup hprefix
  obtain ⟨name, _, _, hn, hlen, hencoded⟩ :=
    inlineFieldTable_entry order names entry htable (List.mem_of_getElem? hlookup)
  rw [hencoded] at hprefix
  have heq := inline_text_prefix head payload (b :: tail) rem name enc hlen bound
    (by simpa only [List.length_cons, Nat.add_comm] using hprefix)
  have hb := hascii name (List.mem_of_getElem? hn) b (by rw [← heq.2.1]; simp)
  unfold Utf8BadLead at bad
  omega

/-- The actual Header and GasPrice schemas contain only ASCII names. -/
theorem headerFieldNames_ascii : ∀ name ∈ headerFieldNames, ASCIIBytes name := by
  simp only [headerFieldNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

theorem gasFieldNames_ascii : ∀ name ∈ gasFieldNames, ASCIIBytes name := by
  simp only [gasFieldNames, List.mem_cons, List.not_mem_nil,
    _root_.or_false, forall_eq_or_imp, forall_eq]
  repeat' apply And.intro
  all_goals unfold ASCIIBytes; decide

/-- Cached search, alternate lookup and public field reading preserve all
    state except the remaining pair count on illegal-leading-byte rejection. -/
theorem wp_nextField_bad_lead (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload tail rem : List w8)
    (head b : w8) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (hascii : ∀ name ∈ names, ASCIIBytes name)
    (enc : HeadArgument head payload (W64 (b :: tail).length))
    (hm : head &&& W8 224 = W8 96)
    (bad : Utf8BadLead b)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ (b :: tail) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrSyntax ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗
       err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs ((head :: payload) ++ (b :: tail)) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length + (b :: tail).length ≤ sint.Z v.buf'.len := by
    simp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext ⊢
    omega
  have hkey : (b :: tail).length < 2^63 := by word
  have hmiss := inlineFieldTable_bad_lead_miss order names head b payload tail rem htable hascii enc hkey bad
  wp_apply wp_nextField_reject c st fields v v s f order xs ((head :: payload) ++ (b :: tail) ++ rem) qf qo dq
    iprop(globalAddr cursor.ErrSyntax ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"key is not valid UTF-8") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_bad_lead c st v xs payload tail rem head b dq kind enc hm bad hp hbytes)
    $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
  · iframe Hkind Hc Hst Hfields Horder Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_bad_lead_miss

#print axioms Perennial.github_com.NethermindEth.juno.core.headerFieldNames_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.gasFieldNames_ascii

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_bad_lead
