import ProofCheck.BytesSpec
import ProofCheck.BytesRead

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

def BytesError (err : Loc) (pos : w64) (kinds : FeltErrorKinds) : BytesFailureReason → IProp GF
  | .expected reason => ExpectedFailureError err pos (W8 64) reason (expectedFailureKind reason kinds)
  | .truncated => iprop(err ↦ cursor.DecodeError.mk kinds.truncatedKind pos (go!"byte or text string longer than the input") slice.nil)

/-- Every independent successful byte-string encoding is copied into a fresh
    nonnil slice, preserving the input and restoring its exact suffix. -/
theorem wp_bytes_valid (c : Loc) (v : cursor.Cursor)
    (xs item body rest : List w8) (dq : DFrac) (valid : BytesValid item body)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (dst : GoSlice), RET (PairV #dst #interface.nil); dst ↦* body ∗ ⌜dst.ptr ≠ null⌝ ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  cases valid with
  | @bytes head payload body argument enc major count =>
    have argEq : argument = W64 body.length := by word
    subst argument
    have length : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
    simpa only [length, Int.add_assoc] using
      wp_bytes_encoded (GF := GF) c v xs payload body rest head dq enc major hp hbytes

/-- Every rejection returns nil and preserves the cursor. Error
    categories are the actual initialized objects, with exact owned arguments. -/
theorem wp_bytes_rejected (c : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BytesFailureReason) (kinds : FeltErrorKinds)
    (failure : BytesRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       FeltErrorGlobals kinds ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (err : Loc), RET (PairV #slice.nil #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BytesError err v.pos' kinds reason }} := by
  wp_start_folded as ⟨#Hglobals, Hc, Hs⟩
  cases failure with
  | expected failure =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 64) _) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind] at Hkind
    wp_apply wp_bytes_expected c v xs wire dq _ _ failure hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [BytesError]
    iframe Hc Hs Herr
  | @truncated head payload tail argument enc major short =>
    ihave #Hkind := feltErrorGlobals_lookup kinds (.expected (W8 64) .eof) $$ Hglobals
    isimp only [feltFailureAddr, feltFailureKind, expectedFailureAddr, expectedFailureKind] at Hkind
    wp_apply wp_bytes_truncated c v xs payload tail head argument dq kinds.truncatedKind enc major short hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hkind Hc Hs
    iapply HΦ
    isimp only [BytesError]
    iframe Hc Hs Herr

def BytesReadOutcome (c : Loc) (dst : GoSlice) (v : cursor.Cursor) (xs wire : List w8)
    (kinds : FeltErrorKinds) (result : GoInterface) : BytesResult → IProp GF
  | .success body rest => iprop(∃ consumed : Nat,
      dst ↦* body ∗ ⌜dst.ptr ≠ null⌝ ∗ c ↦ {v with pos' := v.pos' + W64 consumed} ∗
      ⌜result = interface.nil ∧ consumed + rest.length = wire.length ∧
        sint.Z (v.pos' + W64 consumed) = sint.Z v.pos' + consumed ∧
        sint.Z (v.pos' + W64 consumed) ≤ sint.Z v.buf'.len ∧
        subslice (sint.nat (v.pos' + W64 consumed)) (sint.nat v.buf'.len) xs = rest⌝)
  | .failure reason => iprop(c ↦ v ∗ ∃ err : Loc,
      ⌜dst = slice.nil ∧ result = interface.mkOk (go.PointerType cursor.DecodeError.ty) #err⌝ ∗
      BytesError err v.pos' kinds reason)

/-- The public allocating Bytes implementation on every input. No UTF-8, copy, payload or initializer correctness is assumed. -/
theorem wp_bytes_complete (c : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bytes")) (Val #()))
    {{ (dst : GoSlice) (result : GoInterface), RET (PairV #dst #result);
       v.buf' ↦*{dq} xs ∗ ∃ (kinds : FeltErrorKinds) (outcome : BytesResult),
       FeltErrorGlobals kinds ∗ ⌜BytesSpec wire outcome ∧ ∀ other, BytesSpec wire other → other = outcome⌝ ∗
       BytesReadOutcome c dst v xs wire kinds result outcome }} := by
  wp_start_folded as ⟨Hc, Hs⟩
  ihave Hglobals := cursor_initialized_felt_globals (GF := GF) $$ []
  · solve_pkg_init
  icases Hglobals with ⟨%kinds, #Hglobals⟩
  obtain ⟨outcome, spec, unique⟩ := bytesSpec_total_unique wire
  cases outcome with
  | success body rest =>
    obtain ⟨item, split, valid⟩ := spec
    wp_apply wp_bytes_valid c v xs item body rest dq valid hp (hbytes.trans split)
      $$ [Hc Hs] as %dst ⟨Hdst, %Hnonnull, Hc, Hs, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BytesResult.success body rest
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨⟨item, split, valid⟩, unique⟩
    isimp only [BytesReadOutcome]
    iexists item.length
    iframe Hdst Hc
    isplitl []
    · ipureintro; exact Hnonnull
    ipureintro; exact ⟨trivial, by rw [split, List.length_append], Hnext⟩
  | failure reason =>
    wp_apply wp_bytes_rejected c v xs wire dq reason kinds spec hp hbytes
      $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
    · iframe Hglobals Hc Hs
    iapply HΦ
    iframe Hs
    iexists kinds, BytesResult.failure reason
    iframe Hglobals
    isplitl []
    · ipureintro; exact ⟨spec, unique⟩
    isimp only [BytesReadOutcome]
    iframe Hc
    iexists err
    iframe Herr
    ipureintro; exact ⟨trivial, rfl⟩

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bytes_complete
