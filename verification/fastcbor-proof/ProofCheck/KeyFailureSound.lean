import ProofCheck.KeyFailureSpec

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The public field reader realizes every specified key rejection, preserving
    input, cursor, search index, seen bits and open state. Only the remaining
    pair count decreases; all diagnostic allocations are explicitly owned. -/
theorem wp_nextField_failure (c st fields : Loc) (v : cursor.Cursor) (s : cursor.Struct) (f : cursor.Fields)
    (order : List cursor.StaticField) (names : List GoString) (xs wire : List w8)
    (qf qo dq : DFrac) (reason : KeyFailureReason) (kind : GoInterface)
    (hfields : s.fields' = fields) (hleft : 0 < sint.Z s.left')
    (hstart : 0 ≤ sint.Z s.next' ∧ (sint.Z s.next').toNat ≤ order.length)
    (hnonempty : ∀ entry ∈ order, entry.Encoded' ≠ []) (hsorted : FieldHeadsOrdered order)
    (hcache : CachedShortTable order) (htable : InlineFieldTable order names)
    (failure : KeyFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ st ↦ s ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"NextField")) (Val #st))
    {{ (err : Loc), RET (PairV #(W64 (-1)) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ st ↦ {s with left' := s.left' - W64 1} ∗ fields ↦{qf} f ∗
       f.order' ↦*{qo} order ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  cases failure with
  | @head head tail reason fault =>
    have hinfo := headFault_info head reason fault
    cases fault with
    | indefinite info major =>
      simpa only [keyFailureAddr, headFaultAddr, KeyFailureError, HeadFaultError] using
        wp_nextField_head_indefinite (GF := GF) c st fields v s f order names xs tail head qf qo dq kind
          hfields hleft hstart hnonempty hsorted hcache htable hp hbytes hinfo info major
    | breakCode info major =>
      simpa only [keyFailureAddr, headFaultAddr, KeyFailureError, HeadFaultError] using
        wp_nextField_head_break (GF := GF) c st fields v s f order names xs tail head qf qo dq kind
          hfields hleft hstart hnonempty hsorted hcache htable hp hbytes hinfo info major
    | invalid info indefinite breakCode =>
      simpa only [keyFailureAddr, headFaultAddr, KeyFailureError, HeadFaultError] using
        wp_nextField_head_invalid (GF := GF) c st fields v s f order names xs tail head qf qo dq kind
          hfields hleft hstart hnonempty hsorted hcache htable hp hbytes info indefinite breakCode
  | @incomplete head tail size info sizeEq short =>
    wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    have hshort : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen short ⊢; omega
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_nextField_head_truncated c st fields v s f order names xs tail head size qf qo dq kind
      hfields hleft hstart hnonempty hsorted hcache htable hp hbytes info sizeEq hshort
      $$ [Hc Hst Hfields Horder Hs] as %err ⟨Hc, Hst, Hfields, Horder, Hs, Herr⟩
    · iframe Hkind Hc Hst Hfields Horder Hs
    wp_end
  | eof =>
    wp_start_folded as ⟨#Hkind, Hc, Hst, Hfields, Horder, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_nextField_eof c st fields v s f kind qf hfields hleft heof ⟨by omega, Hwf.2⟩
      $$ [Hc Hst Hfields] as %err ⟨Hc, Hst, Hfields, Herr⟩
    · iframe Hkind Hc Hst Hfields
    wp_end
  | @integerRange head payload tail argument enc major bound =>
    simpa only [keyFailureAddr, KeyFailureError] using
      wp_nextField_integer_overflow (GF := GF) c st fields v s f order names xs payload tail head argument qf qo dq kind
        hfields hleft hstart hnonempty hsorted hcache htable enc major bound hp hbytes
  | @tag head payload tail argument enc major =>
    simpa only [keyFailureAddr, KeyFailureError] using
      wp_nextField_key_tag (GF := GF) c st fields v s f order names xs payload tail head argument qf qo dq kind
        hfields hleft hstart hnonempty hsorted hcache htable enc (by rw [major]; decide) (by rw [major]; decide) (by rw [major]; decide) major hp hbytes
  | @typeMismatch head payload tail argument enc text uint negative tag =>
    simpa only [keyFailureAddr, KeyFailureError] using
      wp_nextField_key_type (GF := GF) c st fields v s f order names xs payload tail head argument qf qo dq kind
        hfields hleft hstart hnonempty hsorted hcache htable enc text uint negative tag hp hbytes
  | @textTruncated head payload tail argument enc major short =>
    simpa only [keyFailureAddr, KeyFailureError] using
      wp_nextField_text_truncated (GF := GF) c st fields v s f order names xs payload tail head argument qf qo dq kind
        hfields hleft hstart hnonempty hsorted hcache htable enc major short hp hbytes

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_nextField_failure
