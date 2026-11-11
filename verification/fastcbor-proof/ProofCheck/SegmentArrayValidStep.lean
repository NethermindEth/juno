import ProofCheck.SegmentArrayValid

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

theorem wp_readSegmentArray_valid_of_record (d : Nat)
    (next : d < 32 → SegmentRecordValidContract (GF := GF) (d + 1))
    (c dst : Loc) (old : GoSlice) (v : cursor.Cursor)
    (xs wire rem : List w8) (values : Option (List SegmentValue)) (dq : DFrac)
    (valid : SegmentArraySpec d wire (.success values rem)) (hd : sint.Z v.depth' = d)
    (hp : 0 ≤ sint.Z v.pos' ∧ sint.Z v.pos' ≤ sint.Z v.buf'.len)
    (hbytes : subslice (sint.nat v.pos') (sint.nat v.buf'.len) xs = wire) :
    {{ isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborclasses ∗
       isPkgInit (PROP := IProp GF) pkg_id.github_com.NethermindEth.juno.core.internal.cborschema ∗
       dst ↦ old ∗ c ↦ v ∗ v.buf' ↦*{dq} xs }}
      (App (App (Val (@! cborReadSegmentLengthsValues)) (Val #c)) (Val #dst))
    {{ (out : GoSlice), RET #interface.nil;
       dst ↦ out ∗ SegmentArrayResult out values ∗
       c ↦ {v with pos' := v.pos' + W64 (wire.length - rem.length : Nat)} ∗ v.buf' ↦*{dq} xs ∗
       ⌜sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) = sint.Z v.pos' + (wire.length - rem.length : Nat) ∧
         sint.Z (v.pos' + W64 (wire.length - rem.length : Nat)) ≤ sint.Z v.buf'.len ∧
         subslice (sint.nat (v.pos' + W64 (wire.length - rem.length : Nat))) (sint.nat v.buf'.len) xs = rem⌝ }} := by
  wp_start_folded as ⟨Hdst, Hc, Hs⟩
  ihave %Hlen := ownSlice_len _ _ _ $$ Hs
  cases valid with
  | nullish head tail hn =>
    have hi := input_suffix_head xs rem (sint.nat v.pos') (sint.nat v.buf'.len) head hbytes
    have Hnext := input_consumption xs [head] rem v.pos' v.buf'.len hp Hlen.1 hbytes
    wp_apply wp_readSegmentArray_null c dst old v xs head dq hp.1 hi hn
      $$ [Hdst Hc Hs] with ⟨Hdst, Hc, Hs⟩
    · iframe Hdst Hc Hs
    have hlength : (head :: rem).length - rem.length = 1 := by simp
    isimp only [hlength, Int.natCast_one, SegmentArrayResult] at HΦ
    iapply HΦ
    iframe Hdst Hc Hs
    ipureintro
    exact ⟨rfl, by simpa only [List.length_singleton, Int.natCast_one] using Hnext⟩
  | @array _ head payload item rest argument values count enc major size fit depth items =>
    have harg : argument = W64 count := by word
    subst argument
    have Hnext := input_consumption xs ((head :: payload) ++ item) rem v.pos' v.buf'.len hp Hlen.1 hbytes
    wp_apply wp_readSegmentArray_array_of_record c dst old v xs payload rem head dq d (next depth) count item values
      items enc major hd depth hp hbytes $$ [Hdst Hc Hs] as %out ⟨Hdst, Howned, Hc, Hs, %Hstep⟩
    · iframe Hdst Hc Hs
    have hlength : (((head :: payload) ++ item) ++ rem).length - rem.length = 1 + payload.length + item.length := by simp; omega
    isimp only [hlength, Int.natCast_add, Int.natCast_one, SegmentArrayResult] at HΦ
    iapply HΦ
    iframe Hdst Howned Hc Hs
    ipureintro
    refine ⟨?_, ?_, Hstep.2⟩
    · simpa only [Int.add_assoc] using Hstep.1
    · simpa only [List.length_append, List.length_cons, Int.natCast_add, Int.natCast_one,
        Int.add_comm (payload.length : Int) 1, Int.add_assoc] using Hnext.2.1

end github_com.NethermindEth.juno.core
end Perennial
#print axioms Perennial.github_com.NethermindEth.juno.core.wp_readSegmentArray_valid_of_record
