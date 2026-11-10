import ProofCheck.BigIntPayloadFailure
import ProofCheck.InitializationErrors

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

structure BigIntErrorKinds where
  truncated : Loc
  syntaxError : Loc
  typeError : Loc
  unsupported : Loc

abbrev bigIntCategory (p : Loc) : GoInterface := interface.mkOk (go.PointerType errors.errorString.ty) #p

def BigIntErrorGlobals (k : BigIntErrorKinds) : IProp GF := iprop(
  globalAddr cursor.ErrTruncated ↦□ bigIntCategory k.truncated ∗
  globalAddr cursor.ErrSyntax ↦□ bigIntCategory k.syntaxError ∗
  globalAddr cursor.ErrType ↦□ bigIntCategory k.typeError ∗
  globalAddr cursor.ErrUnsupported ↦□ bigIntCategory k.unsupported ∗
  ⌜k.truncated ≠ k.typeError ∧ k.syntaxError ≠ k.typeError ∧ k.unsupported ≠ k.typeError⌝)

instance bigIntErrorGlobals_persistent (k : BigIntErrorKinds) : Persistent (BigIntErrorGlobals (GF := GF) k) := by
  unfold BigIntErrorGlobals
  infer_instance

def bigIntHeadKind (k : BigIntErrorKinds) : HeadError → Loc
  | .indefinite => k.unsupported
  | .breakCode | .invalid => k.syntaxError

def bigIntExpectedKind (k : BigIntErrorKinds) : ExpectedFailureReason → Loc
  | .head _ reason => bigIntHeadKind k reason
  | .incomplete | .eof => k.truncated
  | .typeMismatch _ => k.typeError
  | .tag => k.unsupported

def BigIntFailureError (err : Loc) (pos : w64) (reason : BigIntFailureReason)
    (k : BigIntErrorKinds) : IProp GF :=
  match reason with
  | .head bad reason => HeadFaultError err pos bad reason (bigIntCategory (bigIntHeadKind k reason))
  | .incomplete => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated) pos (go!"input ends inside the head") slice.nil)
  | .eof => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated) pos (go!"input ends before the item") slice.nil)
  | .typeMismatch bad => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk (bigIntCategory k.typeError) pos (go!"expected %s, found %s") args ∗
      args ↦* [interface.mkOk go.string #(go!"an integer"), interface.mkOk go.string #(headDescription bad)])
  | .tag tag => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk (bigIntCategory k.unsupported) pos (go!"tag %d where an integer is expected") args ∗
      args ↦* [interface.mkOk go.uint64 #tag])
  | .payloadExpected _ (.typeMismatch _) =>
      iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.syntaxError) pos (go!"tag 2 or 3 must hold a byte string") slice.nil)
  | .payloadExpected width reason => ExpectedFailureError err (pos + W64 width) (W8 64) reason
      (bigIntCategory (bigIntExpectedKind k reason))
  | .payloadTruncated width => iprop(err ↦ cursor.DecodeError.mk (bigIntCategory k.truncated)
      (pos + W64 width) (go!"byte or text string longer than the input") slice.nil)

/-- All specified rejections are realized by the public decoder with rollback,
    unchanged input and the exact diagnostic object and argument ownership. -/
theorem wp_bigInt_rejected (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (reason : BigIntFailureReason) (k : BigIntErrorKinds) (failure : BigIntRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       BigIntErrorGlobals k ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (err : Loc), RET (PairV #null #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BigIntFailureError err v.pos' reason k }} := by
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
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_head_fault c v xs head dq reason (bigIntCategory (bigIntHeadKind k reason))
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
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_head_truncated c v xs head size dq (bigIntCategory k.truncated)
      hp.1 hlt hi info sizeEq hshort $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | eof =>
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_eof c v (bigIntCategory k.truncated) heof $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @typeMismatch head payload argument tail enc hu hn ht =>
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_type c v xs payload tail head argument dq (bigIntCategory k.typeError)
      enc hu hn ht hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @tag head payload argument tail enc major h2 h3 =>
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_tag_rejected c v xs payload tail head argument dq (bigIntCategory k.unsupported)
      enc major h2 h3 hp hbytes $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end
  | @payloadExpected tagHead tagBytes tail tag reason enc major allowed failure =>
    ihave #Hkind : (expectedFailureAddr (GF := GF) reason ↦□ bigIntCategory (bigIntExpectedKind k reason) : IProp GF) $$ []
    · cases reason with
      | head _ reason => cases reason <;> isimp only [expectedFailureAddr, headFaultAddr, bigIntExpectedKind, bigIntHeadKind] <;> (iframe # <;> iframe)
      | incomplete | eof | typeMismatch _ | tag => isimp only [expectedFailureAddr, bigIntExpectedKind]; iframe # <;> iframe
    wp_apply wp_bigInt_payload_expected c v xs tagBytes tail tagHead tag dq reason (bigIntExpectedKind k reason)
      k.typeError (bigIntCategory k.syntaxError) enc major allowed failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    iapply HΦ
    iframe Hc Hs
    cases reason with
    | head bad reason =>
      cases reason <;> isimp only [bigIntExpectedKind, bigIntHeadKind, Hne.2.1, Hne.2.2, ite_false, BigIntFailureError] at Herr <;> isimp only [BigIntFailureError, bigIntExpectedKind, bigIntHeadKind] <;> iframe
    | incomplete | eof | typeMismatch _ | tag =>
      isimp only [bigIntExpectedKind, Hne.1, Hne.2.2, ite_false, ite_true, BigIntFailureError] at Herr
      isimp only [BigIntFailureError, bigIntExpectedKind]
      iframe
  | @payloadTruncated tagHead bodyHead tagBytes lengthBytes tail tag count enc major allowed bodyEnc bodyMajor short =>
    isimp only [BigIntFailureError] at HΦ
    wp_apply wp_bigInt_payload_truncated c v xs tagBytes lengthBytes tail tagHead bodyHead tag count dq
      k.truncated k.typeError Hne.1 enc major allowed bodyEnc bodyMajor short hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe # <;> iframe
    wp_end

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_rejected
