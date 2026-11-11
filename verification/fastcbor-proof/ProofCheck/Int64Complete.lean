import ProofCheck.Int64Read
import ProofCheck.Int64Spec
import ProofCheck.BigIntComplete

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

def Int64FailureError (err : Loc) (pos : w64) (reason : Int64FailureReason)
    (k : BigIntErrorKinds) : IProp GF :=
  match reason with
  | .head bad reason => HeadFaultError err pos bad reason (bigIntCategory (bigIntHeadKind k reason))
  | .incomplete => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated) pos (go!"input ends inside the head") slice.nil)
  | .eof => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated) pos (go!"input ends before the item") slice.nil)
  | .typeMismatch bad => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk (bigIntCategory k.typeError) pos (go!"expected %s, found %s") args ∗
      args ↦* [interface.mkOk go.string #(go!"an integer"), interface.mkOk go.string #(headDescription bad)])
  | .tag => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk (bigIntCategory k.unsupported) pos (go!"tag where %s is expected") args ∗
      args ↦* [interface.mkOk go.string #(go!"an integer")])
  | .overflow => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.typeError) pos
      (go!"integer does not fit int64") slice.nil)

/-- All specified rejections are realized by the public decoder with rollback,
    unchanged input and the exact diagnostic object and argument ownership. -/
theorem wp_int64_rejected (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : Int64FailureReason) (k : BigIntErrorKinds) (failure : Int64Rejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       BigIntErrorGlobals k ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Int64")) (Val #()))
    {{ (err : Loc), RET (PairV #(W64 0) #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ Int64FailureError err v.pos' reason k }} := by
  iintro %Φ ⟨#Hpkg, #Hglobals, Hc, Hs⟩ HΦ
  isimp only [BigIntErrorGlobals] at Hglobals
  icases Hglobals with ⟨#Htruncated, #Hsyntax, #Htype, #Hunsupported, %Hne⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  cases failure with
  | @head head reason tail fault =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_cons, List.length_nil] at Hnext; omega
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_head_fault c v xs head dq reason (bigIntCategory (bigIntHeadKind k reason))
      fault hp.1 hlt hi $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · cases reason <;> isimp only [headFaultAddr, bigIntHeadKind] <;> (iframe # <;> iframe)
    wp_end
  | @incomplete head tail size info sizeEq short =>
    have hi := input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] tail v.pos' v.buf'.len hp Hlen.1 hbytes
    have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
      simp only [List.length_cons, List.length_nil] at Hnext; omega
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_cons] at hlen
    have hshort : sint.Z v.buf'.len - (sint.Z v.pos' + 1) < sint.Z size := by
      simp only [sint.nat, sint.Z] at Hlen hp hlen short ⊢; omega
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_head_truncated c v xs head size dq (bigIntCategory k.truncated)
      hp.1 hlt hi info sizeEq hshort $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | eof =>
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_eof c v (bigIntCategory k.truncated) heof $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @typeMismatch head payload argument tail enc hu hn ht =>
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_type c v xs payload tail head argument dq (bigIntCategory k.typeError)
      enc hu hn ht hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @tag head payload argument tail enc major =>
    have hu : head &&& W8 224 ≠ W8 0 := by rw [major]; decide
    have hn : head &&& W8 224 ≠ W8 32 := by rw [major]; decide
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_tag c v xs payload tail head argument dq (bigIntCategory k.unsupported)
      enc hu hn major hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @overflow head payload argument tail enc major bound =>
    isimp only [Int64FailureError] at HΦ
    wp_apply wp_int64_overflow c v xs payload tail head argument dq (bigIntCategory k.typeError)
      enc major bound hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end

/-- Valid signed integers have their specified mathematical value and exact suffix. -/
theorem wp_int64_valid (c : Loc) (v : cursor.Cursor) (xs item rest : List w8)
    (number : Int) (dq : DFrac) (valid : Int64Valid item number)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Int64")) (Val #()))
    {{ (value : w64), RET (PairV #value #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z value = number ∧
         sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  cases valid with
  | @unsigned head payload argument enc major bound =>
    wp_apply wp_int64_encoded c v xs payload rest head argument dq enc (Or.inl major) bound hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    isimp only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
    iframe Hc Hs
    ipureintro
    constructor
    · simp only [major, show W8 0 ≠ W8 32 by decide, ite_false]; word
    · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hnext
  | @negative head payload argument enc major bound =>
    wp_apply wp_int64_encoded c v xs payload rest head argument dq enc (Or.inr major) bound hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    isimp only [List.length_cons, Int.natCast_add, Int.natCast_one, Int.add_comm (payload.length : Int) 1]
    iframe Hc Hs
    ipureintro
    constructor
    · simp only [major, ite_true]; word
    · simpa only [Int.natCast_add, Int.natCast_one, Int.add_assoc] using Hnext

/-- The observable result of an Int64 call: an independently specified integer
    and exact consumption, or a specified rejection with complete rollback. -/
def Int64Outcome (c : Loc) (before : cursor.Cursor) (wire : List w8) (xs : List w8)
    (value : w64) (result : GoInterface) : IProp GF := iprop(
  ∃ k : BigIntErrorKinds, BigIntErrorGlobals k ∗
    ((∃ (item rest : List w8) (number : Int),
      ⌜result = interface.nil ∧ wire = item ++ rest ∧ Int64Valid item number⌝ ∗
      c ↦ {before with pos' := before.pos' + W64 item.length} ∗
      ⌜sint.Z value = number ∧ sint.Z (before.pos' + W64 item.length) = sint.Z before.pos' + item.length ∧
        sint.Z (before.pos' + W64 item.length) ≤ sint.Z before.buf'.len ∧
        subslice (sint.nat (before.pos' + W64 item.length)) (sint.nat before.buf'.len) xs = rest⌝) ∨
     (∃ (reason : Int64FailureReason) (err : Loc),
      ⌜value = W64 0 ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err ∧
        Int64Rejected wire reason⌝ ∗ c ↦ before ∗ Int64FailureError err before.pos' reason k)))

/-- The actual public Int64 decoder on every byte input at any valid cursor
    offset. The independent encoding/rejection relations determine the result;
    no correctness premise for head parsing or integer conversion remains. -/
theorem wp_int64_complete (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Int64")) (Val #()))
    {{ (value : w64) (result : GoInterface), RET (PairV #value #result);
       v.buf' ↦*{dq} xs ∗ Int64Outcome c v wire xs value result }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave Hglobals := cursor_initialized_bigInt_globals (GF := GF) $$ Hpkg
  icases Hglobals with ⟨%k, #Hglobals⟩
  rcases int64_classify wire with ⟨item, rest, number, split, valid⟩ | ⟨reason, failure⟩
  · wp_apply wp_int64_valid c v xs item rest number dq valid hp (by simpa only [split] using hbytes)
      $$ [Hc Hs] as %value ⟨Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    unfold Int64Outcome
    iexists k
    iframe Hglobals
    ileft
    iexists item, rest, number
    iframe Hc
    ipureintro; exact ⟨⟨rfl, split, valid⟩, Hnext⟩
  · wp_apply wp_int64_rejected c v xs wire dq reason k failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    unfold Int64Outcome
    iexists k
    iframe Hglobals
    iright
    iexists reason, err
    iframe Hc Herr
    ipureintro; exact ⟨rfl, rfl, failure⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_int64_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_int64_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_int64_complete
