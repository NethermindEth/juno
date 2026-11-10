import ProofCheck.FieldFallback
import ProofCheck.FieldRead

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 3200000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- An input outside the inline text-head range cannot match any indexed schema
    entry, regardless of its payload bytes or remaining length. -/
theorem inlineFieldTable_head_miss (order : List cursor.StaticField) (names : List GoString)
    (head : w8) (rest : List w8) (htable : InlineFieldTable order names)
    (hout : ¬ (96 ≤ uint.Z head ∧ uint.Z head < 120)) :
    ∀ (j : Nat) (entry : cursor.StaticField), order[j]? = some entry → (head :: rest).take entry.Encoded'.length ≠ entry.Encoded' := by
  intro j entry hlookup hprefix
  obtain ⟨name, _, _, _, hlen, hencoded⟩ := inlineFieldTable_entry order names entry htable (List.mem_of_getElem? hlookup)
  rw [hencoded] at hprefix
  have hhead := prefix_head_eq (head :: rest) (W8 (96 + name.length) :: name) (by simp) hprefix
  simp only [List.headD_cons] at hhead
  apply hout
  rw [hhead]
  word

/-- Every non-text CBOR major type falls outside the inline text-head range. -/
theorem nontext_head_outside (head : w8) (hm : head &&& W8 224 ≠ W8 96) :
    ¬ (96 ≤ uint.Z head ∧ uint.Z head < 120) := by
  have h : ∀ b : w8, b &&& W8 224 ≠ W8 96 → ¬ (96 ≤ uint.Z b ∧ uint.Z b < 120) := by decide
  exact h head hm

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Public NextField consumes an in-range integer key as unknown. The cache and
    search cannot mistake its bytes for a field name, and exactly one pair is consumed. -/
theorem wp_nextField_integer (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload rem : List w8)
    (head : w8) (argument : w64) (qf qo dq : DFrac)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32) (hrange : uint.Z argument < 2^63)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ RET (PairV #(W64 (-2)) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗
       st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  let Q (code : w64) (err : GoInterface) : IProp GF := iprop(
    ⌜code = W64 (-2) ∧ err = interface.nil⌝ ∗
    c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗
    st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗
    ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
      sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝)
  have hother :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         True ∗ c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
         f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (code : w64) (err : GoInterface), RET (PairV #code #err); Q code err }} := by
    wp_start_folded as ⟨-, Hc, Hst, Hfields, Horder, Hs⟩
    wp_apply wp_otherField_integer c st v xs payload rem head argument dq enc hm hrange hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    unfold Q
    iframe Hc Hst Hfields Horder Hs
    ipureintro; exact ⟨⟨rfl, rfl⟩, Hnext⟩
  have hmajor : head &&& W8 224 ≠ W8 96 := by rcases hm with hm | hm <;> rw [hm] <;> decide
  have hmiss := inlineFieldTable_head_miss order names head (payload ++ rem) htable (nontext_head_outside head hmajor)
  iintro %Φ ⟨#Hpkg, Hc, Hst, Hfields, Horder, Hs⟩ HΦ
  wp_apply wp_nextField_other c st fields v s f order xs ((head :: payload) ++ rem) qf qo dq iprop(True) Q
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes hother
    $$ [Hc Hst Hfields Horder Hs] as %code %err HQ
  · iframe Hc Hst Hfields Horder Hs
  isimp only [Q] at HQ
  icases HQ with ⟨%Heq, Hc, Hst, Hfields, Horder, Hs, %Hnext⟩
  rcases Heq with ⟨rfl, rfl⟩
  iapply HΦ
  iframe Hc Hst Hfields Horder Hs
  ipureintro; exact Hnext

/-- Out-of-range integer keys reject through public NextField, restoring the input
    cursor while retaining the one-pair decrement and owned range diagnostic. -/
theorem wp_nextField_integer_overflow (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs payload rem : List w8)
    (head : w8) (argument : w64) (qf qo dq : DFrac) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (enc : HeadArgument head payload argument)
    (hm : head &&& W8 224 = W8 0 ∨ head &&& W8 224 = W8 32) (hrange : 2^63 ≤ uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       globalAddr cursor.ErrType ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗
       v.buf' ↦*{dq} xs ∗ err ↦ cursor.DecodeError.mk kind v.pos' (go!"integer key does not fit int64") slice.nil }} := by
  have hmajor : head &&& W8 224 ≠ W8 96 := by rcases hm with hm | hm <;> rw [hm] <;> decide
  have hmiss := inlineFieldTable_head_miss order names head (payload ++ rem) htable (nontext_head_outside head hmajor)
  exact wp_nextField_reject c st fields v v s f order xs ((head :: payload) ++ rem) qf qo dq
    iprop(globalAddr cursor.ErrType ↦□ kind)
    (fun err => iprop(err ↦ cursor.DecodeError.mk kind v.pos' (go!"integer key does not fit int64") slice.nil))
    hfields hleft hstart hnonempty hsorted hcache (fun j entry _ he => hmiss j entry he) hp hbytes
    (wp_otherField_integer_overflow c st v xs payload rem head argument dq kind enc hm hrange hp hbytes)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_head_miss
#print axioms Perennial.github_com.NethermindEth.juno.core.nontext_head_outside
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_integer

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_integer_overflow
