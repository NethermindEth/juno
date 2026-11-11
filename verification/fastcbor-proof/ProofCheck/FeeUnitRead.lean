import ProofCheck.FeeUnitSpec
import ProofCheck.InitializationErrors

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

/-- The actual byte-backed field reader preserves nullish fields and assigns
    every representable unsigned byte, including non-preferred wire widths. -/
theorem wp_readFeeUnit_valid (c dst : Loc) (old : w8) (v : cursor.Cursor)
    (xs item rest : List w8) (value : w8) (dq : DFrac) (valid : FeeUnitValid old item value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = item ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeeUnit)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ value ∗
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
    wp_end
  | @unsigned head payload argument enc major fits =>
    have hi := input_suffix_head xs (payload ++ rest) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
      constructor <;> intro h <;> subst head <;> contradiction
    wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_uint64_wire c v xs rest ⟨head, payload, argument, major, enc⟩ dq hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
    · iframe Hc Hs
    have hgood : ¬ uint.Z (W64 255) < uint.Z argument := by change ¬ (255 : Int) < uint.Z argument; omega
    simp only [hgood, _root_.decide_false]
    wp_auto
    iapply HΦ
    isimp only [UnsignedWire.bytes] at Hc
    iframe Hdst Hc Hs
    ipureintro
    exact Hnext

/-- Unsigned parser failure returns its allocated diagnostic, restoring cursor
    and preserving the old byte. This is distinct from width overflow. -/
theorem wp_readFeeUnit_expected (c dst : Loc) (old : w8) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeeUnit)) (Val #c)) (Val #dst))
    {{ (err : Loc), RET #(interface.mkOk (go.PointerType cursor.DecodeError.ty) #err);
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs ∗ ExpectedFailureError err v.pos' (W8 0) reason kind }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_not_nullish c v xs wire dq failure.1 failure.2.1 hp hbytes $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_failure c v xs wire dq reason kind failure.2.2 hp hbytes
    $$ [Hc Hs] as %err ⟨Hc, Hs, Herr⟩
  · iframe Hkind Hc Hs
  wp_end

/-- Width overflow consumes the entire unsigned encoding, preserves the old
    field, and returns the actual bare ErrType object without allocating a
    diagnostic. Input and cursor depth are unchanged. -/
theorem wp_readFeeUnit_overflow (c dst : Loc) (old : w8) (v : cursor.Cursor)
    (xs payload rest : List w8) (head : w8) (argument : w64) (dq : DFrac) (kind : GoInterface)
    (enc : HeadArgument head payload argument) (major : head &&& W8 224 = W8 0)
    (large : 255 < uint.Z argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rest) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       globalAddr cursor.ErrType ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeeUnit)) (Val #c)) (Val #dst))
    {{ RET #kind; dst ↦ old ∗ c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rest⌝ }} := by
  wp_start as ⟨#Hkind, Hdst, Hc, Hs⟩
  wp_auto
  have hi := input_suffix_head xs (payload ++ rest) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
    constructor <;> intro h <;> subst head <;> contradiction
  wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_wire c v xs rest ⟨head, payload, argument, major, enc⟩ dq hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  have hbad : uint.Z (W64 255) < uint.Z argument := large
  simp only [hbad, _root_.decide_true]
  wp_auto
  iapply HΦ
  isimp only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1] at Hc
  iframe Hdst Hc Hs
  ipureintro
  simpa only [UnsignedWire.bytes, List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeeUnit_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeeUnit_expected
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeeUnit_overflow
