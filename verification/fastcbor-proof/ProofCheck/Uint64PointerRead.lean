import ProofCheck.Uint64PointerSpec

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

/-- An allocated zero word is distinct from nil and owns its own storage. -/
def Uint64PointerResult (p : Loc) : Option w64 → IProp GF
  | none => iprop(⌜p = null⌝)
  | some value => iprop(p ↦ value ∗ ⌜p ≠ null⌝)

/-- The actual helper clears a pointer on null/undefined or publishes freshly
    allocated storage for every supported unsigned width. Any old pointee is
    outside the precondition and remains available through the frame rule. -/
theorem wp_readUint64Pointer_valid (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option w64) (dq : DFrac) (valid : Uint64PointerValid wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64Pointer)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; ∃ p : Loc, dst ↦ p ∗ Uint64PointerResult p value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  wp_auto
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ
    iexists null
    isimp only [Uint64PointerResult, List.length_singleton, Int.natCast_one]
    iframe Hdst Hc Hs
    ipureintro
    simpa only [List.length_singleton, Int.natCast_one] using Hnext
  | @unsigned head payload argument enc major =>
    have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have hn : head ≠ W8 246 ∧ head ≠ W8 247 := by
      constructor <;> intro h <;> subst head <;> contradiction
    wp_apply wp_null_other c v xs head dq hp.1 hi hn.1 hn.2 $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    wp_apply wp_uint64_wire c v xs rem ⟨head, payload, argument, major, enc⟩ dq hp hbytes
      $$ [Hc Hs] with ⟨Hc, Hs, %Hvalue⟩
    · iframe Hc Hs
    ihave %Hnn := typedPointsto_not_null _ _ _ $$ v
    iapply HΦ
    iexists v_ptr
    isimp only [Uint64PointerResult]
    isimp only [UnsignedWire.bytes] at Hc
    iframe Hdst v Hc Hs
    ipureintro
    exact ⟨Hnn, Hnext⟩

/-- Every scalar rejection preserves the old pointer and cursor; allocated
    temporary words are never published on the error path. -/
theorem wp_readUint64Pointer_failure (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire : List w8) (dq : DFrac) (reason : ExpectedFailureReason) (kind : GoInterface)
    (failure : ScalarFailure wire reason)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       expectedFailureAddr (GF := GF) reason ↦□ kind ∗ dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64Pointer)) (Val #c)) (Val #dst))
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

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64Pointer_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64Pointer_failure
