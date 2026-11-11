import ProofCheck.BoolSpec
import ProofCheck.FeltComplete

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

/-- Reuse the initialized error-category inventory, without claiming that Bool
    executes the generic head parser or has its diagnostic classification. -/
def boolExpectedReason : BoolFailureReason → ExpectedFailureReason
  | .eof => .eof
  | .mismatch head => if head &&& W8 224 = W8 192 then .tag else .typeMismatch head

def BoolFailureError (err : Loc) (pos : w64) (reason : BoolFailureReason)
    (kind : GoInterface) : IProp GF :=
  match reason with
  | .eof => err ↦ cursor.DecodeError.mk kind pos (go!"input ends before the item") slice.nil
  | .mismatch head =>
    if head &&& W8 224 = W8 192 then iprop(
      ∃ args : GoSlice,
        err ↦ cursor.DecodeError.mk kind pos (go!"tag where %s is expected") args ∗
        args ↦* [interface.mkOk go.string #(go!"a boolean")])
    else iprop(
      ∃ args : GoSlice,
        err ↦ cursor.DecodeError.mk kind pos (go!"expected %s, found %s") args ∗
        args ↦* [interface.mkOk go.string #(go!"a boolean"),
          interface.mkOk go.string #(headDescription head)])

/-- The actual primitive consumes exactly the Boolean byte at any valid input
    offset, retaining the entire input and all other cursor fields. -/
theorem wp_bool_valid (c : Loc) (v : cursor.Cursor) (xs rest : List w8)
    (value : Bool) (dq : DFrac)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      (if value then W8 245 else W8 244) :: rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bool")) (Val #()))
    {{ RET (PairV #value #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_peek_prefix c v xs rest (if value then W8 245 else W8 244) dq hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  cases value <;> wp_auto <;> wp_end

/-- Every primitive failure preserves the cursor and input and returns false
    together with the allocated diagnostic at the original cursor position. -/
theorem wp_bool_rejected (c : Loc) (v : cursor.Cursor) (xs wire : List w8)
    (dq : DFrac) (reason : BoolFailureReason) (kind : GoInterface)
    (rejected : BoolCursorRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) (boolExpectedReason reason) ↦□ kind ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Bool")) (Val #()))
    {{ (err : Loc), RET (PairV #false #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err));
       c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BoolFailureError err v.pos' reason kind }} := by
  cases rejected with
  | eof =>
    wp_start as ⟨#Hkind, Hc, Hs⟩
    isimp only [boolExpectedReason, expectedFailureAddr] at Hkind
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    wp_auto
    wp_apply wp_peek_eof c v kind heof $$ [Hc] as %err ⟨Hc, Herr⟩
    · iframe Hkind Hc
    iapply HΦ
    isimp only [BoolFailureError]
    iframe Hc Hs Herr
  | mismatch head tail hf ht =>
    wp_start as ⟨#Hkind, Hc, Hs⟩
    wp_auto
    wp_apply wp_peek_prefix c v xs tail head dq hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    simp only [hf, ht, _root_.decide_false]
    wp_auto
    simp only [ht, _root_.decide_false]
    wp_auto
    by_cases htag : head &&& W8 224 = W8 192
    · isimp only [boolExpectedReason, htag, ↓reduceIte, expectedFailureAddr] at Hkind
      wp_apply wp_mismatch_tag c v (go!"a boolean") head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hkind Hc
      iapply HΦ
      isimp only [BoolFailureError, htag, ↓reduceIte]
      iframe Hc Hs Herr
    · isimp only [boolExpectedReason, htag, ↓reduceIte, expectedFailureAddr] at Hkind
      wp_apply wp_mismatch_type c v (go!"a boolean") head kind htag $$ [Hc] as %err ⟨Hc, Herr⟩
      · iframe Hkind Hc
      iapply HΦ
      isimp only [BoolFailureError, htag, ↓reduceIte]
      iframe Hc Hs Herr

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bool_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bool_rejected
