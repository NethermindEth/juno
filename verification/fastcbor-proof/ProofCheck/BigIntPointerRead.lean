import ProofCheck.BigIntPointerSpec
import ProofCheck.BigIntComplete

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

/-- Nil is distinct from an owned, normalized mathematical integer, including zero. -/
def BigIntPointerOwned (out : Loc) : Option Int → IProp GF
  | none => iprop(⌜out = null⌝)
  | some body => BigMagnitudeOwned out body

/-- A successful integer value, including zero, owns a nonnil object. -/
theorem bigIntPointerOwned_some_nonnull (out : Loc) (value : Int) :
    (BigIntPointerOwned out (some value) : IProp GF) ⊢ ⌜out ≠ null⌝ := by
  iintro Howned
  isimp only [BigIntPointerOwned, BigMagnitudeOwned] at Howned
  icases Howned with ⟨%storage, %words, %negative, Hobject, Hwords, %Hvalue⟩
  ihave %Hnonnull := typedPointsto_not_null _ _ _ $$ Hobject
  ipureintro; exact Hnonnull

/-- The actual BigIntPointer reader publishes nil or a complete new integer,
    preserving the input and returning exact consumption and suffix facts. -/
theorem wp_readBigIntPointer_valid (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs item rest : List w8) (value : Option Int) (dq : DFrac) (valid : BigIntPointerValid item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBigIntPointer)) (Val #c)) (Val #dst))
    {{ (out : Loc), RET #interface.nil; dst ↦ out ∗ BigIntPointerOwned out value ∗
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
    iapply HΦ $$ %null
    isimp only [BigIntPointerOwned]
    iframe Hdst Hc Hs
    ipureintro
    exact Hnext
  | @integer item body valid =>
    have hn := bigIntValid_not_nullish item rest body valid
    wp_apply wp_null_not_nullish c v xs (item ++ rest) dq hn.1 hn.2 hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_bigInt_valid c v xs item rest body dq valid hp hbytes
      $$ [Hc Hs] as %out ⟨Hc, Hs, Hout, %Hnext2⟩
    · iframe Hc Hs
    iapply HΦ $$ %out
    isimp only [BigIntPointerOwned]
    iframe Hdst Hout Hc Hs
    ipureintro; exact Hnext2

/-- Rejected BigIntPointers preserve their prior pointer and cursor, and
    return the initialized integer diagnostic with owned formatting arguments. -/
theorem wp_readBigIntPointer_rejected (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : BigIntFailureReason) (kinds : BigIntErrorKinds)
    (failure : BigIntPointerRejected wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       BigIntErrorGlobals kinds ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBigIntPointer)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ BigIntFailureError err v.pos' reason kinds }} := by
  wp_start as ⟨Hglobals, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq failure.1 failure.2.1 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_bigInt_rejected c v xs wire dq reason kinds failure.2.2 hp hbytes
    $$ [Hc Hs Hglobals] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hglobals Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBigIntPointer_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readBigIntPointer_rejected
#print axioms Perennial.github_com.NethermindEth.juno.core.bigIntPointerOwned_some_nonnull
