import ProofCheck.FeltPointers

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

theorem array_head_not_nullish (head : w8) (major : head &&& W8 224 = W8 128) :
    head ≠ W8 246 ∧ head ≠ W8 247 := by
  constructor
  · intro heq; rw [heq] at major; contradiction
  · intro heq; rw [heq] at major; contradiction

/-- A specified typed felt failure cannot be intercepted by nullable wrappers. -/
theorem feltFailure_not_nullish (d offset : Nat) (wire : List w8) (reason : FeltFailureReason)
    (failure : FeltFailure d wire offset reason) :
    wire.head? ≠ some (W8 246) ∧ wire.head? ≠ some (W8 247) := by
  cases failure with
  | outer hn hu failure => exact ⟨hn, hu⟩
  | depth tail enc major => simpa using array_head_not_nullish _ major
  | limb items enc major bound short more failure => simpa using array_head_not_nullish _ major
  | extras items extras enc major bound four more valid failure => simpa using array_head_not_nullish _ major

/-- Null preserves the input/cursor and returns false for every nonnullish
    unread sequence, including EOF without attempting an input load. -/
theorem wp_null_not_nullish (c : Loc) (v : cursor.Cursor) (xs wire : List w8) (dq : DFrac)
    (notNull : wire.head? ≠ some (W8 246)) (notUndefined : wire.head? ≠ some (W8 247))
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #false; c ↦ v ∗ v.buf' ↦*{dq} xs }} := by
  cases wire with
  | nil =>
    wp_start_folded as ⟨Hc, Hs⟩
    ihave %Hlen := ownSlice_len _ _ _ $$ Hs
    have hlen := congrArg List.length hbytes
    simp only [subslice_length (sint.nat v.pos') (sint.nat v.buf'.len) xs (by omega), List.length_nil] at hlen
    have heof : v.pos' = v.buf'.len := by simp only [sint.nat] at Hlen hlen; word
    wp_apply wp_null_eof c v heof $$ [Hc] with Hc
    · iframe Hc
    wp_end
  | cons head tail =>
    exact wp_null_other c v xs head dq hp.1
      (input_suffix_head xs tail (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes)
      (by simpa using notNull) (by simpa using notUndefined)

/-- The first unpublished slot exists whenever any capacity remains. -/
theorem uninitialized_prefix_lookup {V : Type} (done : List V) (n : Nat) (zero : V) (hn : 0 < n) :
    (done ++ List.replicate n zero)[done.length]? = some zero := by simp [hn]

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.array_head_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.feltFailure_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null_not_nullish
#print axioms Perennial.github_com.NethermindEth.juno.core.uninitialized_prefix_lookup
