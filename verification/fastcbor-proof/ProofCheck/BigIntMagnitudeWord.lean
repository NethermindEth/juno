import ProofCheck.BigIntMagnitudeSpec
import ProofCheck.FeltWide

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- The partial-word step is arithmetic base-256 accumulation without overflow. -/
theorem bigMagnitude_shift_byte (acc : w64) (byte : w8) (hbound : uint.Z acc < 2 ^ 56) :
    uint.Z ((acc <<< W64 8) ||| W64 (uint.Z byte)) = 256 * uint.Z acc + uint.Z byte := by
  have hacc : acc.toNat < 2 ^ 56 := by word
  have hbyte : (byte.setWidth 64).toNat < 2 ^ 8 := by
    simp only [BitVec.toNat_setWidth]
    have := byte.isLt
    omega
  have h := encoding.binary.toNat_or_shl (byte.setWidth 64) (acc.setWidth 56) 8 hbyte (by decide)
  have hsame : (acc.setWidth 56).setWidth 64 = acc := by
    apply BitVec.eq_of_toNat_eq
    simp only [BitVec.toNat_setWidth, Nat.mod_eq_of_lt hacc]
    have := acc.isLt
    omega
  rw [hsame] at h
  simp only [BitVec.toNat_setWidth, Nat.mod_eq_of_lt hacc, Nat.reducePow] at h
  have hb := byte.isLt
  have hbyte64 : byte.toNat % 18446744073709551616 = byte.toNat := by omega
  rw [hbyte64] at h
  have hnum : (byte.setWidth 64 ||| (acc <<< (8 : Nat))).toNat =
      256 * acc.toNat + byte.toNat := by omega
  have hcast : W64 (uint.Z byte) = byte.setWidth 64 := by
    simp only [W64, uint.Z, BitVec.ofInt_natCast, BitVec.ofNat_toNat]
  have hshift : acc <<< W64 8 = acc <<< (8 : Nat) := rfl
  rw [hcast, hshift, BitVec.or_comm]
  change ((byte.setWidth 64 ||| (acc <<< (8 : Nat))).toNat : Int) =
    256 * (acc.toNat : Int) + (byte.toNat : Int)
  rw [hnum]
  simp only [Int.natCast_add, Int.natCast_mul]
  rfl

/-- The actual eight-byte load used by bigIntMagnitude agrees with the unbounded
    byte specification and preserves the entire fractional input ownership.
    This proves the word-load dependency, not the enclosing packing loop. -/
theorem wp_bigMagnitude_word_load (data : GoSlice) (xs bytes rem : List w8)
    (off high : w64) (dq : DFrac) (hhigh : high = data.len)
    (hbound : 0 ≤ sint.Z off ∧ sint.Z off ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat off) (sint.nat data.len) xs = bytes ++ rem)
    (hlen : bytes.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint64"))
        (Val #(slice.slice data w8 off high)))
    {{ RET #(leToU64 bytes.reverse); data ↦*{dq} xs ∗
       ⌜uint.Z (leToU64 bytes.reverse) = bigEndianMagnitude bytes⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  wp_apply wp_bigEndian_offset data xs bytes rem off high dq hhigh hbound hbytes hlen
    $$ [Hs] with Hs
  · iframe Hs
  iapply HΦ
  iframe Hs
  ipureintro
  exact bigWord_bigEndian_value bytes (by omega)

/-- The full-word packing loop can borrow an interior eight-byte window while
    retaining ownership of the already-consumed suffix and unread prefix. -/
theorem wp_bigMagnitude_window_load (data : GoSlice) (xs bytes : List w8)
    (low high : w64) (dq : DFrac)
    (hbound : 0 ≤ sint.Z low ∧ sint.Z low ≤ sint.Z high ∧ sint.Z high ≤ sint.Z data.len)
    (hbytes : subslice (sint.nat low) (sint.nat high) xs = bytes)
    (hlen : bytes.length = 8) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.encoding.binary ∗ data ↦*{dq} xs }}
      (App (Val ((globalAddr encoding.binary.BigEndian) @!!
        go.PointerType encoding.binary.bigEndian.ty @!! go!"Uint64"))
        (Val #(slice.slice data w8 low high)))
    {{ RET #(leToU64 bytes.reverse); data ↦*{dq} xs }} := by
  iintro %Φ ⟨#Hpkg, Hs⟩ HΦ
  icases (ownSlice_slice low high data dq xs hbound).1 $$ Hs with ⟨Hpre, Hbytes, Hpost⟩
  isimp only [hbytes] at Hbytes
  wp_apply encoding.binary.wp_BigEndian_Uint64 (slice.slice data w8 low high)
    bytes dq [] hlen $$ [Hbytes] with Hbytes
  · isimp only [List.append_nil]; iframe Hbytes
  iapply HΦ
  iapply (ownSlice_slice low high data dq xs hbound).2
  rw [hbytes]
  isimp only [List.append_nil] at Hbytes
  iframe Hpre Hbytes Hpost

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigMagnitude_word_load
#print axioms Perennial.github_com.NethermindEth.juno.core.bigMagnitude_shift_byte

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigMagnitude_window_load
