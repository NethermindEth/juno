import ProofCheck.Header

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

/-- Null leaves every non-null byte and every part of the cursor unchanged. -/
theorem wp_null_other (c : Loc) (v : cursor.Cursor) (xs : List w8) (b : w8)
    (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b)
    (hn : b ≠ W8 246) (hu : b ≠ W8 247) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Null")) (Val #()))
    {{ RET #false; c ↦ v ∗ v.buf' ↦*{dq} xs }} := by
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
  simp only [hn, _root_.decide_false]
  wp_auto
  rw [ite_eq_left_of_eq_true _ _ (eq_true ⟨hp, hlt⟩)]
  wp_apply wp_load_slice_index v.buf' _ xs dq b hp $$ [Hs] with Hs
  · iframe Hs; ipureintro; exact hi
  simp only [hu, _root_.decide_false]
  wp_auto
  wp_end

/-- An inline uint at any valid cursor offset, preserving all input bytes. -/
theorem wp_expect_inline_at (c : Loc) (v : cursor.Cursor) (xs : List w8) (b : w8)
    (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"expect")) (Val #(W8 0)))
    {{ RET (PairV #(W64 (uint.Z b)) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
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
  have hb' : uint.Z (b - W8 0) < uint.Z (W8 24) := by word
  simp only [hb', _root_.decide_true]
  wp_auto
  have hsub : b - W8 0 = b := by word
  simp only [hsub]
  wp_end

theorem wp_uint64_inline_at (c : Loc) (v : cursor.Cursor) (xs : List w8) (b : w8)
    (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"Uint64")) (Val #()))
    {{ RET (PairV #(W64 (uint.Z b)) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_expect_inline_at c v xs b dq hp hi hb $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The production scalar reader overwrites a possibly nonzero destination. -/
theorem wp_readUint64_inline_at (c dst : Loc) (old : w64) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c)) (Val #dst))
    {{ RET #interface.nil; dst ↦ W64 (uint.Z b) ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  wp_start as ⟨Hdst, Hc, Hs⟩
  wp_auto
  wp_apply wp_null_other c v xs b dq hp hi (by word) (by word) $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_apply wp_uint64_inline_at c v xs b dq hp hi hb $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  wp_end

/-- The field call used by the generated Header switch changes Number only. -/
theorem wp_headerNumber_inline_at (c dst : Loc) (h : Header) (v : cursor.Cursor)
    (xs : List w8) (b : w8) (dq : DFrac) (hp : 0 ≤ sint.Z v.pos')
    (hi : xs[(sint.Z v.pos').toNat]? = some b) (hb : uint.Z b < 24) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ h ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadUint64)) (Val #c))
        (Val #(structFieldRef Header go!"Number" dst)))
    {{ RET #interface.nil; dst ↦ {h with Number' := W64 (uint.Z b)} ∗
       c ↦ {v with pos' := v.pos' + W64 1} ∗ v.buf' ↦*{dq} xs }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  icases (Header.Header_access_store_Number dst h (W64 (uint.Z b))).access_strict $$ Hdst with ⟨Hnumber, Hclose⟩
  wp_apply wp_readUint64_inline_at c _ h.Number' v xs b dq hp hi hb $$ [Hnumber Hc Hs] with ⟨Hnumber, Hc, Hs⟩
  · iframe Hnumber Hc Hs
  ihave Hdst := Hclose $$ Hnumber
  wp_end

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_null_other
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_expect_inline_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_uint64_inline_at
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readUint64_inline_at

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_headerNumber_inline_at
