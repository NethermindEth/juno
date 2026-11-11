import ProofCheck.BytesIntoSpec
import ProofCheck.BytesIntoMismatch

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

local instance (kinds : FeltErrorKinds) : Persistent (FeltErrorGlobals (GF := GF) kinds) := by
  unfold FeltErrorGlobals SkipErrorGlobals
  infer_instance

def BytesIntoError (err : Loc) (pos : w64) (size : Nat) (kinds : FeltErrorKinds) : BytesIntoFailureReason → IProp GF
  | .expected reason => ExpectedFailureError err pos (W8 64) reason (expectedFailureKind reason kinds)
  | .truncated => iprop(err ↦ cursor.DecodeError.mk kinds.truncatedKind pos (go!"byte or text string longer than the input") slice.nil)
  | .size actual => iprop(∃ args : GoSlice,
      err ↦ cursor.DecodeError.mk kinds.typeKind pos (go!"byte string of %d bytes, destination of %d") args ∗
      args ↦* [interface.mkOk go.int #(W64 actual), interface.mkOk go.int #(W64 size)])

/-- Every independent successful byte-string encoding is copied into exactly
    the owned destination, preserving the input and restoring its exact suffix. -/
theorem wp_bytesInto_valid (c : Loc) (dst : GoSlice) (old : List w8) (v : cursor.Cursor)
    (xs item body rest : List w8) (dq : DFrac) (valid : BytesIntoValid old.length item body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesInto")) (Val #dst))
    {{ RET #interface.nil; dst ↦* body ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  cases valid with
  | @bytes head payload body argument enc major count size =>
    have argEq : argument = W64 body.length := by word
    subst argument
    have length : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
    simpa only [length, Int.add_assoc] using
      wp_bytesInto_encoded (GF := GF) c dst old v xs payload body rest head dq enc major size hp hbytes

/-- Every rejection preserves both destination bytes and cursor. Error
    categories are the actual initialized objects, with exact owned arguments. -/
theorem wp_bytesInto_rejected (c : Loc) (dst : GoSlice) (old : List w8) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BytesIntoFailureReason) (kinds : FeltErrorKinds)
    (failure : BytesIntoRejected old.length wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesInto")) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BytesIntoError err v.pos' old.length kinds reason }} := by
  wp_start_folded as ⟨#Hglobals, Hdst, Hc, Hs⟩
  cases failure with
  | expected failure =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 64) _) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_bytesInto_expected c dst old v xs wire dq _ _ failure hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    isimp only [BytesIntoError]
    iframe Hdst Hc Hs Herr
  | @truncated head payload tail argument enc major short =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 64) .eof) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_bytesInto_truncated c dst old v xs payload tail head argument dq kinds.truncatedKind enc major short hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    isimp only [BytesIntoError]
    iframe Hdst Hc Hs Herr
  | @size head payload body rest argument enc major count different =>
    have argEq : argument = W64 body.length := by word
    subst argument
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 64) (.typeMismatch (W8 0))) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_bytesInto_size c dst old v xs payload body rest head dq kinds.typeKind enc major different hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, %args, Herr, Hargs⟩
    · iframe Hkind Hdst Hc Hs
    iapply HΦ
    isimp only [BytesIntoError]
    iframe Hdst Hc Hs
    iexists args
    iframe Herr Hargs

def BytesIntoReadOutcome (c : Loc) (dst : GoSlice) (old : List w8) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : BytesIntoResult → IProp GF
  | .success body rest => iprop(∃ consumed : Nat,
      dst ↦* body ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure reason => iprop(dst ↦* old ∗ c ↦ v ∗ ∃ err : Loc,
      ⌜result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      BytesIntoError err v.pos' old.length kinds reason)

/-- The public BytesInto implementation on every input and separately owned
    destination. No UTF-8, copy, payload or initializer correctness is assumed. -/
theorem wp_bytesInto_complete (c : Loc) (dst : GoSlice) (old : List w8) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦* old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BytesInto")) (Val #dst))
    {{ (result : GoInterface), RET #result;
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : FeltErrorKinds) (outcome : BytesIntoResult),
       FeltErrorGlobals kinds ∗ ⌜BytesIntoSpec old.length wire outcome ∧ ∀ other, BytesIntoSpec old.length wire other → other = outcome⌝ ∗
       BytesIntoReadOutcome c dst old v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := bytesIntoSpec_total_unique old.length wire
  cases outcome with
  | success body rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_bytesInto_valid c dst old v xs item body rest dq valid hp (hbytes.trans split)
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs, %Hnext⟩
    · iframe Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BytesIntoResult.success body rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [BytesIntoReadOutcome]
    iexists item.length
    iframe Hdst Hc
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure reason =>
    wp_apply wp_bytesInto_rejected c dst old v xs wire dq reason kinds spec hp hbytes
      $$ [Hdst Hc Hs] as %err ⟨Hdst, Hc, Hs, Herr⟩
    · iframe Hglobals Hdst Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BytesIntoResult.failure reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [BytesIntoReadOutcome]
    iframe Hdst Hc
    iexists err
    iframe Herr
    ipureintro; rfl

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesInto_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesInto_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytesInto_complete
