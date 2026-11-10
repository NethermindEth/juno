import ProofCheck.BigIntSpec
import ProofCheck.BigIntOwned
import ProofCheck.Payload

set_option linter.iris.style.nameCheck false
set_option linter.unusedSectionVars false
set_option maxHeartbeats 6400000
set_option maxRecDepth 10000

noncomputable section
namespace Perennial
open Iris Iris.BI Iris.ProgramLogic Iris.Std OFE Iris.ProofMode
open github_com.NethermindEth.juno.utils.cbor
namespace github_com.NethermindEth.juno.core

variable [GooseExts] [GooseSemantics] [GooseInterp] [GoGlobalContext]
variable {hlc : HasLC} {GF : BundledGFunctors} [hG : HeapGS hlc GF]
variable [sem : go.Semantics] [package_sem : Assumptions]

/-- Head parsing with its exact suffix and nonwrapping byte-consumption fact. -/
theorem wp_head_prefix (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (enc : HeadArgument head payload argument)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"head")) (Val #()))
    {{ RET (PairV (PairV #(head &&& W8 224) #argument) #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hnext := input_consumption xs (head :: payload) rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have hbound : sint.Z v.pos' + 1 + payload.length ≤ sint.Z v.buf'.len := by
    simp only [List.length_cons, Int.natCast_add, Int.natCast_one] at Hnext; omega
  have hi := input_suffix_head xs (payload ++ rem) (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
  have hafter : subslice (sint.nat (v.pos' + W64 1)) (sint.nat v.buf'.len) xs = payload ++ rem := by
    have hn : sint.nat (v.pos' + W64 1) = sint.nat v.pos' + 1 := by word
    rw [hn]
    exact input_suffix_after xs [head] (payload ++ rem) _ _ hbytes
  wp_apply wp_head_encoded c v xs payload rem head argument dq enc hp.1 hbound hi hafter
    $$ [Hc Hs] with ⟨Hc, Hs⟩
  · iframe Hc Hs
  iapply HΦ
  iframe Hc Hs
  ipureintro
  simpa only [List.length_cons, Int.natCast_add, Int.natCast_one,
    Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext

/-- All plain unsigned and negative CBOR integers, including -2^64, are decoded
    to canonical independently owned big.Int values. -/
theorem wp_bigInt_plain (c : Loc) (v : cursor.Cursor) (xs payload rem : List w8)
    (head : w8) (argument : w64) (dq : DFrac) (negative : Bool)
    (enc : HeadArgument head payload argument)
    (major : head &&& W8 224 = if negative then W8 32 else W8 0)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = (head :: payload) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (p : Loc), RET (PairV #p #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (1 + payload.length)} ∗ v.buf' ↦*{dq} xs ∗
       BigMagnitudeOwned p (if negative then -1 - uint.Z argument else uint.Z argument) ∗
       ⌜sint.Z (v.pos' + W64 (1 + payload.length)) = sint.Z v.pos' + 1 + payload.length ∧
         sint.Z (v.pos' + W64 (1 + payload.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (1 + payload.length))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  wp_auto
  wp_apply wp_head_prefix c v xs payload rem head argument dq enc hp hbytes
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  cases negative with
  | false =>
    isimp only [Bool.false_eq_true, ite_false] at HΦ
    simp only [major]
    wp_auto
    wp_alloc p as Hp
    wp_apply wp_bigSetUint64_owned p argument $$ [Hp] with Howned
    · iframe Hp
    iapply HΦ
    iframe Hc Hs Howned
    ipureintro; exact Hnext
  | true =>
    isimp only [ite_true] at HΦ
    simp only [major]
    wp_auto
    by_cases hmax : argument = W64 18446744073709551615
    · simp only [hmax, _root_.decide_true]
      wp_auto
      wp_apply wp_slice_literal (V := w8) [W8 1, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0]
      isplitr
      · ipureintro; rfl
      iintro %sl ⟨Hmag, -⟩
      simp only [go.arrayLiteralSize, List.foldl_cons, List.foldl_nil, Int.reduceAdd,
        show max (9 : Int) 0 = 9 from by decide]
      wp_apply wp_bigMagnitude (slice.mk sl (W64 9) (W64 9))
        [W8 1, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0] (DFrac.own 1) true false
        $$ [Hmag] as %p ⟨-, Howned⟩
      · iframe Hmag
      have heq : (if true then -1 else 1) *
          (bigEndianMagnitude [W8 1, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0, W8 0] + if false then 1 else 0) =
          -1 - uint.Z (W64 18446744073709551615) := by decide
      simp only [ite_true, Bool.false_eq_true, ite_false] at heq
      isimp only [heq] at Howned
      iapply HΦ
      iframe Hc Hs Howned
      ipureintro; exact Hnext
    · simp only [hmax, _root_.decide_false]
      wp_auto
      wp_alloc p as Hp
      wp_apply wp_bigSetUint64_owned p (argument + W64 1) $$ [Hp] with Howned
      · iframe Hp
      wp_apply wp_bigNeg_owned p (uint.Z (argument + W64 1)) $$ [Howned] with Howned
      · iframe Howned
      have heq : -(uint.Z (argument + W64 1)) = -1 - uint.Z argument := by word
      isimp only [heq] at Howned
      iapply HΦ
      iframe Hc Hs Howned
      ipureintro; exact Hnext

/-- Tags 2 and 3 decode every definite byte-string magnitude, preserving exact
    suffixes and input ownership even for empty bodies and long carry chains. -/
theorem wp_bigInt_tagged (c : Loc) (v : cursor.Cursor) (xs tagBytes lengthBytes body rem : List w8)
    (tagHead bodyHead : w8) (tag count : w64) (dq : DFrac)
    (tagEnc : HeadArgument tagHead tagBytes tag) (tagMajor : tagHead &&& W8 224 = W8 192)
    (allowed : tag = W64 2 ∨ tag = W64 3)
    (bodyEnc : HeadArgument bodyHead lengthBytes count) (bodyMajor : bodyHead &&& W8 224 = W8 64)
    (countEq : uint.Z count = body.length)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs =
      ((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ body) ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (p : Loc), RET (PairV #p #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)} ∗
       v.buf' ↦*{dq} xs ∗ BigMagnitudeOwned p
         (if tag = W64 3 then -1 - bigEndianMagnitude body else bigEndianMagnitude body) ∗
       ⌜sint.Z (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)) =
           sint.Z v.pos' + 2 + tagBytes.length + lengthBytes.length + body.length ∧
         sint.Z (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)))
           (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start as ⟨Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  have Hfull := input_consumption xs ((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ body)
    rem v.pos' v.buf'.len hp Hlen.1 hbytes
  have htotal : (((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ body).length : Int) =
      2 + tagBytes.length + lengthBytes.length + body.length := by simp; omega
  have hadvance : sint.Z (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)) =
      sint.Z v.pos' + 2 + tagBytes.length + lengthBytes.length + body.length ∧
      sint.Z (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)) ≤ sint.Z v.buf'.len ∧
      subslice (sint.nat (v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length)))
        (sint.nat v.buf'.len) xs = rem := by
    simpa only [htotal, Int.add_assoc] using Hfull
  wp_auto
  wp_apply wp_head_prefix c v xs tagBytes (((bodyHead :: lengthBytes) ++ body) ++ rem)
    tagHead tag dq tagEnc hp (by simpa only [List.append_assoc] using hbytes)
    $$ [Hc Hs] with ⟨Hc, Hs, %Hnext⟩
  · iframe Hc Hs
  simp only [tagMajor]
  wp_auto
  wp_join (iprop(c ↦ {v with pos' := v.pos' + W64 (1 + tagBytes.length)} ∗
      v.buf' ↦*{dq} xs ∗ argument_ptr ↦ tag) : IProp GF)
      at (if: _ then _ else _) with [Hc Hs argument] as ⟨Hc, Hs, argument⟩
  · rcases allowed with rfl | rfl <;> wp_auto <;> wp_join_done
  have hc : count = W64 body.length := by word
  have hpay := wp_payload_encoded (GF := GF) c {v with pos' := v.pos' + W64 (1 + tagBytes.length)}
    xs lengthBytes body rem bodyHead dq (by simpa only [← hc] using bodyEnc)
    (by dsimp only; constructor; rw [Hnext.1]; omega; exact Hnext.2.1) Hnext.2.2
  simp only [bodyMajor] at hpay
  wp_apply hpay $$ [Hc Hs] with ⟨Hc, Hs, %Hbody⟩
  · iframe Hc Hs
  wp_apply wp_bigMagnitude_window v.buf' xs body
    ((v.pos' + W64 (1 + tagBytes.length)) + W64 (1 + lengthBytes.length))
    ((v.pos' + W64 (1 + tagBytes.length)) + W64 (1 + lengthBytes.length + body.length))
    dq (decide (tag = W64 3)) (decide (tag = W64 3))
    ⟨Hbody.1, Hbody.2.1, Hbody.2.2.1⟩ Hbody.2.2.2.1
    $$ [Hs] as %p ⟨Hs, Howned⟩
  · iframe Hs
  have heq : (if decide (tag = W64 3) then -1 else 1) *
      (bigEndianMagnitude body + if decide (tag = W64 3) then 1 else 0) =
      (if tag = W64 3 then -1 - bigEndianMagnitude body else bigEndianMagnitude body) := by
    by_cases ht : tag = W64 3 <;>
      simp only [ht, _root_.decide_true, _root_.decide_false, Bool.false_eq_true, ite_true, ite_false, Int.neg_one_mul, Int.one_mul, Int.add_zero] <;> omega
  isimp only [heq] at Howned
  have hpos : v.pos' + W64 (1 + tagBytes.length) + W64 (1 + lengthBytes.length + body.length) =
      v.pos' + W64 (2 + tagBytes.length + lengthBytes.length + body.length) := by word
  isimp only [hpos] at Hc
  iapply HΦ
  iframe Hc Hs Howned
  ipureintro; exact hadvance

/-- The public decoder realizes every supported integer encoding at any valid
    input offset, returning an owned mathematical value and the exact suffix. -/
theorem wp_bigInt_valid (c : Loc) (v : cursor.Cursor) (xs wire rem : List w8)
    (value : Int) (dq : DFrac) (valid : BigIntValid wire value)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire ++ rem) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.utils.cbor.cursor ∗
       c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (Val (c @!! go.PointerType cursor.Cursor.ty @!! go!"BigInt")) (Val #()))
    {{ (p : Loc), RET (PairV #p #interface.nil);
       c ↦ {v with pos' := v.pos' + W64 wire.length} ∗ v.buf' ↦*{dq} xs ∗ BigMagnitudeOwned p value ∗
       ⌜sint.Z (v.pos' + W64 wire.length) = sint.Z v.pos' + wire.length ∧
         sint.Z (v.pos' + W64 wire.length) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 wire.length)) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  iintro %Φ ⟨#Hpkg, Hc, Hs⟩ HΦ
  cases valid with
  | @unsigned head payload argument enc major =>
    have hsize : ((head :: payload).length : Int) = 1 + payload.length := by simp; omega
    isimp only [hsize] at HΦ
    wp_apply wp_bigInt_plain c v xs payload rem head argument dq false enc major hp hbytes
      $$ [Hc Hs] as %p ⟨Hc, Hs, Howned, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hc Hs Howned
    ipureintro; simpa only [Int.add_assoc] using Hnext
  | @negative head payload argument enc major =>
    have hsize : ((head :: payload).length : Int) = 1 + payload.length := by simp; omega
    isimp only [hsize] at HΦ
    wp_apply wp_bigInt_plain c v xs payload rem head argument dq true enc major hp hbytes
      $$ [Hc Hs] as %p ⟨Hc, Hs, Howned, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hc Hs Howned
    ipureintro; simpa only [Int.add_assoc] using Hnext
  | @tagged tagHead bodyHead tagBytes lengthBytes body tag count tagEnc tagMajor allowed bodyEnc bodyMajor countEq =>
    have hsize : (((tagHead :: tagBytes) ++ (bodyHead :: lengthBytes) ++ body).length : Int) =
        2 + tagBytes.length + lengthBytes.length + body.length := by simp; omega
    isimp only [hsize] at HΦ
    wp_apply wp_bigInt_tagged c v xs tagBytes lengthBytes body rem tagHead bodyHead tag count dq
      tagEnc tagMajor allowed bodyEnc bodyMajor countEq hp hbytes
      $$ [Hc Hs] as %p ⟨Hc, Hs, Howned, %Hnext⟩
    · iframe Hc Hs
    iapply HΦ
    iframe Hc Hs Howned
    ipureintro; simpa only [Int.add_assoc] using Hnext

end github_com.NethermindEth.juno.core
end Perennial

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_head_prefix
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_plain

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_tagged

#print axioms Perennial.github_com.NethermindEth.juno.core.wp_bigInt_valid
