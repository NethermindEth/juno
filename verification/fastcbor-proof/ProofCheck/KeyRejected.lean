import ProofCheck.KeyText
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
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Every independently specified Key rejection, including arbitrary invalid
    UTF-8. Only the existing validator premise remains unresolved. -/
theorem wp_Key_rejected (validator : Utf8ValidatorContract (GF := GF))
    (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : KeyFailureReason) (kind : GoInterface) (failure : KeyRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       keyFailureAddr (GF := GF) reason ↦□ kind ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Key")) (Val #()))
    {{ (err : Loc), RET (PairV (PairV #slice.nil #false) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ KeyFailureError err v.pos' reason kind }} := by
  wp_start_folded as ⟨#Hkind, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  cases failure with
  | @head head tail reason fault =>
    have hinfo := headFault_info head reason fault
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_singleton, Int.natCast_one] at Hnext; omega
    cases fault with
    | indefinite info major =>
      isimp only [keyFailureAddr, headFaultAddr] at Hkind
      isimp only [KeyFailureError, HeadFaultError] at HΦ
      wp_apply wp_Key_head_indefinite c v xs tail head dq kind hp.1 hlt hi hbytes hinfo info major
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    | breakCode info major =>
      isimp only [keyFailureAddr, headFaultAddr] at Hkind
      isimp only [KeyFailureError, HeadFaultError] at HΦ
      wp_apply wp_Key_head_break c v xs tail head dq kind hp.1 hlt hi hbytes hinfo info major
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
    | invalid info indefinite breakCode =>
      isimp only [keyFailureAddr, headFaultAddr] at Hkind
      isimp only [KeyFailureError, HeadFaultError] at HΦ
      wp_apply wp_Key_head_invalid c v xs tail head dq kind hp.1 hlt hi hbytes info indefinite breakCode
        $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
      · iframe Hkind Hc Hs
      wp_end
  | @incomplete head tail size info sizeEq short =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_singleton, Int.natCast_one] at Hnext; omega
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    have hshort : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen short ⊢; omega
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_head_truncated c v xs tail head size dq kind hp.1 hlt hi hbytes info sizeEq hshort
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | eof =>
    ihave %Hwf := ownSlice_wf _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_key_eof c v kind heof ⟨by omega, Hwf.2⟩ $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    wp_end
  | @integerRange head payload tail argument enc major bound =>
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_integer_overflow c v xs payload tail head argument dq kind enc major bound hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @tag head payload tail argument enc major =>
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_tag c v xs payload tail head argument dq kind enc
      (by rw [major]; decide) (by rw [major]; decide) (by rw [major]; decide) major hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @typeMismatch head payload tail argument enc ht hu hn htag =>
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_type c v xs payload tail head argument dq kind enc ht hu hn htag hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @textTruncated head payload tail argument enc major short =>
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_text_truncated c v xs payload tail head argument dq kind enc major short hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end
  | @invalidUTF8 head payload body rem argument enc major count invalid =>
    have heq : argument = W64 body.length := by word
    subst argument
    isimp only [keyFailureAddr] at Hkind
    isimp only [KeyFailureError] at HΦ
    wp_apply wp_Key_invalid_utf8 validator c v xs payload body rem head dq kind enc major invalid hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_Key_rejected
