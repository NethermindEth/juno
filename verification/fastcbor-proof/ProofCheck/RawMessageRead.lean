import ProofCheck.RawMessageSpec
import ProofCheck.BytesComplete

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

/-- Null is distinct from an allocated, possibly empty byte payload. -/
def RawMessageOwned (out : GoSlice) : Option (List w8) → IProp GF
  | none => iprop(⌜out = slice.nil⌝)
  | some body => iprop(out ↦* body ∗ ⌜out.ptr ≠ null⌝)

/-- The actual RawMessage reader publishes nil or a complete fresh byte copy,
    preserving the input and returning exact consumption and suffix facts. -/
theorem wp_readRawMessage_valid (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option (List w8)) (dq : DFrac) (valid : RawMessageValid item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadRawMessage)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil; dst ↦ out ∗ RawMessageOwned out value ∗
       c ↦ {v with pos' := v.pos' + W64 item.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 item.length) = sint.Z v.pos' + item.length ∧
         sint.Z (v.pos' + W64 item.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 item.length)) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs item rest v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rest (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ $$ %slice.nil
    isimp only [RawMessageOwned]
    iframe Hdst Hc Hs
    ipureintro
    exact Hnext
  | @bytes item body valid =>
    have hn : item.head? ≠ some (W8 246) ∧ item.head? ≠ some (W8 247) := by
      cases valid with
      | bytes enc major count =>
        simp only [List.cons_append, List.head?_cons, ne_eq, Option.some.injEq]
        constructor <;> intro h <;> subst h <;> contradiction
    have hnWire : (item ++ rest).head? = item.head? := by
      cases item with
      | nil => have := bytesValid_nonempty _ _ valid; simp at this
      | cons => rfl
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq (by rw [hnWire]; exact hn.1)
      (by rw [hnWire]; exact hn.2) hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_bytes_valid c v xs item body rest dq valid hp hbytes
      $$ [Hc Hs] as %out ⟨Hout, %Hnonnull, Hc, Hs, %Hnext2⟩
    · iframe Hc Hs
    iapply HΦ $$ %out
    isimp only [RawMessageOwned]
    iframe Hdst Hout Hc Hs
    ipureintro; exact ⟨Hnonnull, Hnext2⟩

/-- Rejected RawMessages preserve their prior slice header and cursor, and
    return the initialized byte-string diagnostic with owned formatting arguments. -/
theorem wp_readRawMessage_rejected (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BytesFailureReason) (kinds : FeltErrorKinds)
    (failure : RawMessageRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       FeltErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadRawMessage)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BytesError err v.pos' kinds reason }} := by
  wp_start as ⟨Hglobals, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq failure.1 failure.2.1 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_bytes_rejected c v xs wire dq reason kinds failure.2.2 hp hbytes
    $$ [Hc Hs Hglobals] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hglobals Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readRawMessage_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readRawMessage_rejected
