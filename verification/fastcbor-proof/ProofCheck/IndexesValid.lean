import ProofCheck.IndexesMapValid
import ProofCheck.IndexesZero

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

/-- Complete successful record decoding, including null/undefined preservation
    and all map encodings at any supported offset and nesting depth. -/
theorem wp_readIndexes_valid (validator : Utf8ValidatorContract (GF := GF))
    (c dst : Loc) (h : BlockTransactionsIndexes) (initial : IndexesValue)
    (v : cursor.Cursor) (xs wire rem : List w8) (final : IndexesValue) (dq : DFrac) (d : Nat)
    (valid : IndexesRecordValid d initial wire final) (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ IndexesChildren h initial ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadBlockTransactionsIndexes)) (Val #c)) (Val #dst))
    {{ (out : BlockTransactionsIndexes), RET #interface.nil;
       dst ↦ out ∗ IndexesChildren out final ∗ c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hdst, Hchildren, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_func_call
    wp_call
    wp_auto
    wp_apply wp_null_at c v xs head dq hp.1 hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
    · iframe Hc Hs
    iapply HΦ
    isimp only [List.length_singleton, Int.natCast_one]
    iframe Hdst Hchildren Hc Hs
    ipureintro
    exact Hnext
  | @map head payload body argument count final enc major length depth pairs =>
    have harg : argument = W64 count := by word
    subst argument
    wp_apply wp_readIndexes_map_valid validator c dst h initial v xs payload body rem head dq d count final
      enc major hd depth pairs hp hbytes $$ [Hdst Hchildren Hc Hs] as %out ⟨Hdst, Hchildren, Hc, Hs, %Hmap⟩
    · iframe Hdst Hchildren Hc Hs
    have hsize : (((head :: payload) ++ body).length : Int) = 1 + payload.length + body.length := by simp; omega
    isimp only [← hsize] at Hc
    iapply HΦ
    iframe Hdst Hchildren Hc Hs
    ipureintro
    exact Hnext

theorem indexesChildren_zero :
    ⊢ (IndexesChildren (zero_val BlockTransactionsIndexes) indexesZeroValue : IProp GF) := by
  unfold IndexesChildren indexesZeroValue IntsResult
  isplit <;> ipureintro <;> rfl

/-- Clearing a partial record discards both child allocations and reestablishes
    the public zero ownership contract. -/
theorem wp_clearIndexes_owned (dst : Loc) (h : BlockTransactionsIndexes) (value : IndexesValue) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ IndexesChildren h value }}
      (App (Val (@! cborClearBlockTransactionsIndexes)) (Val #dst))
    {{ RET #(); IndexesOwned dst indexesZeroValue }} := by
  wp_start_folded as ⟨Hdst, Hchildren⟩
  wp_apply wp_clearIndexes dst h $$ [Hdst] with Hdst
  · iframe Hdst
  iapply HΦ
  unfold IndexesOwned
  iexists (zero_val BlockTransactionsIndexes)
  iframe Hdst
  iapply indexesChildren_zero (GF := GF)

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readIndexes_valid
#print axioms Perennial.github_com.NethermindEth.juno.core.indexesChildren_zero
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_clearIndexes_owned
