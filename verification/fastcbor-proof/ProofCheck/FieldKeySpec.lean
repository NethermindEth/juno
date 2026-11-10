import ProofCheck.KeySpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]

/-- Every cached ASCII schema entry is a complete valid text key. -/
theorem inlineFieldTable_keyValid (order : List cursor.StaticField) (names : List GoString)
    (entry : cursor.StaticField) (htable : InlineFieldTable order names)
    (ascii : ∀ name ∈ names, ASCIIBytes name) (member : entry ∈ order) :
    ∃ name, KeyValid entry.Encoded' (some name) := by
  obtain ⟨name, _, _, lookup, small, encoded⟩ := inlineFieldTable_entry order names entry htable member
  have bits : ∀ n : Fin 24,
      uint.Z (W8 (96 + n.val) &&& W8 31) = n.val ∧ W8 (96 + n.val) &&& W8 224 = W8 96 := by decide
  obtain ⟨info, major⟩ := bits ⟨name.length, small⟩
  have enc : HeadArgument (W8 (96 + name.length)) [] (W64 name.length) := by
    simpa only [info] using HeadArgument.inline (W8 (96 + name.length)) (by rw [info]; omega)
  refine ⟨name, ?_⟩
  rw [encoded]
  exact KeyValid.text enc major (by word) (utf8Valid_ascii name (ascii name (List.mem_of_getElem? lookup)))

/-- No specified key error can match a cached ASCII schema key. This includes
    every malformed UTF-8 sequence, not just an illegal leading byte. -/
theorem inlineFieldTable_keyRejected_miss (order : List cursor.StaticField) (names : List GoString)
    (wire : List w8) (reason : KeyFailureReason) (htable : InlineFieldTable order names)
    (ascii : ∀ name ∈ names, ASCIIBytes name) (failure : KeyRejected wire reason) :
    ∀ (j : Nat) (entry : cursor.StaticField), order[j]? = some entry →
      wire.take entry.Encoded'.length ≠ entry.Encoded' := by
  intro j entry lookup equal
  obtain ⟨name, valid⟩ := inlineFieldTable_keyValid order names entry htable ascii (List.mem_of_getElem? lookup)
  have split : entry.Encoded' ++ wire.drop entry.Encoded'.length = wire := by
    calc
      entry.Encoded' ++ wire.drop entry.Encoded'.length =
          wire.take entry.Encoded'.length ++ wire.drop entry.Encoded'.length :=
        congrArg (fun front => front ++ wire.drop entry.Encoded'.length) equal.symm
      _ = wire := List.take_append_drop _ _
  exact keyValid_not_rejected entry.Encoded' (wire.drop entry.Encoded'.length) (some name) reason valid
    (by rw [split]; exact failure)

variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Compose a proved Key rejection through alternate lookup, cache/search and
    public NextField. The Key WP remains explicit; no validator assumption is
    hidden in the wrapper and no missing UTF-8 implementation proof is claimed. -/
theorem wp_nextField_keyRejected (c st fields : Loc) (v vout : cursor.Cursor)
    (s : cursor.Struct) (f : cursor.Fields) (order : List cursor.StaticField) (names : List GoString)
    (xs wire : List w8) (reason : KeyFailureReason) (qf qo dq : DFrac)
    (R : IProp GF) (E : Loc → IProp GF)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (ascii : ∀ name ∈ names, ASCIIBytes name) (failure : KeyRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire)
    (hKey :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
      {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ vout ∗ v.buf' ↦*{dq} xs ∗ E err }}) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       R ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗ f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ vout ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
  have hother :
      {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
         R ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
        (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"otherField")) (Val #st))
      {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
         c ↦ vout ∗ v.buf' ↦*{dq} xs ∗ E err }} := by
    wp_start as ⟨HR, Hc, Hs⟩
    wp_auto
    wp_apply hKey $$ [HR Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe HR Hc Hs
    wp_end
  exact wp_nextField_reject c st fields v vout s f order xs wire qf qo dq R E
    hfields hleft hstart hnonempty hsorted hcache
    (fun j entry _ lookup => inlineFieldTable_keyRejected_miss order names wire reason htable ascii failure j entry lookup)
    hp hbytes hother

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_keyValid
#print axioms Perennial.github_com.NethermindEth.juno.core.inlineFieldTable_keyRejected_miss
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_keyRejected
