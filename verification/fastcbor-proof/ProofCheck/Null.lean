import ProofCheck.Containers

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 1600000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Null and undefined consume one byte at any valid offset, preserving the other cursor fields. -/
theorem wp_null_at (c : Loc) (v : cursor.Cursor) (xs : List w8) (b : w8)
    (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #true; c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have hindex := List.getElem?_eq_some_iff.mp hi
  have hlt : sint.Z v.pos' < sint.Z v.buf'.len := by
    simp only [sint.nat, sint.Z] at Hlen hp hindex ⊢
    obtain ⟨h, _⟩ := hindex
    omega
  wp_auto
  simp only [hlt, _root_.decide_true]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq b hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  rcases hn with rfl | rfl
  · wp_auto
    wp_end
  · wp_auto
    rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
    wp_apply wp_load_slice_index v.buf' _ xs dq (W8 247) hp $$ [Hs] with Hs
    · iframe Hs; ipureintro; exact hi
    wp_end

/-- The production Uint64 field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readUint64_null_at (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production Mode field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readMode_null_at (c dst : Loc) (old : L1DAMode) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadMode)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production String field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readString_null_at (c dst : Loc) (old : GoString) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadString)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ old ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production Felt field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readFelt_null_at (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ null ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production FeltPointers field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readFeltPointers_null_at (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFeltPointers)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ slice.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production Signatures field reader's null/undefined policy, for arbitrary prior state. -/
theorem wp_readSignatures_null_at (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hn : b = W8 246 ∨ b = W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSignatures)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ slice.nil ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_at c v xs b dq hp hi hn $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readMode_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readString_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFeltPointers_null_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSignatures_null_at
