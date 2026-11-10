import ProofCheck.FeltLong

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


/-- Accepted felt field encodings and their value, independently of fast-path selection.
Null/undefined clear the pointer. Short arrays zero-fill. Long arrays retain four
unsigned Montgomery limbs and consume recursively specified ignored values. -/
inductive FeltValid : Nat → List w8 → Option (GoArray w64 4) → Prop where
  | nullish {depth : Nat} {head : w8} (hn : head = W8 246 ∨ head = W8 247) :
      FeltValid depth [head] none
  | small {depth : Nat} {head : w8} {payload : List w8} {argument : w64}
      (items : List UnsignedWire) (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 128) (count : uint.Z argument = items.length)
      (small : items.length ≤ 4) (bound : depth < 32) :
      FeltValid depth ((head :: payload) ++ unsignedBytes items) (some (paddedLimbs items))
  | extended {depth : Nat} {head : w8} {payload : List w8} {argument : w64}
      (items : List UnsignedWire) (extras : List (List w8)) (enc : HeadArgument head payload argument)
      (major : head &&& W8 224 = W8 128) (count : uint.Z argument = 4 + extras.length)
      (four : items.length = 4) (bound : depth < 32)
      (valid : ∀ item ∈ extras, SkipValid (depth + 1) item) :
      FeltValid depth ((head :: payload) ++ unsignedBytes items ++ extras.flatten) (some (paddedLimbs items))

/-- Nil is distinct from an allocated zero felt, and nonnil results own their four words. -/
def FeltResult (p : Loc) (value : Option (GoArray w64 4)) : IProp GF :=
  match value with
  | none => iprop(⌜p = null⌝)
  | some words => iprop(p ↦ words ∗ ⌜p ≠ null⌝)

/-- The production typed reader realizes every accepted felt encoding, including ownership and suffixes. -/
theorem wp_readFelt_valid (c dst : Loc) (old : Loc) (v : cursor.Cursor)
    (xs wire rem : List w8) (value : Option (GoArray w64 4)) (dq : DFrac) (d : Nat)
    (hd : sint.Z v.depth' = d) (hdepth : d ≤ 32) (valid : FeltValid d wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadFelt)) (Val #c)) (Val #dst))
    {{ (p : Loc), RET #interface.nil; dst ↦ p ∗ FeltResult (GF := GF) p value ∗
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hdst, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs wire rem v.pos' v.buf'.len hp Hlen.1 hbytes
  cases valid with
  | @nullish head hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    wp_apply wp_readFelt_null_at c dst old v xs head dq hp.1 hi hn $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_singleton, Int.natCast_one] at HΦ
    simp only [List.length_singleton, Int.natCast_one] at Hnext
    iapply HΦ
    isimp only [FeltResult]
    iframe Hdst Hc Hs
    ipureintro
    exact ⟨rfl, Hnext⟩
  | @small head payload argument items enc hm count small bound =>
    have heq : argument = W64 items.length := by word
    rw [heq] at enc
    wp_apply wp_readFelt_small c dst old v xs payload rem head items dq enc hm small hp (by omega) hbytes
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hv, Hc, Hs, %Hout⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    iapply HΦ
    isimp only [FeltResult]
    iframe Hdst Hv Hc Hs
    ipureintro
    exact Hout
  | @extended head payload argument items extras enc hm count four bound children =>
    have heq : argument = W64 (4 + extras.length) := by word
    rw [heq] at enc
    wp_apply wp_readFelt_extended c dst old v xs payload rem head items extras dq d hd bound enc hm four children hp hbytes
      $$ [Hdst Hc Hs] as %p ⟨Hdst, Hv, Hc, Hs, %Hout⟩
    · iframe Hdst Hc Hs
    isimp only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
      Int.add_comm (payload.length : Int) 1] at HΦ
    iapply HΦ
    isimp only [FeltResult]
    iframe Hdst Hv Hc Hs
    ipureintro
    exact Hout

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readFelt_valid
